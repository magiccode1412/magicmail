// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// 请求级 Locals key
const (
	localsBasePath   = "basePath"   // 本次请求被剥离的公开前缀（如 /app/magicmail）；未剥离为 ""
	localsViaGateway = "viaGateway" // 本次请求是否来自飞牛统一网关（Unix Socket 连接）
)

// gatewayConnCheck 判定连接是否来自统一网关：只有 Unix Socket 连接才是网关。
// 抽成包级变量以便单测替换（app.Test 用内存 listener，拿不到 unix 连接）。
var gatewayConnCheck = func(c *fiber.Ctx) bool {
	conn := c.Context().Conn()
	if conn == nil {
		return false
	}
	if addr := conn.RemoteAddr(); addr != nil {
		return addr.Network() == "unix"
	}
	return false
}

// BasePath 剥离飞牛统一网关透传的公开前缀（manifest.gatewayPrefix），
// 使后端只需维护一套根路由：/api/v1/**、/health、静态资源、SPA fallback。
//
// ⚠️ 注册顺序约束：必须在任何路由（app.Get / app.Group）之前 app.Use。
// Fiber 的路由栈只前进不回退，注册晚于路由的中间件不参与这些路由的匹配。
//
// 行为：
//   - 路径 == prefix 或以 prefix+"/" 开头 → 剥离，Locals(basePath)=prefix
//   - 其余路径（含 /app/magicmailx/...）→ 原样，Locals(basePath)=""
//   - prefix 为空（Docker / 非飞牛部署）→ 完全透传，仅写 Locals
//   - 无论来源，均写入 Locals(viaGateway)=是否为 Unix Socket 连接
func BasePath(prefix string) fiber.Handler {
	canonical := ""
	if p := strings.Trim(prefix, "/"); p != "" {
		canonical = "/" + p
	}

	return func(c *fiber.Ctx) error {
		c.Locals(localsViaGateway, gatewayConnCheck(c))

		if canonical == "" {
			// 非飞牛部署：无前缀概念，保持零行为差异
			c.Locals(localsBasePath, "")
			return c.Next()
		}

		p := c.Path()
		if p == canonical || strings.HasPrefix(p, canonical+"/") {
			rest := strings.TrimPrefix(p, canonical)
			if rest == "" || rest[0] != '/' {
				rest = "/" + rest
			}
			c.Locals(localsBasePath, canonical)
			c.Path(rest) // 重写路径 → 后续栈内路由按剥离后的路径匹配
		} else {
			c.Locals(localsBasePath, "")
		}
		return c.Next()
	}
}

// GetBasePath 返回本次请求被剥离的公开前缀（未剥离为 ""）。
// 生成「对外可见的绝对 URL / 重定向」时必须用它把前缀补回去。
func GetBasePath(c *fiber.Ctx) string {
	s, _ := c.Locals(localsBasePath).(string)
	return s
}

// IsGatewayRequest 判定请求是否来自飞牛统一网关（Unix Socket 连接）。
// 非 Unix Socket 入口（自建 TCP 部署 / app.Test）即使携带 X-Trim-* 也返回 false。
func IsGatewayRequest(c *fiber.Ctx) bool {
	b, _ := c.Locals(localsViaGateway).(bool)
	return b
}
