// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package middleware

import (
	"strings"

	"magicmail/models"
	"magicmail/services"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// XAuthTokenHeader 传递 magicmail JWT 的自定义请求头。
//
// ⚠️ 严禁使用 Authorization：飞牛统一网关会占用该头并当成飞牛自己的会话/API 凭证校验，
// 校验失败时网关直接代答「HTTP 200 + 纯文本 invalid token」，请求根本到不了应用的
// Unix Socket（表现为：未登录时一切正常，登录后所有 API 全部失效）。
const XAuthTokenHeader = "X-Auth-Token"

// AuthRequired JWT 认证中间件
func AuthRequired(authService *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var tokenStr string

		// 优先自定义头：飞牛网关下唯一可用通道
		tokenStr = c.Get(XAuthTokenHeader)

		// 回退：Authorization: Bearer <jwt>（仅非飞牛部署 / Docker / curl 调试）
		if tokenStr == "" {
			if authHeader := c.Get(fiber.HeaderAuthorization); authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					tokenStr = strings.TrimSpace(parts[1])
				}
			}
		}

		// ⭐ fallback: 从 query string 获取（用于 SSE 流、文件下载等浏览器直接请求）
		if tokenStr == "" {
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			return c.Status(401).JSON(fiber.Map{"error": "未提供认证令牌"})
		}

		token, err := authService.ParseToken(tokenStr)
		if err != nil || !token.Valid {
			return c.Status(401).JSON(fiber.Map{"error": "认证令牌无效或已过期"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "无法解析认证信息"})
		}

		userID, ok := claims["user_id"].(float64)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "无法解析认证信息"})
		}
		c.Locals("user_id", userID)

		// ⭐ 校验用户是否仍存在：删除用户后，已签发的 JWT 在过期前仍有效，
		// 因此每次请求都在数据库核验用户，不存在则强制 401（前端据此清除登录态）。
		user, err := authService.GetUserByID(uint(userID))
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "账号已被注销，请重新登录"})
		}

		// 以数据库中的权威角色为准，避免角色被降权后旧 token 仍可越权
		c.Locals("username", user.Username)
		c.Locals("role", user.Role)

		return c.Next()
	}
}

// AdminRequired 管理员权限中间件：必须登录且角色为 admin
func AdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role != models.RoleAdmin {
			return c.Status(403).JSON(fiber.Map{"error": "需要管理员权限"})
		}
		return c.Next()
	}
}

// GatewayIdentity 从飞牛统一网关注入的 Header 中读取当前登录的飞牛用户身份。
//
// ⚠️ 安全约束：身份是否可信由「连接来源」决定，而非路径。
// 只有来自统一网关的 Unix Socket 连接才被认定为网关请求（IsGatewayRequest），
// 其它入口（自建 TCP 部署 / Docker / 反向代理 / app.Test）即使携带 X-Trim-Userid
// 也一律不读取、不信任，伪造 Header 无效。
//
// 返回 (fnosUID, username, ok)：ok=false 表示当前并非飞牛网关环境（非 Unix Socket 连接或无 X-Trim-Userid）。
func GatewayIdentity(c *fiber.Ctx) (fnosUID string, username string, ok bool) {
	if !IsGatewayRequest(c) {
		// 非 Unix Socket 入口：一律不认 X-Trim-*，伪造无效
		return "", "", false
	}
	fnosUID = c.Get("X-Trim-Userid")
	if fnosUID == "" {
		return "", "", false
	}
	username = c.Get("X-Trim-Username")
	return fnosUID, username, true
}
