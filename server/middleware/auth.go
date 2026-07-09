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

// AuthRequired JWT 认证中间件
func AuthRequired(authService *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var tokenStr string

		// 优先从 Authorization header 获取
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenStr = parts[1]
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
