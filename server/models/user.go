// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package models

import "time"

// 用户角色常量
const (
	RoleAdmin = "admin" // 管理员
	RoleUser  = "user"  // 普通用户
)

// User 用户模型（多用户模式）
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64;not null"`
	PasswordHash string    `json:"-" gorm:"size:255;not null;column:password_hash"` // bcrypt 哈希，不序列化输出
	Role         string    `json:"role" gorm:"size:16;not null;default:'user';comment:角色(admin/user)"`
	FnosUID      string    `json:"-" gorm:"size:64;uniqueIndex;default:'';column:fnos_uid;comment:飞牛用户ID，空表示未绑定（一对一约束）"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// IsAdmin 判断是否为管理员
func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// ---- 请求 DTO ----

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest 注册请求（首次初始化或开放注册时可用）
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=32"`
	Password string `json:"password" validate:"required,min=6,max=64"`
}

// AdminCreateUserRequest 管理员后台创建用户请求
type AdminCreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=32"`
	Password string `json:"password" validate:"required,min=6,max=64"`
	Role     string `json:"role" validate:"omitempty,oneof=admin user"` // 可选，默认普通用户
}

// ---- 飞牛统一网关绑定 DTO ----

// FnosStatusResponse 飞牛登录状态响应
type FnosStatusResponse struct {
	Gateway  bool   `json:"gateway"`  // 是否处于飞牛网关环境（请求带 X-Trim-Userid）
	Bound    bool   `json:"bound"`    // 当前飞牛用户是否已绑定 magicmail 账号
	Username string `json:"username"` // 已绑定时返回绑定的 magicmail 用户名
}

// FnosBindRequest 绑定已有账号请求（飞牛身份从 X-Trim-Userid 获取，不信任 body）
type FnosBindRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// FnosRegisterRequest 注册新账号并绑定请求（飞牛身份从 X-Trim-Userid 获取，不信任 body）
type FnosRegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=32"`
	Password string `json:"password" validate:"required,min=6,max=64"`
}

// ---- 响应 DTO ----

// LoginResponse 登录响应（含 Token）
type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UserResponse 用户列表/详情响应（不暴露密码）
type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthStatusResponse 认证状态响应（用于判断是否需要注册）
type AuthStatusResponse struct {
	SetupRequired    bool   `json:"setup_required"`    // true = 需要注册（尚无用户）
	OpenRegistration bool   `json:"open_registration"` // 是否开放公开注册
	Message          string `json:"message"`
}
