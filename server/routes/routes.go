// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package routes

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"magicmail/config"
	"magicmail/embedfs"
	"magicmail/handlers"
	"magicmail/middleware"
	"magicmail/models"
	"magicmail/notifier"
	"magicmail/services"
	"magicmail/sse"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Register 注册所有 API 路由
func Register(app *fiber.App, db *gorm.DB) {
	cfg := config.Load()

	// 全局 CORS 中间件
	app.Use(middleware.CORS())

	// --- 初始化 Service 层（仅一次，避免重复 Seed / 重复注册单例） ---
	accountService := services.NewAccountService(db, cfg)
	mailService := services.NewMailService(db, cfg)
	attachmentService := services.NewAttachmentService(db)
	webhookService := services.NewWebhookService(db)
	authService := services.NewAuthService(db, cfg)
	healthCheckService := services.NewHealthCheckService(db) // 健康检查服务

	// 初始化 VAPID 密钥 + PushService（首次自动生成 ECDSA P-256 密钥对）
	pushPriv, pushPub, _ := EnsureVAPIDKeys(db)
	pushSubject := services.GetVAPIDSubject()
	pushService := services.NewPushService(db, pushPriv, pushPub, pushSubject)
	services.InitGlobalPush(pushService)                         // 注册全局单例供外部调用
	notifier.RegisterPushNotifier(services.SendPushNotification) // 注册推送回调供 Worker 调用

	// 开发环境自动创建默认管理员账号（仅在无用户时生效）
	if isDevMode() {
		authService.SeedDefaultUser("admin", "admin123")
	}

	// --- 初始化 Handler 层 ---
	accountHandler := handlers.NewAccountHandler(accountService, healthCheckService)
	oauthHandler := handlers.NewOAuth2Handler()
	mailHandler := handlers.NewMailHandler(mailService)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentService)
	webhookHandler := handlers.NewWebhookHandler(webhookService)
	authHandler := handlers.NewAuthHandler(authService)
	draftHandler := handlers.NewDraftHandler(services.NewDraftService(db))
	userHandler := handlers.NewUserHandler(authService)
	settingsHandler := handlers.NewSettingsHandler(authService)
	pushHandler := handlers.NewPushHandler(pushService)

	// 认证中间件实例
	authMiddleware := middleware.AuthRequired(authService)

	// 兼容两种部署形态：
	//   1. 根路径（Docker / 旧部署，MAGICMAIL_BASE_PATH 为空）
	//   2. 带基础路径前缀（飞牛统一网关，MAGICMAIL_BASE_PATH=/app/magicmail）
	// 无论前端网关是「透传前缀」还是「剥离前缀」，后端都注册两套路由，全部兼容。
	prefixes := []string{""}
	if bp := strings.Trim(cfg.Server.BasePath, "/"); bp != "" {
		prefixes = append(prefixes, "/"+bp)
	}

	for _, prefix := range prefixes {
		registerAppRoutes(app, prefix, routeDeps{
			accountHandler:   accountHandler,
			oauthHandler:     oauthHandler,
			mailHandler:      mailHandler,
			attachmentHandler: attachmentHandler,
			webhookHandler:   webhookHandler,
			authHandler:      authHandler,
			draftHandler:     draftHandler,
			userHandler:      userHandler,
			settingsHandler:  settingsHandler,
			pushHandler:      pushHandler,
			authMiddleware:   authMiddleware,
		})
	}

	// 前端静态资源与 SPA fallback
	if isEmbedded() {
		// 生产（嵌入式）：注册一次，内部兼容透传/剥离两种前缀
		serveFrontendOnce(app, &cfg.Server)
	} else {
		// 开发模式：从磁盘 ./dist 读取静态资源
		serveFrontend(app, &cfg.Server)
	}
}

