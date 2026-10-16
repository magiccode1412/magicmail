// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package services

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"magicmail/models"

	webpush "github.com/SherClockHolmes/webpush-go"

	"gorm.io/gorm"
)

// SubReq 前端 Push API 订阅请求（Service 层内部使用）
type SubReq struct {
	Endpoint  string `json:"endpoint"`
	P256DH    string `json:"p256dh"`
	Auth      string `json:"auth"`
	UserAgent string `json:"user_agent,omitempty"`
}

// ToPushSubscription 转换为 GORM 模型
func (r *SubReq) ToModel(userID uint) *models.PushSubscription {
	return &models.PushSubscription{
		UserID:    userID,
		Endpoint:  r.Endpoint,
		P256DH:    r.P256DH,
		Auth:      r.Auth,
		UserAgent: r.UserAgent,
	}
}

// PushService Web Push 业务逻辑：订阅管理 + 消息发送
type PushService struct {
	db           *gorm.DB
	vapidPrivate *ecdsa.PrivateKey
	vapidPublic  []byte
	subject      string // VAPID subject (通常是 mailto: 或 https://)
}

// NewPushService 创建 Push Service（需在 EnsureVAPIDKeys 之后调用）
func NewPushService(db *gorm.DB, vapidPriv *ecdsa.PrivateKey, vapidPub []byte, subject string) *PushService {
	return &PushService{
		db:           db,
		vapidPrivate: vapidPriv,
		vapidPublic:  vapidPub,
		subject:      subject,
	}
}

// VAPIDPublicKey 返回 base64url 编码的公钥（供前端使用）
func (s *PushService) VAPIDPublicKey() string {
	return base64.RawURLEncoding.EncodeToString(s.vapidPublic)
}

// --- CRUD ---

// Subscribe 存储或更新用户的 Push Subscription
func (s *PushService) Subscribe(userID uint, req *SubReq) error {
	var existing models.PushSubscription
	result := s.db.Where("user_id = ? AND endpoint = ?", userID, req.Endpoint).
		First(&existing)

	if result.Error == nil {
		return s.db.Model(&existing).Updates(map[string]interface{}{
			"p256dh":     req.P256DH,
			"auth":       req.Auth,
			"user_agent": req.UserAgent,
		}).Error
	}
	sub := req.ToModel(userID)
	return s.db.Create(sub).Error
}

