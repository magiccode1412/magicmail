// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"errors"
	"log"
	"time"

	"magicmail/config"
	"magicmail/imap"
	"magicmail/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证服务
type AuthService struct {
	db        *gorm.DB
	jwtSecret []byte
}

// NewAuthService 创建认证服务实例
func NewAuthService(db *gorm.DB, cfg *config.Config) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: []byte(cfg.Security.JWTSecret),
	}
}

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserExists         = errors.New("管理员已存在，不可重复注册")
	ErrRegistrationClosed = errors.New("公开注册已关闭，仅管理员可创建账号")
	ErrCannotDeleteSelf   = errors.New("不能删除当前登录的管理员账号")
	ErrUserNotFound       = errors.New("用户不存在")
	// 飞牛绑定相关
	ErrFnosUIDEmpty       = errors.New("飞牛用户标识缺失，无法绑定")
	ErrFnosAlreadyBound   = errors.New("该飞牛账号已绑定其他 Magicmail 账号")
	ErrAccountBoundByFnos = errors.New("该 Magicmail 账号已被其他飞牛账号绑定")
	ErrFnosNotBound       = errors.New("该飞牛账号尚未绑定 Magicmail 账号")
)

// HashPassword 对密码进行 bcrypt 哈希
func (s *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// VerifyPassword 验证密码是否匹配哈希值
func (s *AuthService) VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateToken 生成 JWT Token（7天有效期）
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	role := user.Role
	if role == "" {
		role = models.RoleUser
	}
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     role,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ParseToken 解析并验证 JWT Token
func (s *AuthService) ParseToken(tokenStr string) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("无效的签名方法")
		}
		return s.jwtSecret, nil
	})
}

// Login 用户登录
func (s *AuthService) Login(req models.LoginRequest) (*models.LoginResponse, error) {
	var user models.User
	result := s.db.Where("username = ?", req.Username).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, result.Error
	}

	if err := s.VerifyPassword(req.Password, user.PasswordHash); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.GenerateToken(&user)
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{Token: token, Username: user.Username, Role: user.Role}, nil
}

// GetFnosBind 根据飞牛用户 ID 查询是否已绑定 magicmail 账号
//   - 已绑定 → 返回该用户（调用方据此签发 JWT 免密登录）
//   - 未绑定 → 返回 (nil, nil)
func (s *AuthService) GetFnosBind(fnosUID string) (*models.User, error) {
	if fnosUID == "" {
		return nil, ErrFnosUIDEmpty
	}
	var user models.User
	result := s.db.Where("fnos_uid = ?", fnosUID).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &user, nil
}

// FnosLogin 已绑定用户免密登录：直接依据飞牛身份签发 JWT
//   - 未绑定（user==nil）返回 ErrFnosUIDEmpty 语义的未绑定错误，由调用方转 404/引导绑定
func (s *AuthService) FnosLogin(fnosUID string) (*models.User, error) {
	user, err := s.GetFnosBind(fnosUID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrFnosNotBound
	}
	return user, nil
}

