// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package imap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"time"

	"magicmail/config"
	"magicmail/models"
	"magicmail/oauth2"
	pop3pkg "magicmail/pop3"
	"magicmail/proxy"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"gorm.io/gorm"
)

// MailClient 统一邮件客户端接口（IMAP / POP3 共用）
type MailClient interface {
	Authenticate() error
	Close()
}

// IMAPClient 封装 go-imap/v2 客户端连接，提供连接/认证/关闭等基础操作
type IMAPClient struct {
	Client  *imapclient.Client // 底层 IMAP 客户端
	Account *models.MailAccount
	config  *config.Config
}

// NewIMAPClient 创建新的 IMAP 邮件连接实例
func NewIMAPClient(account *models.MailAccount, cfg *config.Config) (*IMAPClient, error) {
	host := account.ImapHost
	port := account.Port
	addr := fmt.Sprintf("%s:%d", host, port)

	// 构建 TLS 配置（默认使用 TLS）
	// 注意：Go 1.24+ 默认不再包含 AES-CBC 等旧版密码套件，
	// 国内邮箱服务器（新浪/163等）可能仍需这些套件，此处显式指定以确保兼容。
	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	// 获取自定义 Dialer（代理）
	customDialer, err := proxy.Dialer(account.ProxyEnabled, account.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理配置错误: %w", err)
	}

	var client *imapclient.Client
	if customDialer != nil {
		// 通过代理建立 TCP 连接，再包装 TLS
		conn, err := customDialer("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("通过代理连接 %s 失败: %w", addr, err)
		}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("TLS 握手失败 (%s): %w", addr, err)
		}
		client = imapclient.New(tlsConn, &imapclient.Options{})
	} else {
		// 直连
		client, err = imapclient.DialTLS(addr, &imapclient.Options{
			TLSConfig: tlsConfig,
		})
		if err != nil {
			return nil, fmt.Errorf("连接 %s 失败: %w", addr, err)
		}
	}

	return &IMAPClient{
		Client:  client,
		Account: account,
		config:  cfg,
	}, nil
}

// NewMailClient 根据协议类型创建对应的邮件客户端（IMAP / POP3）
func NewMailClient(account *models.MailAccount, cfg *config.Config) (MailClient, error) {
	switch account.Protocol {
	case "pop3", "pop3-no-ssl":
		return pop3pkg.NewPOP3Client(account, cfg)
	default:
		return NewIMAPClient(account, cfg)
	}
}

// Authenticate 根据账号 AuthType 自动选择认证方式：
//   - password（默认）：传统 LOGIN 命令
//   - oauth2_*：XOAUTH2 SASL 认证（自动刷新 Token）
func (c *IMAPClient) Authenticate() error {
	// OAuth2 认证路径
	if oauth2.IsOAuth2Account(c.Account.AuthType) {
		return c.authenticateOAuth2()
	}
	// 传统密码认证路径
	return c.authenticateLogin()
}

// authenticateOAuth2 使用 XOAUTH2 SASL 进行 OAuth2 认证
func (c *IMAPClient) authenticateOAuth2() error {
	providerName := oauth2.ParseAuthType(c.Account.AuthType)
	if providerName == "" {
		return fmt.Errorf("无效的 OAuth2 认证类型: %s", c.Account.AuthType)
	}

	provider, clientID, err := oauth2.ResolveProviderAndClientID(providerName, c.Account.CustomClientId)
	if err != nil {
		return fmt.Errorf("OAuth2 Provider 解析失败: %w", err)
	}

	accessToken, err := c.resolveAccessToken(provider, clientID)
	if err != nil {
		return err
	}
	if accessToken == "" {
		return fmt.Errorf("无法获取有效的 AccessToken（可能需要重新授权）")
	}

	// 发送 ID 命令（同 LOGIN 流程）
	c.sendClientID()

	// 使用 XOAUTH2 SASL 机制进行 OAuth2 认证（Microsoft/Gmail IMAP 均支持 XOAUTH2）
	saslClient := &XOAUTH2Client{
		Username: c.Account.Email,
		Token:    accessToken,
	}

	if err := c.Client.Authenticate(saslClient); err != nil {
		return fmt.Errorf("IMAP XOAUTH2 认证失败 (%s@%s): %w", c.Account.Username, c.Account.Email, err)
	}
	log.Printf("✅ IMAP XOAUTH2 认证成功: %s [Provider=%s]", c.Account.Email, providerName)
	return nil
}

