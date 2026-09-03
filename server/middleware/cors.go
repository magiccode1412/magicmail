// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package middleware

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS 配置跨域资源共享中间件
// 开发环境允许所有来源，生产环境通过环境变量配置
func CORS() fiber.Handler {
	allowedOrigins := os.Getenv("MAGICMAIL_CORS_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173,http://localhost:3000,http://127.0.0.1:5173" // Vite 默认端口
	}

	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		// X-Auth-Token 为业务 JWT 通道（Authorization 被飞牛统一网关占用）
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Auth-Token,X-Requested-With",
		ExposeHeaders:    "Content-Disposition,Content-Length",
		AllowCredentials: true,
		MaxAge:           86400, // 24 小时预检缓存
	})
}