// Unsubscribe 删除指定 endpoint 的订阅
func (s *PushService) Unsubscribe(userID uint, endpoint string) error {
	result := s.db.Where("user_id = ? AND endpoint = ?", userID, endpoint).
		Delete(&models.PushSubscription{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListSubscriptions 获取用户的所有订阅
func (s *PushService) ListSubscriptions(userID uint) ([]models.PushSubscription, error) {
	var subs []models.PushSubscription
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&subs).Error
	return subs, err
}

// --- 发送 ---

// PushPayload 推送消息负载结构
type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Icon  string `json:"icon,omitempty"`
	Tag   string `json:"tag,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// SendNotification 向指定用户的所有活跃订阅发送推送消息
func (s *PushService) SendNotification(userID uint, title, body string, data any) {
	log.Printf("[Push] SendNotification (userID=%d, title=%q)", userID, title)
	payload := PushPayload{
		Title: title,
		Body:  body,
		Icon:  "/icons/icon-192x192.png",
		Tag:   fmt.Sprintf("mail-%d", userID),
		Data:  data,
	}
	s.sendToUserSubscriptions(userID, payload)
}

// SendTest 发送测试推送
func (s *PushService) SendTest(userID uint) error {
	s.SendNotification(userID, "Magicmail 测试推送", "这是一条测试通知，如果您收到说明 Web Push 工作正常！", nil)
	return nil
}

// sendToUserSubscriptions 向用户的所有订阅逐个发送
func (s *PushService) sendToUserSubscriptions(userID uint, payload PushPayload) {
	subs, err := s.ListSubscriptions(userID)
	if err != nil {
		log.Printf("[Push] 查询订阅失败 (userID=%d): %v", userID, err)
		return
	}
	log.Printf("[Push] 找到 %d 个活跃订阅 (userID=%d)", len(subs), userID)
	if len(subs) == 0 {
		log.Printf("[Push] ⚠️ 无活跃订阅，推送被跳过 (userID=%d)。请确认前端已订阅 Web Push！", userID)
		return
	}

	payloadBytes, _ := json.Marshal(payload)

	for _, sub := range subs {
		log.Printf("[Push] → 发送到 sub#%d endpoint=%s...", sub.ID, truncateEndpoint(sub.Endpoint))
		go func(sub models.PushSubscription) {
			if err := s.doSend(&sub, payloadBytes); err != nil {
				log.Printf("[Push] 发送失败 (sub=%d): %v", sub.ID, err)
				s.handleSendError(&sub, err)
			} else {
				log.Printf("[Push] ✓ 发送成功 (sub=%d)", sub.ID)
			}
		}(sub)
	}
}

// doSend 执行单次 Web Push 发送
func (s *PushService) doSend(sub *models.PushSubscription, payload []byte) error {
	wpSub := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256DH,
			Auth:   sub.Auth,
		},
	}

	resp, err := webpush.SendNotification(
		payload,
		wpSub,
		&webpush.Options{
			Subscriber:      s.subject,
			VAPIDPrivateKey: encodeVAPIDPrivate(s.vapidPrivate),
			TTL:             30,
		},
	)
	if resp != nil {
		resp.Body.Close()
	}
	return err
}

// handleSendError 处理发送失败：清理过期/无效的订阅
func (s *PushService) handleSendError(sub *models.PushSubscription, err error) {
	msg := err.Error()
	if isGone(msg) || isInvalid(msg) {
		log.Printf("[Push] 清理过期订阅 (id=%d): %s", sub.ID, msg)
		s.db.Delete(sub)
	}
	if isRateLimited(msg) {
		log.Printf("[Push] 限流保留订阅 (id=%d)", sub.ID)
	}
}

func isGone(m string) bool       { return len(m) >= 3 && m[0:3] == "410" }
func isInvalid(m string) bool    { return containsStr(m, "404") || containsStr(m, "invalid") }
func isRateLimited(m string) bool { return containsStr(m, "429") }

// --- VAPID 密钥管理 ---

// GenerateVAPIDKeyPair 生成 VAPID ECDSA P-256 密钥对
// 返回 (privateKey, rawUncompressedPublicKey[65字节], base64urlPublicKey, error)
func GenerateVAPIDKeyPair() (*ecdsa.PrivateKey, []byte, string, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, "", fmt.Errorf("生成 VAPID 密钥失败: %w", err)
	}

	// ⭐ 关键修复：Web Push 要求原始未压缩 P-256 公钥（65 字节：0x04 + x[32] + y[32]）
	// 而非 x509.MarshalPKIXPublicKey 的 ASN.1/DER 编码格式
	pubRaw := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y)
	if len(pubRaw) != 65 {
		return nil, nil, "", fmt.Errorf("公钥长度异常: 期望 65 字节, 实际 %d", len(pubRaw))
	}

	pubBase64 := base64.RawURLEncoding.EncodeToString(pubRaw)
	return priv, pubRaw, pubBase64, nil
}

// GetVAPIDSubject 从环境变量读取 VAPID subject
func GetVAPIDSubject() string {
	if subj := os.Getenv("MAGICMAIL_VAPID_SUBJECT"); subj != "" {
		return subj
	}
	return "mailto:noreply@magicmail.local"
}

// containsStr 检查字符串是否包含子串
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// truncateEndpoint 截断 endpoint 用于日志（隐藏敏感信息）
func truncateEndpoint(ep string) string {
	if len(ep) > 80 { return ep[:80] + "..." }
	return ep
}

// encodeVAPIDPrivate 将 ECDSA 私钥编码为 base64rawurl 字符串（webpush-go 要求）
func encodeVAPIDPrivate(priv *ecdsa.PrivateKey) string {
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		log.Printf("[Push] 序列化 VAPID 私钥失败: %v", err)
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(privBytes)
}

// --- 全局单例（供 IMAP Worker 等外部包调用）---

var globalPush *PushService

// InitGlobalPush 设置全局 PushService 实例（由 routes.Register 调用）
func InitGlobalPush(svc *PushService) { globalPush = svc }

// SendPushNotification 向用户发送推送通知（便捷函数，供 Worker/SSE 回调使用）
// 如果全局实例未初始化则静默跳过
func SendPushNotification(userID uint, title, body string, data any) {
	if globalPush == nil {
		return
	}
	globalPush.SendNotification(userID, title, body, data)
}
