// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"magicmail/config"
	"magicmail/crypto"
	"magicmail/database"
	"magicmail/imap"
	"magicmail/middleware"
	"magicmail/oauth2"
	"magicmail/routes"
	"magicmail/sse"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// isProduction 通过 ldflags 注入：生产构建为 "true"，开发构建为 "false"
var isProduction = "false"

// @title           Magicmail API
// @version         1.0
// @description     邮件代收服务 - IMAP 代理收信 + RESTful API
// @host            localhost:8080
// @BasePath        /api/v1

func main() {
	// ⭐ 全局设置 go-message 的字符集解码器
	// 重要：此处返回透传(passthrough)，不让 go-message 自行解码 body！
	// 原因：go-message 内部对 CharsetReader 返回值的处理有 bug，会导致
	//       双重编码(Mojibake)：GBK→UTF-8→Latin-1→UTF-8 = 乱码
	//       我们在 fetcher.go 的 decodeTextContentWithCharset() 中自行正确解码
	message.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		cs := strings.ToLower(strings.Trim(charset, "\"' \t"))
		log.Printf("🔍 [global-CharsetReader] called with charset=%q, normalized=%q → PASSTHROUGH (raw bytes)", charset, cs)
		// ⭐ 一律透传，返回原始字节流，由调用方自行解码
		return input, nil
	}

	// 生产构建自动静默 SQL 日志（database 包读取此环境变量）
	if isProduction == "true" {
		os.Setenv("MAGICMAIL_ENV", "production")
	}

	// 初始化日志输出：开发→终端 stderr，生产→数据卷日志文件（含轮转）
	setupLogging()

	// 加载配置
	cfg := config.Load()

	// 初始化数据库
	db := database.Init(cfg.Database.DSN)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// 确保安全密钥存在：支持环境变量传入（优先），未设置则自动生成并存入数据库
	database.EnsureSecuritySecrets(db, &cfg.Security.JWTSecret, &cfg.Security.EncryptionKey)

	// 初始化密码加密模块（AES-256-GCM）
	if err := crypto.Init(cfg.Security.EncryptionKey); err != nil {
		log.Fatalf("❌ 加密模块初始化失败: %v", err)
	}

	// 初始化 OAuth2 全局注册中心（注册 Microsoft 等内置 Provider）
	oauth2.InitGlobalRegistry()

	// 创建 Fiber 实例
	// 关键配置：SSE 长连接需要较长的空闲超时和禁用写超时
	app := fiber.New(fiber.Config{
		AppName:          "Magicmail",
		ServerHeader:     "Magicmail",
		IdleTimeout:      60 * time.Second, // 长连接最大空闲时间（心跳间隔15s，留足余量）
		ReadTimeout:      10 * time.Second, // 读取请求头超时
		WriteTimeout:     0,                // 禁用写超时（SSE 长连接需要持续写入）
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		DisableKeepalive: false, // 保持连接活跃
	})

	// 全局中间件
	app.Use(recover.New())

	// ⭐ 统一网关前缀剥离：必须早于任何路由注册（含 logger / CORS / routes.Register）。
	// 飞牛统一网关不会剥离 gatewayPrefix（/app/magicmail），而是整段透传；
	// 该中间件在路由匹配前把前缀去掉，使后端只需维护一套根路由。
	// 非飞牛部署（MAGICMAIL_BASE_PATH 为空）时本中间件完全透传，行为不变。
	app.Use(middleware.BasePath(cfg.Server.BasePath))

	// Logger 中间件：排除 SSE 流端点（避免干扰长连接）
	// 注意：SSE 端点 /api/v1/mails/stream 需要保持长连接，logger 的响应拦截可能影响它
	app.Use(logger.New(logger.Config{
		Next: func(c *fiber.Ctx) bool {
			// 此时 c.Path() 已是剥离前缀后的内部路径
			return c.Path() == "/api/v1/mails/stream"
		},
		// 访问日志与业务日志走同一输出目标（开发=终端，生产=日志文件）
		Output: logWriter,
	}))

	// 注册 API 路由
	// ⚠️ 必须传已完成 EnsureSecuritySecrets 的 cfg：routes.Register 内部若重新
	// config.Load()，会拿到空的 JWT 密钥并以空密钥签发/校验 token。
	routes.Register(app, db, cfg)

	// 启动 IMAP 后台 Worker（所有活跃账号）
	go imap.StartWorkers(db, cfg)

	// 初始化 SSE 实时推送服务
	sse.InitBroker()

	// 启动 HTTP 服务
	// 监听方式由 cfg.Server.Listen 决定（主监听）：
	//   - tcp://HOST:PORT → 普通 TCP 监听（Docker / 旧部署）
	//   - unix:///path/app.sock → Unix Socket（飞牛统一网关）
	// 若 cfg.Server.TCPEnabled 为 true，则会在主监听之外再并行监听一个 TCP 端口
	//（飞牛向导 "both" 模式：网关走 Unix，外部直连走 TCP）。
	if err := listenAndServe(app, cfg); err != nil {
		log.Fatalf("❌ 服务启动失败: %v", err)
	}
}

