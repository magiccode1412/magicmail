// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package handlers

import (
	"log"

	"magicmail/middleware"
	"magicmail/models"
	"magicmail/services"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler 认证相关 Handler
type AuthHandler struct {
	service *services.AuthService
}

// NewAuthHandler 创建认证 Handler
func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{service: svc}
}

// Login 登录
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数无效: " + err.Error()})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名和密码不能为空"})
	}

	result, err := h.service.Login(req)
	if err != nil {
		msg := "登录失败"
		if err == services.ErrInvalidCredentials {
			msg = "用户名或密码错误"
		}
		return c.Status(401).JSON(fiber.Map{"error": msg, "detail": err.Error()})
	}

	// 若当前处于飞牛统一网关环境，则登录后自动将本账号绑定到该飞牛身份（无需用户手动操作）
	if fnosUID, _, ok := middleware.GatewayIdentity(c); ok && fnosUID != "" {
		if bindErr := h.service.BindToFnOSIfFree(fnosUID, result.Username); bindErr != nil {
			// 绑定失败不影响登录本身，但必须落日志（写 c.Locals 无人消费 = 静默吞错）
			log.Printf("⚠️ [fnos] 登录后自动绑定失败: user=%s err=%v", result.Username, bindErr)
		} else {
			log.Printf("✅ [fnos] 登录后自动绑定完成: user=%s", result.Username)
		}
	}

	return c.JSON(result)
}

// Register 注册（首次初始化，单用户模式）
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数无效: " + err.Error()})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名和密码不能为空"})
	}
	if len(req.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "密码长度至少 6 位"})
	}

	err := h.service.Register(req)
	if err != nil {
		statusCode := 500
		msg := "注册失败"
		switch err {
		case services.ErrRegistrationClosed:
			statusCode = 403
			msg = "公开注册已关闭，仅管理员可创建账号"
		case services.ErrUserExists:
			statusCode = 409
			msg = "账号已存在，请直接登录"
		}
		return c.Status(statusCode).JSON(fiber.Map{"error": msg, "detail": err.Error()})
	}

	// 注册成功后自动登录，返回 Token
	loginReq := models.LoginRequest{Username: req.Username, Password: req.Password}
	result, loginErr := h.service.Login(loginReq)
	if loginErr != nil {
		return c.Status(201).JSON(fiber.Map{"message": "注册成功，请手动登录", "detail": loginErr.Error()})
	}

	// 若当前处于飞牛统一网关环境，则注册后自动将本账号绑定到该飞牛身份（无需用户手动操作）
	if fnosUID, _, ok := middleware.GatewayIdentity(c); ok && fnosUID != "" {
		if bindErr := h.service.BindToFnOSIfFree(fnosUID, result.Username); bindErr != nil {
			log.Printf("⚠️ [fnos] 注册后自动绑定失败: user=%s err=%v", result.Username, bindErr)
		} else {
			log.Printf("✅ [fnos] 注册后自动绑定完成: user=%s", result.Username)
		}
	}

	c.Status(201)
	return c.JSON(result)
}

// Me 返回当前登录用户（受 AuthRequired 保护）。
//
// 前端必须用它做「本地 token 探活」：/auth/status 是公开接口，对任何 token
// 都返回 200，用它探活会让失效 token 永不自愈（表现为登录后所有数据请求 401）。
// user_id 取自 AuthRequired 写入的 Locals（源自已验签的 JWT），不信任任何客户端参数。
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(float64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "认证令牌无效或已过期"})
	}
	// 以数据库为权威来源，避免返回已被降权/注销账号的陈旧信息
	user, err := h.service.GetUserByID(uint(userID))
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "账号已被注销，请重新登录"})
	}
	return c.JSON(models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	})
}

// Status 查询认证状态（是否需要初始化）
func (h *AuthHandler) Status(c *fiber.Ctx) error {
	status := h.service.GetAuthStatus()
	return c.JSON(status)
}

