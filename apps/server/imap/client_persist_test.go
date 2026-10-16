package imap

import (
	"testing"

	mcrypto "magicmail/crypto"
	"magicmail/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestPersistOAuthTokensEncrypts 回归测试：
// 验证 persistOAuthTokens 把 refresh_token / access_token 以密文落库，
// 不能依赖 GORM BeforeUpdate 钩子（db.Model(&MailAccount{}).Updates 的钩子作用于空模型而非数据），
// 否则会以明文入库，导致后续 AfterFind 解密报 “RefreshToken 解密失败（密钥不匹配或密文损坏）”。
func TestPersistOAuthTokensEncrypts(t *testing.T) {
	if err := mcrypto.Init("test-key-1234567890"); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.MailAccount{}); err != nil {
		t.Fatal(err)
	}

	acc := models.MailAccount{
		Name: "t", Email: "a@b.com", Username: "a@b.com",
		ImapHost: "x", Port: 993, UserID: 1,
		AuthType: "oauth2_microsoft", OAuthProvider: "microsoft",
		RefreshToken: "PLAINTEXT_RT", Password: "PLAINTEXT_AT",
	}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatal(err)
	}

	// 模拟刷新后内存中的新 token（明文）
	c := &IMAPClient{
		Account: &models.MailAccount{
			ID:           acc.ID,
			AuthType:     "oauth2_microsoft",
			Password:     "NEW_AT",
			RefreshToken: "NEW_RT",
		},
	}
	if err := c.persistOAuthTokens(db); err != nil {
		t.Fatal(err)
	}

	var raw struct {
		Password      string
		RefreshToken  string
	}
	if err := db.Raw("SELECT password, refresh_token FROM mail_accounts WHERE id = ?", acc.ID).Scan(&raw).Error; err != nil {
		t.Fatal(err)
	}
	if !mcrypto.IsEncrypted(raw.RefreshToken) {
		t.Fatalf("refresh_token 明文入库: %q", raw.RefreshToken)
	}
	if !mcrypto.IsEncrypted(raw.Password) {
		t.Fatalf("password 明文入库: %q", raw.Password)
	}
	decRT, err := mcrypto.Decrypt(raw.RefreshToken)
	if err != nil || decRT != "NEW_RT" {
		t.Fatalf("refresh_token 解密不符: dec=%q err=%v", decRT, err)
	}
	decAT, err := mcrypto.Decrypt(raw.Password)
	if err != nil || decAT != "NEW_AT" {
		t.Fatalf("password 解密不符: dec=%q err=%v", decAT, err)
	}
}