// routeDeps 承载已初始化的 Handler 与中间件，供不同前缀的路由注册复用
type routeDeps struct {
	accountHandler    *handlers.AccountHandler
	oauthHandler      *handlers.OAuth2Handler
	mailHandler       *handlers.MailHandler
	attachmentHandler *handlers.AttachmentHandler
	webhookHandler    *handlers.WebhookHandler
	authHandler       *handlers.AuthHandler
	draftHandler      *handlers.DraftHandler
	userHandler       *handlers.UserHandler
	settingsHandler   *handlers.SettingsHandler
	pushHandler       *handlers.PushHandler
	authMiddleware    fiber.Handler
}

// registerAppRoutes 在指定前缀下注册全部 API、健康检查、静态文件与 SPA fallback
func registerAppRoutes(app *fiber.App, prefix string, d routeDeps) {
	api := app.Group(prefix + "/api/v1")

	// ============================================================
	//  公开接口：无需认证
	// ============================================================
	authGroup := api.Group("/auth")
	authGroup.Post("/login", d.authHandler.Login)
	authGroup.Post("/register", d.authHandler.Register)
	authGroup.Get("/status", d.authHandler.Status)

	// 飞牛统一网关登录入口（公开，但身份只信 X-Trim-* Header，伪造无效）
	//   - /fnos/status   查询当前飞牛用户是否已绑定
	//   - /fnos/login    已绑定用户免密登录（签发 JWT）
	//   - /fnos/bind     绑定已有账号（校验原密码）
	//   - /fnos/register 注册新账号并绑定
	authGroup.Get("/fnos/status", d.authHandler.FnosStatus)
	authGroup.Post("/fnos/login", d.authHandler.FnosLogin)
	authGroup.Post("/fnos/bind", d.authHandler.FnosBind)
	authGroup.Post("/fnos/register", d.authHandler.FnosRegister)

	// ============================================================
	//  受保护接口：需要 JWT Token
	// ============================================================
	protected := api.Group("")
	protected.Use(d.authMiddleware)
	// ============================================================
	//  OAuth2 授权 API（设备码流）
	// ============================================================
	oauthGroup := protected.Group("/oauth")
	oauthGroup.Post("/:provider/device-code", d.oauthHandler.DeviceCode)
	oauthGroup.Post("/:provider/poll", d.oauthHandler.PollToken)

	// ============================================================
	//  邮箱管理 API
	// ============================================================
	accounts := protected.Group("/accounts")
	accounts.Get("", d.accountHandler.List)
	accounts.Get("/:id", d.accountHandler.Get)
	accounts.Post("", d.accountHandler.Create)
	accounts.Put("/:id", d.accountHandler.Update)
	accounts.Delete("/:id", d.accountHandler.Delete)
	accounts.Post("/test-connection", d.accountHandler.TestConnection)
	accounts.Post("/:id/sync", d.accountHandler.TriggerSync)
	accounts.Put("/:id/status", d.accountHandler.ToggleStatus)
	accounts.Get("/health", d.accountHandler.HealthCheck) // 健康检查端点

	mails := protected.Group("/mails")

	// ============================================================
	//  SSE 实时推送 API（需认证）- 必须在 /:id 之前注册，否则 "stream" 会被 :id 捕获
	// ============================================================
	mails.Get("/stream", sse.StreamHandler)             // SSE 邮件更新推送流
	mails.Get("/stream/health", sse.HealthCheckHandler) // SSE 服务健康检查

	// ============================================================
	//  邮件管理 API
	// ============================================================
	mails.Get("", d.mailHandler.List)
	mails.Get("/stats", d.mailHandler.GetStats)
	mails.Post("/send", d.mailHandler.Send)
	mails.Get("/:id", d.mailHandler.Get)
	mails.Put("/:id/read", d.mailHandler.MarkAsRead)
	mails.Put("/:id/star", d.mailHandler.MarkAsStarred)
	mails.Delete("/:id", d.mailHandler.Delete)
	mails.Post("/batch-delete", d.mailHandler.BatchDelete)
	mails.Post("/batch-read", d.mailHandler.BatchMarkAsRead)
	mails.Post("/mark-all-read", d.mailHandler.MarkAllAsRead)

	// ============================================================
	//  草稿 API
	// ============================================================
	drafts := protected.Group("/drafts")
	drafts.Get("", d.draftHandler.List)
	drafts.Post("", d.draftHandler.Save)
	drafts.Get("/:id", d.draftHandler.Get)
	drafts.Put("/:id", d.draftHandler.Save)
	drafts.Delete("/:id", d.draftHandler.Delete)
	drafts.Post("/batch-delete", d.draftHandler.BatchDelete)

	// ============================================================
	//  附件 API
	// ============================================================
	attachments := protected.Group("/attachments")
	attachments.Get("/mail/:mail_id", d.attachmentHandler.ListByMailID)
	attachments.Get("/:id/download", d.attachmentHandler.Download)

	// ============================================================
	//  Webhook 通知 API
	// ============================================================
	webhooks := protected.Group("/webhooks")
	webhooks.Get("", d.webhookHandler.List)
	webhooks.Post("", d.webhookHandler.Create)
	// 静态路由必须在参数路由之前注册（避免 /simulate-mail 被 :id 捕获）
	webhooks.Post("/simulate-mail", d.webhookHandler.SimulateMailReceived)
	webhooks.Get("/:id", d.webhookHandler.Get)
	webhooks.Put("/:id", d.webhookHandler.Update)
	webhooks.Delete("/:id", d.webhookHandler.Delete)
	webhooks.Post("/:id/test", d.webhookHandler.Test)
	webhooks.Get("/:id/logs", d.webhookHandler.GetLogs)

	// ============================================================
	//  Web Push 推送 API
	// ============================================================
	api.Get("/push/vapid-public-key", d.pushHandler.GetVAPIDPublicKey) // 公开：获取 VAPID 公钥
	push := protected.Group("/push")
	push.Post("/subscribe", d.pushHandler.Subscribe)            // 订阅推送
	push.Post("/unsubscribe", d.pushHandler.Unsubscribe)        // 取消订阅
	push.Get("/subscriptions", d.pushHandler.ListSubscriptions) // 列出订阅
	push.Post("/test", d.pushHandler.SendTest)                  // 测试推送

	// ============================================================
	//  管理员专属接口（需认证 + 管理员权限）
	// ============================================================
	admin := api.Group("")
	admin.Use(d.authMiddleware)
	admin.Use(middleware.AdminRequired())

	// 用户管理
	adminUsers := admin.Group("/admin/users")
	adminUsers.Get("", d.userHandler.List)          // 用户列表
	adminUsers.Post("", d.userHandler.Create)       // 后台创建用户
	adminUsers.Delete("/:id", d.userHandler.Delete) // 删除用户（含关联数据）

	// 开放注册开关配置
	adminSettings := admin.Group("/settings")
	adminSettings.Get("/open-registration", d.settingsHandler.GetOpenRegistration)
	adminSettings.Put("/open-registration", d.settingsHandler.SetOpenRegistration)

	// 健康检查端点
	app.Get(prefix+"/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "magicmail",
			"version": "1.0.0",
		})
	})
}