// buildListener 根据 listen 字符串创建主监听器：
//   - unix:///path/app.sock → Unix Socket（启动前清理旧文件、权限 0666）
//   - tcp://HOST:PORT / 纯 host:port → TCP 监听
func buildListener(listen string) (net.Listener, error) {
	switch {
	case strings.HasPrefix(listen, "unix://"):
		sockPath := strings.TrimPrefix(listen, "unix://")
		// 启动前清理旧 socket 文件，避免 "address already in use"
		_ = os.Remove(sockPath)
		ln, err := net.Listen("unix", sockPath)
		if err != nil {
			return nil, fmt.Errorf("创建 unix socket %s 失败: %w", sockPath, err)
		}
		// socket 权限 0660：Unix Socket 是飞牛形态的唯一入口，
		// 0666 会让 NAS 上任意本地进程都能连上它并伪造 X-Trim-Userid 免密登录，
		// 因此收紧为「属主 + 属组」可读写（网关进程需与应用同组或具备 root 权限）。
		// ⚠️ 若真机验证发现网关连不上（表现为应用 502 / 白屏），回退为 0666 并记录原因。
		_ = os.Chmod(sockPath, 0660)
		log.Printf("🚀 Magicmail 服务启动于 Unix Socket: %s", sockPath)
		return ln, nil
	case strings.HasPrefix(listen, "tcp://"):
		addr := strings.TrimPrefix(listen, "tcp://")
		log.Printf("🚀 Magicmail 服务启动于 http://%s", addr)
		return net.Listen("tcp", addr)
	default:
		// 兜底：当作纯 host:port
		log.Printf("🚀 Magicmail 服务启动于 http://%s", listen)
		return net.Listen("tcp", listen)
	}
}

// listenAndServe 启动服务：主监听阻塞当前协程；若启用并行 TCP，则在独立 goroutine 监听。
func listenAndServe(app *fiber.App, cfg *config.Config) error {
	mainLn, err := buildListener(cfg.Server.Listen)
	if err != nil {
		return err
	}

	// Unix Socket 进程退出时清理文件（主监听为 TCP 时该值为空，无副作用）
	if up := unixSocketPath(cfg.Server.Listen); up != "" {
		defer os.Remove(up)
	}

	// 并行 TCP 监听（飞牛向导 "both" 模式）：不影响主监听进程生命周期
	if cfg.Server.TCPEnabled {
		addr := cfg.Server.TCPAddr
		if addr == "" {
			addr = cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
		}
		go func() {
			log.Printf("🚀 Magicmail TCP 服务启动于 http://%s", addr)
			if err := app.Listen(addr); err != nil {
				log.Printf("⚠️  TCP 并行监听失败（不影响主监听）: %v", err)
			}
		}()
	}

	// 主协程阻塞在主监听器上（进程生命周期由它决定）
	return app.Listener(mainLn)
}

// unixSocketPath 从 unix:// 监听串中提取 socket 路径，非 Unix 监听返回空字符串。
func unixSocketPath(listen string) string {
	if strings.HasPrefix(listen, "unix://") {
		return strings.TrimPrefix(listen, "unix://")
	}
	return ""
}