// BindExistingByFnOS 将已有 magicmail 账号绑定到飞牛身份（校验原密码后写入 fnos_uid）
//   - 一对一约束：fnos_uid 全局唯一；目标账号的 fnos_uid 必须为空（未被占用）
func (s *AuthService) BindExistingByFnOS(fnosUID, username, password string) (*models.User, error) {
	if fnosUID == "" {
		return nil, ErrFnosUIDEmpty
	}
	// 该飞牛身份是否已绑定过其他账号
	if existing, err := s.GetFnosBind(fnosUID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrFnosAlreadyBound
	}

	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.FnosUID != "" {
		return nil, ErrAccountBoundByFnos
	}
	if err := s.VerifyPassword(password, user.PasswordHash); err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := s.db.Model(&user).Update("fnos_uid", fnosUID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// RegisterByFnOS 注册新 magicmail 账号并绑定飞牛身份
//   - 复用 Register 的「首个用户为 admin / 受开放注册开关约束」逻辑
//   - 注册成功后写入 fnos_uid（一对一约束）
func (s *AuthService) RegisterByFnOS(fnosUID, username, password string) (*models.User, error) {
	if fnosUID == "" {
		return nil, ErrFnosUIDEmpty
	}
	// 该飞牛身份是否已绑定过其他账号
	if existing, err := s.GetFnosBind(fnosUID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrFnosAlreadyBound
	}

	// 注册（含开放注册校验）。复用 createUser 不便，这里走 Register 逻辑后补 fnos_uid。
	regReq := models.RegisterRequest{Username: username, Password: password}
	if err := s.Register(regReq); err != nil {
		return nil, err
	}
	// 注册成功后查回该用户
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&user).Update("fnos_uid", fnosUID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// createUser 内部辅助：创建用户并写入密码哈希
func (s *AuthService) createUser(username, password, role string) (*models.User, error) {
	hashedPassword, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Username:     username,
		PasswordHash: hashedPassword,
		Role:         role,
	}
	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

// Register 注册账号（多用户模式）
//   - 首个注册用户自动成为管理员；
//   - 已有用户时，仅当开放注册开关开启才允许自助注册为普通用户。
func (s *AuthService) Register(req models.RegisterRequest) error {
	var count int64
	s.db.Model(&models.User{}).Count(&count)
	if count == 0 {
		// 首次初始化：创建管理员
		_, err := s.createUser(req.Username, req.Password, models.RoleAdmin)
		return err
	}

	// 已有用户：必须开放注册
	open, err := s.GetOpenRegistration()
	if err != nil {
		return err
	}
	if !open {
		return ErrRegistrationClosed
	}
	_, err = s.createUser(req.Username, req.Password, models.RoleUser)
	return err
}

// GetOpenRegistration 读取开放注册开关
func (s *AuthService) GetOpenRegistration() (bool, error) {
	var cfg models.AppConfig
	if err := s.db.First(&cfg).Error; err != nil {
		return false, err
	}
	return cfg.OpenRegistration, nil
}

// SetOpenRegistration 设置开放注册开关（仅管理员调用）
func (s *AuthService) SetOpenRegistration(enabled bool) error {
	var cfg models.AppConfig
	if err := s.db.First(&cfg).Error; err != nil {
		return err
	}
	return s.db.Model(&cfg).Update("open_registration", enabled).Error
}

// AdminCreateUser 管理员后台创建用户（不受开放注册开关限制，默认普通用户）
func (s *AuthService) AdminCreateUser(req models.AdminCreateUserRequest) (*models.UserResponse, error) {
	role := req.Role
	if role == "" {
		role = models.RoleUser
	}
	user, err := s.createUser(req.Username, req.Password, role)
	if err != nil {
		return nil, err
	}
	return &models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}, nil
}

// ListUsers 列出所有用户（不含密码）
func (s *AuthService) ListUsers() ([]models.UserResponse, error) {
	var users []models.User
	if err := s.db.Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	result := make([]models.UserResponse, 0, len(users))
	for _, u := range users {
		result = append(result, models.UserResponse{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			CreatedAt: u.CreatedAt,
		})
	}
	return result, nil
}

// DeleteUser 删除用户及其全部关联数据（事务级联）
func (s *AuthService) DeleteUser(id uint, currentUserID uint) error {
	if id == currentUserID {
		return ErrCannotDeleteSelf
	}
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 删除该用户的邮件与附件
		var accountIDs []uint
		if err := tx.Model(&models.MailAccount{}).Where("user_id = ?", id).Pluck("id", &accountIDs).Error; err != nil {
			return err
		}
		if len(accountIDs) > 0 {
			if err := tx.Where("account_id IN ?", accountIDs).Delete(&models.Attachment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("account_id IN ?", accountIDs).Delete(&models.Mail{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", id).Delete(&models.MailAccount{}).Error; err != nil {
				return err
			}
		}

		// 先停掉该用户所有邮箱账号的同步 Worker，避免后台线程继续向已删除的数据写入
		if pool := imap.GlobalPool(); pool != nil {
			for _, aid := range accountIDs {
				pool.StopWorker(aid)
			}
		}
		// 草稿 / Webhook / 推送订阅
		if err := tx.Where("user_id = ?", id).Delete(&models.Draft{}).Error; err != nil {
			return err
		}
		// webhook_logs 仅通过 webhook_id 关联，无 user_id 字段，需先按用户查出 webhook 再删除其日志
		var webhookIDs []uint
		if err := tx.Model(&models.Webhook{}).Where("user_id = ?", id).Pluck("id", &webhookIDs).Error; err != nil {
			return err
		}
		if len(webhookIDs) > 0 {
			if err := tx.Where("webhook_id IN ?", webhookIDs).Delete(&models.WebhookLog{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.Webhook{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&models.PushSubscription{}).Error; err != nil {
			return err
		}
		// 最后删除用户本身
		return tx.Delete(&user).Error
	})
}

// GetUserByID 根据 ID 获取用户（不含密码），用于校验 token 持有者是否仍有效
// 管理员删除用户后，已签发的 JWT 无法自行失效，故每次请求需在数据库核验用户仍存在，
// 否则返回 ErrUserNotFound，由中间件强制 401 让前端重新登录。
func (s *AuthService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetAuthStatus 查询认证状态（前端据此决定显示登录还是注册页）
func (s *AuthService) GetAuthStatus() *models.AuthStatusResponse {
	var count int64
	s.db.Model(&models.User{}).Count(&count)

	open, _ := s.GetOpenRegistration()

	if count == 0 {
		return &models.AuthStatusResponse{
			SetupRequired:    true,
			OpenRegistration: open,
			Message:          "欢迎使用 Magicmail，请先创建管理员账号",
		}
	}
	return &models.AuthStatusResponse{
		SetupRequired:    false,
		OpenRegistration: open,
		Message:          "已就绪",
	}
}

// SeedDefaultUser 开发环境：自动创建默认管理员（仅在无用户时生效）
func (s *AuthService) SeedDefaultUser(username, password string) error {
	var count int64
	s.db.Model(&models.User{}).Count(&count)
	if count > 0 {
		return nil // 已有用户则跳过
	}

	hashedPassword, err := s.HashPassword(password)
	if err != nil {
		return err
	}

	user := &models.User{
		Username:     username,
		PasswordHash: hashedPassword,
		Role:         models.RoleAdmin,
	}

	err = s.db.Create(user).Error
	if err != nil {
		return err
	}

	log.Printf("✅ 已创建默认管理员账号: %s", username)
	return nil
}