// serveFrontendOnce 注册一次全局静态文件服务与 SPA fallback。
// 飞牛统一网关可能「透传前缀」(/app/magicmail/assets/...) 或「剥离前缀」(/assets/...)，
// 该中间件内部同时兼容两种形态，确保无论哪种都能正确返回嵌入的前端资源。
func serveFrontendOnce(app *fiber.App, cfg *config.ServerConfig) {
	if !isEmbedded() {
		// 开发/磁盘模式由调用方另行处理，这里仅在嵌入式时注册内存服务
		return
	}

	distSub, err := fs.Sub(embedfs.DistFS, "dist")
	if err != nil {
		return
	}

	// 飞牛基础路径（已去掉首尾斜杠），可能为空
	bp := strings.Trim(cfg.BasePath, "/")

	// --- 静态资源中间件（注册一次，兼容两种前缀） ---
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() != "GET" && c.Method() != "HEAD" {
			return c.Next()
		}
		p := c.Path()

		// 跳过 API 与健康检查端点
		if strings.HasPrefix(p, "/api/") || p == "/health" {
			return c.Next()
		}

		// 可能的内部相对路径候选：原始路径 + （若带前缀）剥离前缀后的路径
		candidates := []string{p}
		if bp != "" {
			prefixed := "/" + bp
			if rel := strings.TrimPrefix(p, prefixed); rel != p {
				candidates = append(candidates, rel) // 透传前缀情况
			}
		}

		for _, cand := range candidates {
			requested := strings.TrimPrefix(cand, "/")
			clean := path.Clean(requested)
			if !fs.ValidPath(clean) || strings.HasPrefix(clean, "..") {
				continue
			}
			if clean == "" || clean == "." {
				continue // 根路径交给 SPA fallback
			}
			if data, err := fs.ReadFile(distSub, clean); err == nil {
				c.Type(path.Ext(clean))
				return c.Send(data)
			}
		}
		return c.Next() // 未命中静态资源，交给 SPA fallback
	})

	// --- SPA fallback：未命中静态资源 / API 时返回 index.html ---
	// 兼容网关「透传前缀」(/app/magicmail/xxx) 与「剥离前缀」(/xxx) 两种形态。
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() != "GET" && c.Method() != "HEAD" {
			return c.Status(404).JSON(fiber.Map{"error": "Not Found"})
		}
		p := c.Path()
		// API 与健康检查已由前序路由注册；未匹配到的 API 路径返回 404 JSON
		if strings.HasPrefix(p, "/api/") || p == "/health" {
			return c.Status(404).JSON(fiber.Map{"error": "Not Found"})
		}
		// 其余 GET 均视为 SPA 前端路由，返回 index.html
		return serveIndexHTML(c)
	})
}