// FnosStatus 查询当前飞牛用户是否已绑定 magicmail 账号
//   - 非网关环境（无 X-Trim-Userid）→ { gateway:false }
//   - 网关环境且已绑定 → { gateway:true, bound:true, username }
//   - 网关环境且未绑定 → { gateway:true, bound:false }
func (h *AuthHandler) FnosStatus(c *fiber.Ctx) error {
	fnosUID, _, ok := middleware.GatewayIdentity(c)
	if !ok {
		return c.JSON(models.FnosStatusResponse{Gateway: false, Bound: false})
	}
	user, err := h.service.GetFnosBind(fnosUID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "查询绑定状态失败", "detail": err.Error()})
	}
	if user == nil {
		return c.JSON(models.FnosStatusResponse{Gateway: true, Bound: false})
	}
	return c.JSON(models.FnosStatusResponse{Gateway: true, Bound: true, Username: user.Username})
}

// FnosLogin 已绑定用户免密登录：直接依据飞牛身份签发 JWT
func (h *AuthHandler) FnosLogin(c *fiber.Ctx) error {
	fnosUID, _, ok := middleware.GatewayIdentity(c)
	if !ok {
		return c.Status(403).JSON(fiber.Map{"error": "非飞牛网关环境，无法使用飞牛登录"})
	}

	user, err := h.service.FnosLogin(fnosUID)
	if err != nil {
		if err == services.ErrFnosNotBound {
			return c.Status(404).JSON(fiber.Map{"error": "该飞牛账号尚未绑定", "not_bound": true})
		}
		return c.Status(500).JSON(fiber.Map{"error": "飞牛登录失败", "detail": err.Error()})
	}

	token, err := h.service.GenerateToken(user)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "签发令牌失败", "detail": err.Error()})
	}
	return c.JSON(models.LoginResponse{Token: token, Username: user.Username, Role: user.Role})
}

// FnosBind 绑定已有 magicmail 账号到飞牛身份（校验原密码）
func (h *AuthHandler) FnosBind(c *fiber.Ctx) error {
	fnosUID, _, ok := middleware.GatewayIdentity(c)
	if !ok {
		return c.Status(403).JSON(fiber.Map{"error": "非飞牛网关环境，无法使用飞牛登录"})
	}

	var req models.FnosBindRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数无效: " + err.Error()})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名和密码不能为空"})
	}

	user, err := h.service.BindExistingByFnOS(fnosUID, req.Username, req.Password)
	if err != nil {
		msg := "绑定失败"
		switch err {
		case services.ErrInvalidCredentials:
			msg = "用户名或密码错误"
		case services.ErrFnosAlreadyBound:
			msg = "该飞牛账号已绑定其他 Magicmail 账号"
		case services.ErrAccountBoundByFnos:
			msg = "该 Magicmail 账号已被其他飞牛账号绑定"
		}
		return c.Status(401).JSON(fiber.Map{"error": msg, "detail": err.Error()})
	}

	token, err := h.service.GenerateToken(user)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "签发令牌失败", "detail": err.Error()})
	}
	return c.JSON(models.LoginResponse{Token: token, Username: user.Username, Role: user.Role})
}

// FnosRegister 注册新 magicmail 账号并绑定飞牛身份
func (h *AuthHandler) FnosRegister(c *fiber.Ctx) error {
	fnosUID, _, ok := middleware.GatewayIdentity(c)
	if !ok {
		return c.Status(403).JSON(fiber.Map{"error": "非飞牛网关环境，无法使用飞牛登录"})
	}

	var req models.FnosRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "请求参数无效: " + err.Error()})
	}
	if req.Username == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "用户名和密码不能为空"})
	}
	if len(req.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{"error": "密码长度至少 6 位"})
	}

	user, err := h.service.RegisterByFnOS(fnosUID, req.Username, req.Password)
	if err != nil {
		statusCode := 500
		msg := "注册失败"
		switch err {
		case services.ErrInvalidCredentials:
			msg = "用户名或密码错误"
		case services.ErrRegistrationClosed:
			statusCode = 403
			msg = "公开注册已关闭，仅管理员可创建账号"
		case services.ErrUserExists:
			statusCode = 409
			msg = "账号已存在，请直接登录或绑定"
		case services.ErrFnosAlreadyBound:
			msg = "该飞牛账号已绑定其他 Magicmail 账号"
		case services.ErrAccountBoundByFnos:
			msg = "该 Magicmail 账号已被其他飞牛账号绑定"
		}
		return c.Status(statusCode).JSON(fiber.Map{"error": msg, "detail": err.Error()})
	}

	token, err := h.service.GenerateToken(user)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "签发令牌失败", "detail": err.Error()})
	}
	return c.Status(201).JSON(models.LoginResponse{Token: token, Username: user.Username, Role: user.Role})
}
