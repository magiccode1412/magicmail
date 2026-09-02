// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package middleware

import (
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestBasePathStrip(t *testing.T) {
	app := fiber.New()
	app.Use(BasePath("/app/magicmail"))
	app.Get("/api/v1/ping", func(c *fiber.Ctx) error {
		return c.SendString(c.Path() + "|base=" + GetBasePath(c) +
			"|gw=" + strconv.FormatBool(IsGatewayRequest(c)))
	})

	for _, tc := range []struct {
		req  string
		want string // 空串表示期望 404
	}{
		{"/app/magicmail/api/v1/ping", "/api/v1/ping|base=/app/magicmail|gw=false"},
		{"/api/v1/ping", "/api/v1/ping|base=|gw=false"},
		// 前缀边界：/app/magicmailx 不应被剥离 → 未注册该路由 → 404
		//（若被误剥离成 /api/v1/ping 则会返回 200，据此可证明边界判定正确）
		{"/app/magicmailx/api/v1/ping", ""},
		// 精确等于前缀 → 剥离为 "/" → 本测试 app 未注册 SPA fallback → 404
		{"/app/magicmail", ""},
		// query 不应影响剥离
		{"/app/magicmail/api/v1/ping?a=1", "/api/v1/ping|base=/app/magicmail|gw=false"},
	} {
		resp, err := app.Test(httptest.NewRequest("GET", tc.req, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if tc.want == "" {
			if resp.StatusCode != fiber.StatusNotFound {
				t.Errorf("req=%s want 404 got %d", tc.req, resp.StatusCode)
			}
			continue
		}
		if string(body) != tc.want {
			t.Errorf("req=%s got=%q want=%q", tc.req, body, tc.want)
		}
	}
}

func TestGatewayIdentityTrustBoundary(t *testing.T) {
	// 替换连接判定以模拟两种入口
	orig := gatewayConnCheck
	defer func() { gatewayConnCheck = orig }()

	app := fiber.New()
	app.Use(BasePath("/app/magicmail"))
	app.Get("/api/v1/auth/fnos/status", func(c *fiber.Ctx) error {
		_, _, ok := GatewayIdentity(c) // 依赖 BasePath 已写入 Locals(viaGateway)
		return c.JSON(fiber.Map{"gateway": ok})
	})

	cases := []struct {
		name string
		gw   bool
		hdr  string
		want string
	}{
		{"unix+带前缀+有Header", true, "1", `{"gateway":true}`},
		{"tcp+带前缀+伪造Header", false, "1", `{"gateway":false}`}, // ★ 伪造无效
		{"tcp+根路径+伪造Header", false, "1", `{"gateway":false}`},
		{"unix+无Header", true, "", `{"gateway":false}`},
	}
	for _, tc := range cases {
		gatewayConnCheck = func(*fiber.Ctx) bool { return tc.gw }
		req := httptest.NewRequest("GET", "/app/magicmail/api/v1/auth/fnos/status", nil)
		if tc.hdr != "" {
			req.Header.Set("X-Trim-Userid", tc.hdr)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		got := strings.TrimSpace(string(body))
		if got != tc.want {
			t.Errorf("%s: got=%s want=%s", tc.name, got, tc.want)
		}
	}
}

// TestBasePathNoopForDocker 保护非飞牛部署（Docker / build.sh / 开发模式）：
// BasePath 为空时中间件必须完全透传，不改写路径、不产生跳转。
func TestBasePathNoopForDocker(t *testing.T) {
	app := fiber.New()
	app.Use(BasePath(""))
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("root|base=" + GetBasePath(c))
	})
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/mails", func(c *fiber.Ctx) error { return c.SendString("mails") })

	for _, path := range []string{"/", "/health", "/mails"} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("path=%s want 200 got %d", path, resp.StatusCode)
		}
		if l := resp.Header.Get(fiber.HeaderLocation); l != "" {
			t.Errorf("path=%s 不应产生 Location，got=%q", path, l)
		}
	}
}