// XOAUTH2Client 实现 sasl.Client 接口，使用 XOAUTH2 SASL 机制
// 微软 Outlook IMAP (outlook.office365.com) 不支持 OAUTHBEARER，仅支持 XOAUTH2
type XOAUTH2Client struct {
	Username string
	Token    string
}

// Start 返回 SASL 机制名称和初始响应
func (c *XOAUTH2Client) Start() (string, []byte, error) {
	// XOAUTH2 格式: "user={email}\x01auth=Bearer {token}\x01\x01"
	resp := []byte("user=" + c.Username + "\x01auth=Bearer " + c.Token + "\x01\x01")
	return "XOAUTH2", resp, nil
}

// Next 处理服务器挑战（XOAUTH2 成功时无挑战，失败时返回空响应中止）
func (c *XOAUTH2Client) Next(challenge []byte) ([]byte, error) {
	return []byte{}, nil
}

// resolveAccessToken 获取有效的 Access Token，过期则自动刷新。
// 返回的 error 区分不同失败原因（解密失败 / RefreshToken 失效需重新授权 / 网络错误），
// 方便运维通过日志一眼定位。
func (c *IMAPClient) resolveAccessToken(provider oauth2.OAuth2Provider, clientID string) (string, error) {
	// 检查是否需要刷新（提前 2 分钟刷新）
	if c.Account.TokenExpiresAt != nil {
		expiresAt := c.Account.TokenExpiresAt.Add(-2 * time.Minute)
		if time.Now().Before(expiresAt) && c.Account.Password != "" {
			// Password 字段复用存储内存中的 Access Token（运行时有效）
			// 注：Password 在 AfterFind 后是明文，但 OAuth2 模式下这里存的是 AT
			_ = clientID // 避免未使用警告，实际使用场景中 Password 即为 AT
			return c.Account.Password, nil
		}
	}

	// 区分两种"无 RefreshToken"的情况：
	//   1) AfterFind 解密失败（密钥不匹配/密文损坏）——属于数据/密钥问题，必须重新授权
	//   2) 账号从未授权——正常提示重新授权
	if c.Account.RefreshToken == "" {
		if c.Account.RefreshTokenDecryptFailed {
			log.Printf("❌ [OAuth2] 账号 %s 的 RefreshToken 解密失败（密钥不匹配或密文损坏），无法刷新 AccessToken，需用户重新授权",
				c.Account.Email)
			return "", fmt.Errorf("RefreshToken 解密失败（密钥不匹配或密文损坏），需用户重新授权")
		}
		log.Printf("[WARN] [OAuth2] 账号 %s 无 RefreshToken，需要用户重新授权", c.Account.Email)
		return "", fmt.Errorf("账号未授权（无 RefreshToken），需要用户重新授权")
	}

	log.Printf("🔄 正在刷新 AccessToken (%s@%s)...", c.Account.Email, provider.Name())
	tokenResp, err := provider.RefreshAccessToken(context.Background(), clientID, c.Account.RefreshToken)
	if err != nil {
		// 区分"RefreshToken 已被吊销"（需重新授权）与"网络/服务端临时错误"（可重试）
		if errors.Is(err, oauth2.ErrRefreshTokenRevoked) {
			log.Printf("❌ [OAuth2] AccessToken 刷新失败（%s）：RefreshToken 已被微软吊销/失效，需用户重新授权: %v",
				c.Account.Email, err)
		} else {
			log.Printf("❌ [OAuth2] AccessToken 刷新失败（%s）：网络或服务端错误，可稍后重试: %v",
				c.Account.Email, err)
		}
		return "", err
	}

	// 更新本地缓存（注意：此处仅更新运行时值，数据库更新需由上层调用方处理）
	c.Account.Password = tokenResp.AccessToken
	now := time.Now()
	expiresAt := now.Add(tokenResp.ExpiresIn)
	c.Account.TokenExpiresAt = &expiresAt

	// 如果微软返回了新的 RefreshToken，记录日志提示保存
	if tokenResp.RefreshToken != "" {
		log.Printf("[INFO] Provider %s 返回了新 RefreshToken for %s，建议保存到数据库", provider.Name(), c.Account.Email)
		c.Account.RefreshToken = tokenResp.RefreshToken
	}

	log.Printf("✅ AccessToken 刷新成功: %s (有效期 %v)", c.Account.Email, tokenResp.ExpiresIn)
	return tokenResp.AccessToken, nil
}