// isDevMode 判断是否为开发环境
func isDevMode() bool {
	mode := os.Getenv("MAGICMAIL_ENV")
	return mode == "dev" || mode == "development"
}

// EnsureVAPIDKeys 确保 VAPID 密钥对存在：
//   - 首次启动时自动生成 ECDSA P-256 密钥对
//   - DER 编码后 base64 存入 AppConfig 表
//   - 支持环境变量 MAGICMAIL_VAPID_PUBLIC_KEY / MAGICMAIL_VAPID_PRIVATE_KEY 覆盖
//
// 返回 (privateKey, publicKeyDER, error)
func EnsureVAPIDKeys(db *gorm.DB) (*ecdsa.PrivateKey, []byte, error) {
	envPub := os.Getenv("MAGICMAIL_VAPID_PUBLIC_KEY")
	envPriv := os.Getenv("MAGICMAIL_VAPID_PRIVATE_KEY")

	var cfg models.AppConfig
	result := db.First(&cfg)

	if result.Error != nil {
		// 首次启动：环境变量 > 自动生成
		priv, pubBytes, pubBase64, err := services.GenerateVAPIDKeyPair()
		if err != nil {
			return nil, nil, err
		}

		privDER, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			return nil, nil, fmt.Errorf("序列化 VAPID 私钥失败: %w", err)
		}
		privBase64 := base64.StdEncoding.EncodeToString(privDER)

		usePub, usePriv := pubBase64, privBase64
		source := "自动生成"

		if envPub != "" {
			usePub = envPub
			source = "环境变量"
		}
		if envPriv != "" {
			usePriv = envPriv
			source = "环境变量"
		}

		appCfg := models.AppConfig{
			JWTSecret:       "", // 由 EnsureSecuritySecrets 处理
			EncryptionKey:   "",
			VAPIDPublicKey:  usePub,
			VAPIDPrivateKey: usePriv,
		}
		if err := db.Create(&appCfg).Error; err != nil {
			return nil, nil, fmt.Errorf("保存 VAPID 密钥失败: %w", err)
		}

		log.Printf("🔑 VAPID 密钥已生成（来源：%s）", source)

		return priv, pubBytes, nil
	}

	// 已有记录：加载或覆盖
	pubB64 := cfg.VAPIDPublicKey
	privB64 := cfg.VAPIDPrivateKey
	log.Printf("[VAPID] DB 已有记录 (ID=%d), pub_present=%v priv_len=%d", cfg.ID, pubB64 != "", len(privB64))

	if envPub != "" && envPub != cfg.VAPIDPublicKey {
		pubB64 = envPub
		db.Model(&cfg).Update("vapid_public_key", envPub)
	}
	if envPriv != "" && envPriv != cfg.VAPIDPrivateKey {
		privB64 = envPriv
		db.Model(&cfg).Update("vapid_private_key", envPriv)
	}

	// 公钥或私钥为空（之前初始化不完整）：自动重新生成
	if pubB64 == "" || privB64 == "" {
		log.Printf("[VAPID] 检测到密钥缺失，正在重新生成...")
		priv, pubBytes, pubBase64, err := services.GenerateVAPIDKeyPair()
		if err != nil {
			return nil, nil, fmt.Errorf("重新生成 VAPID 密钥失败: %w", err)
		}
		privDER, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			return nil, nil, fmt.Errorf("序列化 VAPID 私钥失败: %w", err)
		}
		privBase64 := base64.StdEncoding.EncodeToString(privDER)

		usePub, usePriv := pubBase64, privBase64
		if envPub != "" {
			usePub = envPub
		}
		if envPriv != "" {
			usePriv = envPriv
		}

		result := db.Model(&cfg).Updates(map[string]interface{}{
			"vapid_public_key":  usePub,
			"vapid_private_key": usePriv,
		})
		if result.Error != nil {
			log.Printf("[VAPID] ⚠️ 写入 VAPID 密钥失败: %v", result.Error)
		} else {
			log.Printf("[VAPID] ✓ VAPID 密钥已写入 DB (影响行数: %d)", result.RowsAffected)
		}
		pubB64, privB64 = usePub, usePriv

		log.Printf("🔑 VAPID 密钥已重新生成")
		return priv, pubBytes, nil
	}

	// 解码私钥
	privDER, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		return nil, nil, fmt.Errorf("解码 VAPID 私钥失败: %w", err)
	}
	priv, err := x509.ParseECPrivateKey(privDER)
	if err != nil {
		return nil, nil, fmt.Errorf("解析 VAPID 私钥失败: %w", err)
	}

	// 解码公钥
	pubBytes, err := base64.RawURLEncoding.DecodeString(pubB64)
	if err != nil {
		return nil, nil, fmt.Errorf("解码 VAPID 公钥失败: %w", err)
	}

	return priv, pubBytes, nil
}

// isEmbedded 检查前端产物是否已嵌入二进制
func isEmbedded() bool {
	_, err := fs.Stat(embedfs.DistFS, "dist/index.html")
	return err == nil
}

// serveFrontend 仅用于开发模式：从磁盘 ./dist 读取静态资源（配合 Vite dev server）。
// 生产模式的静态服务由 serveFrontendOnce 基于 embed.FS 统一提供。
func serveFrontend(app *fiber.App, cfg *config.ServerConfig) {
	if isEmbedded() || !isDevMode() {
		return
	}
	bp := strings.Trim(cfg.BasePath, "/")
	if bp == "" {
		app.Static("/", "./dist")
	} else {
		// 开发环境带前缀时同样从根目录读取静态资源
		app.Group("/"+bp).Static("/", "./dist")
	}
}

// serveIndexHTML 返回 SPA 入口文件（embed.FS 或磁盘 ./dist）。
func serveIndexHTML(c *fiber.Ctx) error {
	if isEmbedded() {
		data, err := fs.ReadFile(embedfs.DistFS, "dist/index.html")
		if err != nil {
			return c.Status(500).SendString("frontend not embedded")
		}
		c.Type("html")
		return c.Send(data)
	}
	return c.SendFile("./dist/index.html")
}
