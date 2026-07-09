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

	api := app.Group("/api/v1")

	// --- 初始化 Service 层 ---
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
	services.InitGlobalPush(pushService) // 注册全局单例供外部调用
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

	// ============================================================
	//  公开接口：无需认证
	// ============================================================
	authGroup := api.Group("/auth")
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/register", authHandler.Register)
	authGroup.Get("/status", authHandler.Status)

	// ============================================================
	//  受保护接口：需要 JWT Token
	// ============================================================
	protected := api.Group("")
	protected.Use(authMiddleware)
	// ============================================================
	//  OAuth2 授权 API（设备码流）
	// ============================================================
	oauthGroup := protected.Group("/oauth")
	oauthGroup.Post("/:provider/device-code", oauthHandler.DeviceCode)
	oauthGroup.Post("/:provider/poll", oauthHandler.PollToken)

	// ============================================================
	//  邮箱管理 API
	// ============================================================
	accounts := protected.Group("/accounts")
	accounts.Get("", accountHandler.List)
	accounts.Get("/:id", accountHandler.Get)
	accounts.Post("", accountHandler.Create)
	accounts.Put("/:id", accountHandler.Update)
	accounts.Delete("/:id", accountHandler.Delete)
	accounts.Post("/test-connection", accountHandler.TestConnection)
	accounts.Post("/:id/sync", accountHandler.TriggerSync)
	accounts.Put("/:id/status", accountHandler.ToggleStatus)
	accounts.Get("/health", accountHandler.HealthCheck) // 健康检查端点

	mails := protected.Group("/mails")

	// ============================================================
	//  SSE 实时推送 API（需认证）- 必须在 /:id 之前注册，否则 "stream" 会被 :id 捕获
	// ============================================================
	mails.Get("/stream", sse.StreamHandler)                // SSE 邮件更新推送流
	mails.Get("/stream/health", sse.HealthCheckHandler)     // SSE 服务健康检查

	// ============================================================
	//  邮件管理 API
	// ============================================================
	mails.Get("", mailHandler.List)
	mails.Get("/stats", mailHandler.GetStats)
	mails.Post("/send", mailHandler.Send)
	mails.Get("/:id", mailHandler.Get)
	mails.Put("/:id/read", mailHandler.MarkAsRead)
	mails.Put("/:id/star", mailHandler.MarkAsStarred)
	mails.Delete("/:id", mailHandler.Delete)
	mails.Post("/batch-delete", mailHandler.BatchDelete)
	mails.Post("/batch-read", mailHandler.BatchMarkAsRead)
	mails.Post("/mark-all-read", mailHandler.MarkAllAsRead)

	// ============================================================
	//  草稿 API
	// ============================================================
	drafts := protected.Group("/drafts")
	drafts.Get("", draftHandler.List)
	drafts.Post("", draftHandler.Save)
	drafts.Get("/:id", draftHandler.Get)
	drafts.Put("/:id", draftHandler.Save)
	drafts.Delete("/:id", draftHandler.Delete)
	drafts.Post("/batch-delete", draftHandler.BatchDelete)

	// ============================================================
	//  附件 API
	// ============================================================
	attachments := protected.Group("/attachments")
	attachments.Get("/mail/:mail_id", attachmentHandler.ListByMailID)
	attachments.Get("/:id/download", attachmentHandler.Download)

	// ============================================================
	//  Webhook 通知 API
	// ============================================================
	webhooks := protected.Group("/webhooks")
	webhooks.Get("", webhookHandler.List)
	webhooks.Post("", webhookHandler.Create)
	// 静态路由必须在参数路由之前注册（避免 /simulate-mail 被 :id 捕获）
	webhooks.Post("/simulate-mail", webhookHandler.SimulateMailReceived)
	webhooks.Get("/:id", webhookHandler.Get)
	webhooks.Put("/:id", webhookHandler.Update)
	webhooks.Delete("/:id", webhookHandler.Delete)
	webhooks.Post("/:id/test", webhookHandler.Test)
	webhooks.Get("/:id/logs", webhookHandler.GetLogs)

	// ============================================================
	//  Web Push 推送 API
	// ============================================================
	api.Get("/push/vapid-public-key", pushHandler.GetVAPIDPublicKey) // 公开：获取 VAPID 公钥
	push := protected.Group("/push")
	push.Post("/subscribe", pushHandler.Subscribe)       // 订阅推送
	push.Post("/unsubscribe", pushHandler.Unsubscribe)   // 取消订阅
	push.Get("/subscriptions", pushHandler.ListSubscriptions) // 列出订阅
	push.Post("/test", pushHandler.SendTest)             // 测试推送

	// ============================================================
	//  管理员专属接口（需认证 + 管理员权限）
	// ============================================================
	admin := api.Group("")
	admin.Use(authMiddleware)
	admin.Use(middleware.AdminRequired())

	// 用户管理
	adminUsers := admin.Group("/admin/users")
	adminUsers.Get("", userHandler.List)          // 用户列表
	adminUsers.Post("", userHandler.Create)       // 后台创建用户
	adminUsers.Delete("/:id", userHandler.Delete) // 删除用户（含关联数据）

	// 开放注册开关配置
	adminSettings := admin.Group("/settings")
	adminSettings.Get("/open-registration", settingsHandler.GetOpenRegistration)
	adminSettings.Put("/open-registration", settingsHandler.SetOpenRegistration)

	// 健康检查端点
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "magicmail",
			"version": "1.0.0",
		})
	})

	// 前端静态文件服务
	serveFrontend(app)

	// SPA fallback: 所有未匹配路由返回 index.html
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() == "GET" {
			return serveIndexHTML(c)
		}
		return c.Status(404).JSON(fiber.Map{"error": "Not Found"})
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

		if envPub != "" { usePub = envPub; source = "环境变量" }
		if envPriv != "" { usePriv = envPriv; source = "环境变量" }

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
		if envPub != "" { usePub = envPub }
		if envPriv != "" { usePriv = envPriv }

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

// serveFrontend 根据环境选择静态文件服务方式：
//   - 生产环境：从 embed.FS 读取（已编译进二进制）
//   - 开发环境：从磁盘 ./dist 读取
func serveFrontend(app *fiber.App) {
	if isEmbedded() {
		// 提取 dist 子目录
		distSub, err := fs.Sub(embedfs.DistFS, "dist")
		if err != nil {
			return
		}
		// 手动从 embed.FS 读取并返回静态资源
		app.Use(func(c *fiber.Ctx) error {
			// 跳过已注册的路由路径
			p := c.Path()
			if p == "/health" || (len(p) > 4 && p[:5] == "/api/") {
				return c.Next()
			}

			// 安全清理: 防止路径穿越（embed.FS 使用正斜杠，必须用 path 而非 filepath）
			requested := strings.TrimPrefix(p, "/")
			clean := path.Clean(requested)
			if !fs.ValidPath(clean) || strings.HasPrefix(clean, "..") {
				return c.Status(400).SendString("invalid path")
			}
			if clean == "" || clean == "." {
				return c.Next() // 让 SPA fallback 处理根路径
			}

			data, err := fs.ReadFile(distSub, clean)
			if err != nil {
				// 文件不存在，交给后续中间件处理（SPA fallback）
				return c.Next()
			}

			// 根据扩展名设置 MIME 类型
			ext := path.Ext(clean)
			c.Type(ext)
			return c.Send(data)
		})
	} else if isDevMode() {
		// 开发模式：从磁盘读取，配合 Vite dev server 代理
		app.Static("/", "./dist")
	}
}

// serveIndexHTML 返回 SPA 入口文件
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