// persistOAuthTokens 将刷新得到的 OAuth2 Token 持久化到数据库。
//
// 背景：微软 Consumer 账号（Hotmail/Outlook）的 RefreshToken 会在刷新后轮换/被吊销。
// 原实现仅在内存中更新 token，未落库，导致进程重启后会拿旧的已失效 RefreshToken 去刷新，
// 从而报 “无法获取有效的 AccessToken（可能需要重新授权）”。此处将 RefreshToken、Access Token
// 及过期时间一并写回，保证滚动后的 token 在重启后依然可用。
//
// 仅 OAuth2 账号需要此处理；传统密码账号直接跳过。
func (c *IMAPClient) persistOAuthTokens(db *gorm.DB) error {
	if !oauth2.IsOAuth2Account(c.Account.AuthType) {
		return nil
	}

	updates := models.MailAccount{
		Password:       c.Account.Password, // Access Token（明文，写入时由 BeforeUpdate 钩子加密）
		RefreshToken:   c.Account.RefreshToken,
		TokenExpiresAt: c.Account.TokenExpiresAt,
	}
	// 使用结构体更新：GORM 仅写入非零字段（Password / RefreshToken / TokenExpiresAt），
	// 并触发 BeforeUpdate 钩子对敏感字段加密存储，不会覆盖其它列。
	if err := db.Model(&models.MailAccount{}).
		Where("id = ?", c.Account.ID).
		Updates(&updates).Error; err != nil {
		return fmt.Errorf("持久化 OAuth2 Token 失败: %w", err)
	}
	log.Printf("💾 已持久化刷新后的 OAuth2 Token (%s)", c.Account.Email)
	return nil
}

// authenticateLogin 使用传统 LOGIN 命令进行密码认证
func (c *IMAPClient) authenticateLogin() error {
	if c.Account.Password == "" {
		return fmt.Errorf("密码为空，无法认证")
	}

	c.sendClientID()

	if err := c.Client.Login(c.Account.Username, c.Account.Password).Wait(); err != nil {
		return fmt.Errorf("IMAP 登录失败 (%s@%s): %w", c.Account.Username, c.Account.Email, err)
	}
	log.Printf("✅ IMAP 认证成功: %s", c.Account.Email)
	return nil
}

// sendClientID 发送 IMAP ID 命令声明客户端身份（RFC 2971）
// 163/126等网易邮箱要求客户端必须发送ID命令，否则会返回 "SELECT Unsafe Login" 错误
func (c *IMAPClient) sendClientID() {
	idData := &imap.IDData{
		Name:    "MagicMail",
		Version: "1.0.0",
		Vendor:  "MagicCode",
	}
	if _, err := c.Client.ID(idData).Wait(); err != nil {
		// ID 命令失败不阻塞登录（部分服务器可能不支持），仅记录日志
		log.Printf("⚠️  发送 IMAP ID 命令失败 (可能服务器不支持): %v", err)
	}
}

// SelectINBOX 选择收件箱并返回状态信息
func (c *IMAPClient) SelectINBOX() (*imap.SelectData, error) {
	return c.SelectMailbox("INBOX")
}