// logWriter 是全局日志输出目标：
//   - 开发环境 → os.Stderr（终端实时可见）
//   - 生产环境 → 数据卷日志文件 + stderr（文件可用 NAS 文件管理器查看，stderr 保留给 docker logs）
//   - 所有输出再经过 debugFilter：未开启 debug 级别时不写入 [DEBUG] 日志
var logWriter io.Writer = os.Stderr

// debugFilter 根据 enabled 决定是否丢弃含 [DEBUG] 标记的日志行。
// 用于在生产环境默认关闭冗余调试日志，避免日志膨胀、并减少敏感调用链外泄。
type debugFilter struct {
	w       io.Writer
	enabled bool
}

func (f *debugFilter) Write(p []byte) (int, error) {
	if !f.enabled && bytes.Contains(p, []byte("[DEBUG]")) {
		return len(p), nil
	}
	return f.w.Write(p)
}

// setupLogging 根据运行环境配置日志输出：
//   - 开发（isProduction != "true"）：输出到终端 stderr，且默认开启 debug 日志
//   - 生产（isProduction == "true"）：输出到 MAGICMAIL_LOG_FILE 指定的文件（带大小轮转），
//     默认关闭 debug 日志；设置环境变量 MAGICMAIL_LOG_LEVEL=debug 可开启
func setupLogging() {
	log.SetFlags(log.LstdFlags)

	// 确定 debug 级别是否开启
	lvl := strings.ToLower(os.Getenv("MAGICMAIL_LOG_LEVEL"))
	debugEnabled := lvl == "debug"
	if isProduction != "true" && lvl == "" {
		debugEnabled = true // 开发环境默认开启 debug
	}

	// 确定底层写入目标
	var base io.Writer = os.Stderr
	if isProduction == "true" {
		logPath := os.Getenv("MAGICMAIL_LOG_FILE")
		if logPath == "" {
			logPath = filepath.Join(dataDirFromDSN(), "magicmail.log")
		}
		rw, err := newRotatingWriter(logPath, 10*1024*1024, 3) // 单文件上限 10MB，保留 3 个备份
		if err != nil {
			log.Printf("⚠️  无法创建日志文件 %s，回退到 stderr: %v", logPath, err)
			base = os.Stderr
		} else {
			// 同时写文件 + stderr：文件供 NAS 文件管理器查看，stderr 保留给 docker logs
			base = io.MultiWriter(rw, os.Stderr)
			log.Printf("📝 生产环境：日志写入文件 %s（每 10MB 轮转，保留 3 份）", logPath)
		}
	} else {
		log.Printf("🖥️  开发环境：日志输出到终端")
	}

	// 套用 debug 过滤器（debug 关闭时静默 [DEBUG] 日志）
	logWriter = &debugFilter{w: base, enabled: debugEnabled}
	log.SetOutput(logWriter)
}

// dataDirFromDSN 根据 MAGICMAIL_DSN 推断数据目录（默认 data/）
func dataDirFromDSN() string {
	dsn := os.Getenv("MAGICMAIL_DSN")
	if dsn == "" {
		dsn = "data/magicmail.db"
	}
	dir := filepath.Dir(dsn)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// rotatingWriter 基于文件大小的日志轮转写入器，防止日志无限增长占满磁盘
type rotatingWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	backups  int
	f        *os.File
	size     int64
}

func newRotatingWriter(path string, maxBytes int64, backups int) (*rotatingWriter, error) {
	w := &rotatingWriter{path: path, maxBytes: maxBytes, backups: backups}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.f = f
	if fi, err := f.Stat(); err == nil {
		w.size = fi.Size()
	}
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.maxBytes > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotate() error {
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	if w.backups > 0 {
		// 删除最老的备份
		_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.backups))
		// 平移备份：magicmail.log.2 → .3 ... magicmail.log → .1
		for i := w.backups - 1; i >= 1; i-- {
			_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
		}
		_ = os.Rename(w.path, w.path+".1")
	}
	return w.open()
}