// SelectMailbox 选择指定邮箱（如 INBOX, Sent 等）并返回状态信息
func (c *IMAPClient) SelectMailbox(name string) (*imap.SelectData, error) {
	mbox, err := c.Client.Select(name, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("选择 %s 失败: %w", name, err)
	}
	return mbox, nil
}

// Close 关闭连接
func (c *IMAPClient) Close() {
	if c.Client != nil {
		if err := c.Client.Logout().Wait(); err != nil {
			log.Printf("⚠️  IMAP 连接关闭异常 (%s): %v", c.Account.Email, err)
		}
	}
}

// DeleteMessage 通过 UID 删除服务器上的邮件（Store + \Deleted 标志 → Expunge/UID Expunge）
func (c *IMAPClient) DeleteMessage(uid uint32) error {
	// 先选择 INBOX（使用读写模式）
	selectData, err := c.SelectINBOX()
	if err != nil {
		return fmt.Errorf("选择 INBOX 失败: %w", err)
	}

	// 检查服务器是否允许设置删除标志（\Deleted 是否在 PermanentFlags 中）
	canDelete := false
	for _, f := range selectData.PermanentFlags {
		if f == imap.FlagDeleted || f == imap.FlagWildcard {
			canDelete = true
			break
		}
	}
	if !canDelete {
		return fmt.Errorf("INBOX 不支持删除操作（PermanentFlags 中无 \\Deleted）")
	}

	// 设置 \Deleted 标志
	uidSet := imap.UIDSetNum(imap.UID(uid))
	storeCmd := c.Client.Store(uidSet, &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}, nil)
	if err := storeCmd.Close(); err != nil {
		return fmt.Errorf("标记删除标志失败 (UID=%d): %w", uid, err)
	}

	// 优先尝试 UID EXPUNGE (RFC 4315)，更精确地只删除指定 UID 的邮件
	// 如果服务器不支持，则回退到普通 EXPUNGE
	uidExpungeErr := c.tryUIDExpunge(uid)
	if uidExpungeErr == nil {
		log.Printf("🗑️  已通过 UID EXPUNGE 从源服务器删除邮件 (UID=%d, %s)", uid, c.Account.Email)
		return nil
	}

	log.Printf("[INFO] UID EXPUNGE 不可用，尝试普通 EXPUNGE (UID=%d): %v", uid, uidExpungeErr)

	// 回退：执行普通 Expunge 永久删除所有已标记邮件
	expungeCmd := c.Client.Expunge()
	if err := expungeCmd.Close(); err != nil {
		return fmt.Errorf("Expunge 失败 (UID=%d): %w", uid, err)
	}

	log.Printf("🗑️  已通过 EXPUNGE 从源服务器删除邮件 (UID=%d, %s)", uid, c.Account.Email)
	return nil
}

// tryUIDExpunge 尝试使用 UID EXPUNGE 命令（RFC 4315）
// 只删除指定 UID 的邮件，不影响其他已标记删除的邮件
func (c *IMAPClient) tryUIDExpunge(uid uint32) error {
	uidSet := imap.UIDSetNum(imap.UID(uid))
	uidExpungeCmd := c.Client.UIDExpunge(uidSet)
	if err := uidExpungeCmd.Close(); err != nil {
		return err
	}
	return nil
}

// TestConnection 测试邮箱账号的连接是否可用（根据协议自动选择）
func TestConnection(account *models.MailAccount, cfg *config.Config) error {
	client, err := NewMailClient(account, cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	// POP3 认证后额外验证邮件列表
	if pc, ok := client.(*pop3pkg.POP3Client); ok {
		if err := pc.Authenticate(); err != nil {
			return err
		}
		_, err = pc.MessageCount()
		return err
	}

	// IMAP：认证 + 选择 INBOX
	if ic, ok := client.(*IMAPClient); ok {
		if err := ic.Authenticate(); err != nil {
			return err
		}
		_, err = ic.SelectINBOX()
		return err
	}

	return fmt.Errorf("未知的协议类型: %s", account.Protocol)
}
