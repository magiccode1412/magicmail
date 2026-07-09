// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"magicmail/crypto"
	"magicmail/models"

	// 纯 Go SQLite 驱动（基于 modernc.org/sqlite，无需 CGO）
	"github.com/glebarez/sqlite"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Init 初始化 SQLite 数据库连接并执行自动迁移
func Init(dsn string) *gorm.DB {
	// 确保 data 目录存在
	dbDir := filepath.Dir(dsn)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatalf("❌ 无法创建数据目录 %s: %v", dbDir, err)
		}
	}

	// 按环境控制 SQL 日志：生产环境静默，开发环境输出 SQL
	logLevel := gormlogger.Silent
	if os.Getenv("MAGICMAIL_ENV") != "production" {
		logLevel = gormlogger.Info
	}

	// 连接 SQLite
	db, err := gorm.Open(sqlite.Open(dsn+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}

	// 获取底层数据库连接并配置
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ 获取数据库实例失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)

	// 自动迁移表结构
	if err := db.AutoMigrate(
		&models.MailAccount{},
		&models.Mail{},
		&models.Attachment{},
		&models.Webhook{},
		&models.WebhookLog{},
		&models.User{},
		&models.AppConfig{},
		&models.Draft{},
		&models.PushSubscription{},
	); err != nil {
		log.Fatalf("❌ 数据库迁移失败: %v", err)
	}

	// 迁移后清理：修复历史重复入库问题
	// 1. 删除 account_id+folder+message_uid 维度的重复记录（保留最早入库的一条）
	// 2. 将旧的带时间戳的 fallback message_id 更新为稳定格式
	// 3. 创建复合唯一索引 account_id+folder+message_uid
	// 4. 移除过宽的旧唯一索引（message_id 全局唯一）
	cleanupDuplicateMails(db)

	// 多用户改造兜底迁移：确保涉及用户隔离的表都存在 user_id 列。
	// 部分老版本数据库在建表时尚未引入 user_id，而 SQLite + GORM 的 AutoMigrate
	// 在某些场景下未能补齐 NOT NULL 列，导致运行期出现 "no such column: user_id" 错误。
	// 此处显式 ALTER TABLE 补齐，使老库在重启后自愈。
	ensureUserIDColumns(db)

	// 多用户改造迁移：将旧版单用户库（user_id=0）的历史数据归属到唯一管理员
	migrateUserOwnership(db)

	// 多用户改造迁移：移除 email 的全局唯一约束，改为 (user_id, email) 复合唯一，
	// 允许不同用户各自绑定同一邮箱（同一用户仍不可重复绑定）。复合唯一索引 idx_user_email
	// 由 GORM 在上面的 AutoMigrate 阶段创建，此处仅删除旧版的全局唯一索引。
	ensureMailAccountUserEmailIndex(db)

	// 兜底迁移：确保 app_configs 表存在 VAPID 相关列。
	// 老版本数据库在建表时尚未引入 VAPID 字段，而 SQLite + GORM 的 AutoMigrate
	// 在某些场景下未能补齐这些列，导致运行期出现 "no such column: vapid_private_key" 错误。
	// 此处显式 ALTER TABLE 补齐，使老库在重启后自愈，避免在 EnsureVAPIDKeys 中每次都重新生成密钥。
	ensureAppConfigColumns(db)

	// 凭据自愈迁移：修复因 map 更新绕过加密钩子而落入库的明文密码/Token。
	// 这些残留明文会被重新加密，避免后续读取解密失败、密码被置空。
	encryptPlaintextSecrets(db)

	fmt.Println("✅ 数据库初始化成功:", dsn)
	return db
}

// migrateUserOwnership 多用户改造的一次性迁移：
//   - 将历史唯一用户（最小 ID）角色置为管理员；
//   - 将 mail_accounts / mails / webhooks / drafts / push_subscriptions 中
//     user_id=0 的记录归属到该管理员，避免孤立数据。
func migrateUserOwnership(db *gorm.DB) {
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)
	if userCount == 0 {
		return // 全新实例，无需迁移
	}

	// 1. 确定管理员：取注册最早（ID 最小）的用户并置为 admin
	var adminID int64
	if err := db.Raw("SELECT MIN(id) FROM users").Scan(&adminID).Error; err != nil {
		log.Printf("⚠️  [迁移] 查询管理员失败: %v", err)
		return
	}
	if adminID == 0 {
		return
	}
	db.Exec("UPDATE users SET role = ? WHERE id = ? AND (role IS NULL OR role = '')", models.RoleAdmin, adminID)

	// 2. 邮箱账号归属管理员（user_id=0 视为历史数据）
	db.Exec("UPDATE mail_accounts SET user_id = ? WHERE user_id = 0", adminID)

	// 3. 邮件归属：通过所属邮箱账号回填 user_id
	db.Exec(`
		UPDATE mails
		SET user_id = (SELECT ma.user_id FROM mail_accounts ma WHERE ma.id = mails.account_id)
		WHERE mails.user_id = 0
	`)

	// 4. Webhook / 草稿 / 推送订阅归属管理员
	db.Exec("UPDATE webhooks SET user_id = ? WHERE user_id = 0", adminID)
	db.Exec("UPDATE drafts SET user_id = ? WHERE user_id = 0", adminID)
	db.Exec("UPDATE push_subscriptions SET user_id = ? WHERE user_id = 0", adminID)

	log.Printf("✅ [迁移] 历史数据已归属管理员 (user_id=%d)", adminID)
}

// ensureUserIDColumns 兜底迁移：确保多用户隔离涉及的表都存在 user_id 列。
// 老版本库在 AutoMigrate 阶段可能未补齐该列，这里显式 ALTER TABLE ADD COLUMN 修复，
// 避免运行期出现 "no such column: user_id"。webhook_logs 仅通过 webhook_id 关联，不需要 user_id。
func ensureUserIDColumns(db *gorm.DB) {
	type colDef struct {
		table   string
		notNull bool // 是否以 NOT NULL DEFAULT 0 追加（兼容存量行）
	}
	tables := []colDef{
		{"mails", true},
		{"mail_accounts", true},
		{"webhooks", true},
		{"drafts", false},
		{"push_subscriptions", true},
	}
	for _, t := range tables {
		var cnt int64
		db.Raw(fmt.Sprintf("SELECT count(*) FROM pragma_table_info('%s') WHERE name = 'user_id'", t.table)).Scan(&cnt)
		if cnt > 0 {
			continue // 列已存在，跳过
		}
		typ := "INTEGER"
		if t.notNull {
			typ += " NOT NULL DEFAULT 0"
		}
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN user_id %s", t.table, typ)).Error; err != nil {
			log.Printf("⚠️  [迁移] 为 %s 表补充 user_id 列失败: %v", t.table, err)
		} else {
			log.Printf("✅ [迁移] 已为 %s 表补充 user_id 列", t.table)
		}
	}
}

// ensureMailAccountUserEmailIndex 兜底迁移：删除旧版 mail_accounts.email 的全局唯一索引，
// 改为 (user_id, email) 复合唯一（由 GORM 在 AutoMigrate 阶段创建 idx_user_email）。
// 多用户改造前 email 是全局唯一，导致不同用户无法绑定同一邮箱；移除后按用户隔离。
func ensureMailAccountUserEmailIndex(db *gorm.DB) {
	// 旧索引名为 GORM 旧模型生成的 idx_mail_accounts_email；存在则删除（幂等）。
	if err := db.Exec("DROP INDEX IF EXISTS idx_mail_accounts_email").Error; err != nil {
		log.Printf("⚠️  [迁移] 删除旧 email 全局唯一索引失败: %v", err)
		return
	}
	// 确认新的复合唯一索引已存在（GORM AutoMigrate 应已创建）
	var cnt int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_user_email'").Scan(&cnt)
	if cnt == 0 {
		// 兜底：自动创建复合唯一索引，防止 AutoMigrate 因故未创建
		if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_email ON mail_accounts(user_id, email)").Error; err != nil {
			log.Printf("⚠️  [迁移] 创建 (user_id, email) 复合唯一索引失败: %v", err)
			return
		}
	}
	log.Printf("✅ [迁移] 已移除 email 全局唯一约束（改为按用户隔离）")
}

// ensureAppConfigColumns 兜底迁移：确保 app_configs 表存在 VAPID 相关列。
// 老版本数据库在建表时尚未引入 VAPID 字段，而 SQLite + GORM 的 AutoMigrate
// 在某些场景下未能补齐这些列，导致运行期出现 "no such column: vapid_private_key" 错误。
// 此处显式 ALTER TABLE 补齐，使老库在重启后自愈，避免在 EnsureVAPIDKeys 中每次都重新生成密钥。
func ensureAppConfigColumns(db *gorm.DB) {
	columns := []string{"vapid_public_key", "vapid_private_key"}
	for _, col := range columns {
		var cnt int64
		db.Raw(fmt.Sprintf("SELECT count(*) FROM pragma_table_info('app_configs') WHERE name = '%s'", col)).Scan(&cnt)
		if cnt > 0 {
			continue // 列已存在，跳过
		}
		if err := db.Exec(fmt.Sprintf("ALTER TABLE app_configs ADD COLUMN %s text DEFAULT ''", col)).Error; err != nil {
			log.Printf("⚠️  [迁移] 为 app_configs 表补充 %s 列失败: %v", col, err)
		} else {
			log.Printf("✅ [迁移] 已为 app_configs 表补充 %s 列", col)
		}
	}
}

// encryptPlaintextSecrets 兜底迁移：将 mail_accounts 中残留的明文敏感字段
// （password / refresh_token / custom_client_id）重新加密为密文。
//
// 背景：历史版本中邮箱账号的更新走的是 map 更新（s.db.Model(&account).Updates(map)），
// GORM 的 BeforeUpdate 钩子对结构体字段的加密不会作用到 map，导致密码以明文入库；
// 后续 AfterFind 解密时因非 ENC: 密文而失败、将密码置空，表现为"密码为空，无法认证"。
// 此处扫描未加密的字段并就地加密，使旧数据自愈。
func encryptPlaintextSecrets(db *gorm.DB) {
	type rawSecret struct {
		ID            uint
		Password      string
		RefreshToken  string
		CustomClientId string
	}
	var rows []rawSecret
	// 用裸查询绕过 MailAccount 的 AfterFind 钩子，直接读取原始（可能明文）值
	if err := db.Raw("SELECT id, password, refresh_token, custom_client_id FROM mail_accounts").Scan(&rows).Error; err != nil {
		log.Printf("⚠️  [迁移] 读取邮箱凭据失败: %v", err)
		return
	}

	reencrypted := 0
	for _, r := range rows {
		needUpdate := false
		newPass := r.Password
		if r.Password != "" && !crypto.IsEncrypted(r.Password) {
			if enc, err := crypto.Encrypt(r.Password); err == nil {
				newPass = enc
				needUpdate = true
			}
		}
		newRT := r.RefreshToken
		if r.RefreshToken != "" && !crypto.IsEncrypted(r.RefreshToken) {
			if enc, err := crypto.Encrypt(r.RefreshToken); err == nil {
				newRT = enc
				needUpdate = true
			}
		}
		newCID := r.CustomClientId
		if r.CustomClientId != "" && !crypto.IsEncrypted(r.CustomClientId) {
			if enc, err := crypto.Encrypt(r.CustomClientId); err == nil {
				newCID = enc
				needUpdate = true
			}
		}
		if !needUpdate {
			continue
		}
		if err := db.Exec(
			"UPDATE mail_accounts SET password = ?, refresh_token = ?, custom_client_id = ? WHERE id = ?",
			newPass, newRT, newCID, r.ID,
		).Error; err != nil {
			log.Printf("⚠️  [迁移] 重新加密账号 #%d 凭据失败: %v", r.ID, err)
			continue
		}
		reencrypted++
	}
	if reencrypted > 0 {
		log.Printf("✅ [迁移] 已重新加密 %d 个邮箱账号的明文凭据", reencrypted)
	}
}

// EnsureSecuritySecrets 确保安全密钥存在：
//   - 首次启动时优先使用环境变量传入的密钥（MAGICMAIL_JWT_SECRET / MAGICMAIL_ENCRYPT_KEY），
//     未设置则自动生成随机密钥，最终持久化到数据库；
//   - 后续启动从数据库读取，但若环境变量有新值则以环境变量为准（覆盖数据库值）。
func EnsureSecuritySecrets(db *gorm.DB, jwtSecret, encryptionKey *string) {
	// 优先从环境变量读取用户指定的密钥
	envJWT := os.Getenv("MAGICMAIL_JWT_SECRET")
	envEncKey := os.Getenv("MAGICMAIL_ENCRYPT_KEY")

	var cfg models.AppConfig
	result := db.First(&cfg)

	if result.Error != nil {
		// 首次启动：环境变量 > 自动生成
		jwtSec := envJWT
		encKey := envEncKey
		var err error

		if jwtSec == "" {
			jwtSec, err = models.GenerateRandomKey(32)
			if err != nil {
				log.Fatalf("❌ 生成 JWT 密钥失败: %v", err)
			}
			log.Println("🔑 JWT 密钥：已自动生成随机密钥")
		} else {
			log.Println("🔑 JWT 密钥：从环境变量 MAGICMAIL_JWT_SECRET 读取")
		}

		if encKey == "" {
			encKey, err = models.GenerateRandomKey(32)
			if err != nil {
				log.Fatalf("❌ 生成加密密钥失败: %v", err)
			}
			log.Println("🔐 加密密钥：已自动生成随机密钥")
		} else {
			log.Println("🔐 加密密钥：从环境变量 MAGICMAIL_ENCRYPT_KEY 读取")
		}

		cfg = models.AppConfig{
			JWTSecret:        jwtSec,
			EncryptionKey:    encKey,
			OpenRegistration: false, // 默认关闭公开注册
		}
		if err := db.Create(&cfg).Error; err != nil {
			log.Fatalf("❌ 保存安全配置失败: %v", err)
		}

		*jwtSecret = jwtSec
		*encryptionKey = encKey
	} else {
		// 已有记录：环境变量 > 数据库存储
		useJWT := cfg.JWTSecret
		useEncKey := cfg.EncryptionKey
		sourceJWT := "数据库"
		sourceEncKey := "数据库"

		if envJWT != "" && envJWT != cfg.JWTSecret {
			useJWT = envJWT
			sourceJWT = "环境变量"
			// 同步更新数据库，保证下次启动一致
			cfg.JWTSecret = envJWT
			db.Save(&cfg)
		}

		if envEncKey != "" && envEncKey != cfg.EncryptionKey {
			useEncKey = envEncKey
			sourceEncKey = "环境变量"
			cfg.EncryptionKey = envEncKey
			db.Save(&cfg)
		}

		*jwtSecret = useJWT
		*encryptionKey = useEncKey
		log.Printf("🔐 安全密钥已加载（JWT 来源：%s，加密密钥 来源：%s）", sourceJWT, sourceEncKey)
	}
}

// cleanupDuplicateMails 迁移后清理函数：修复历史重复入库问题
//
// 问题背景：旧版 fallback Message-ID 使用了 time.Now()，导致同一封邮件
// 每次同步生成不同 message_id，去重失效，同一封邮件被反复入库。
//
// 本函数执行以下步骤：
//  1. 删除 account_id + folder + message_uid 维度的重复记录（保留最早入库的一条）
//  2. 将旧的带时间戳的 fallback message_id 更新为新的稳定格式
//  3. 创建复合唯一索引 idx_mails_account_folder_uid
//  4. 移除过宽的旧唯一索引（message_id 全局唯一），避免不同账号收到相同 Message-ID 时冲突
func cleanupDuplicateMails(db *gorm.DB) {
	// 检查 mails 表是否存在
	var tableExists int64
	db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='mails'").Scan(&tableExists)
	if tableExists == 0 {
		return // 新数据库，无需清理
	}

	// 步骤1：删除 account_id + folder + message_uid 维度的重复记录（保留最小 id）
	result := db.Exec(`
		DELETE FROM mails
		WHERE id NOT IN (
			SELECT MIN(id) FROM mails
			WHERE message_uid > 0
			GROUP BY account_id, folder, message_uid
		)
		AND message_uid > 0
	`)
	if result.Error != nil {
		log.Printf("⚠️  [迁移] 清理重复邮件记录失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🧹 [迁移] 已清理 %d 条重复邮件记录", result.RowsAffected)
	}

	// 步骤2：将旧的 fallback message_id 更新为新的稳定格式
	// 旧格式: <auto-{uid}-{timestamp}@proxy>  →  新格式: <auto-account-{aid}-folder-{folder}-uid-{uid}@magicmail>
	result = db.Exec(`
		UPDATE mails
		SET message_id = '<auto-account-' || account_id || '-folder-' || COALESCE(folder, 'inbox') || '-uid-' || message_uid || '@magicmail>'
		WHERE message_id LIKE '<auto-%@proxy>'
	`)
	if result.Error != nil {
		log.Printf("⚠️  [迁移] 更新旧 IMAP fallback message_id 失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🔄 [迁移] 已更新 %d 条旧格式 IMAP fallback message_id", result.RowsAffected)
	}

	// 旧格式: <pop3-{seq}-{timestamp}@proxy>  →  新格式: <pop3-account-{aid}-seq-{seq}@magicmail>
	result = db.Exec(`
		UPDATE mails
		SET message_id = '<pop3-account-' || account_id || '-seq-' || message_uid || '@magicmail>'
		WHERE message_id LIKE '<pop3-%@proxy>'
	`)
	if result.Error != nil {
		log.Printf("⚠️  [迁移] 更新旧 POP3 fallback message_id 失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("🔄 [迁移] 已更新 %d 条旧格式 POP3 fallback message_id", result.RowsAffected)
	}

	// 步骤3：移除过宽的旧唯一索引（message_id 全局唯一）
	// GORM 默认命名为 uniq_mails_message_id
	db.Exec("DROP INDEX IF EXISTS uniq_mails_message_id")

	// 步骤4：创建复合唯一索引（如果不存在）
	// 确保 account_id + folder + message_uid 三元组唯一，防止重复入库
	result = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_mails_account_folder_uid
		ON mails(account_id, folder, message_uid)
	`)
	if result.Error != nil {
		log.Printf("⚠️  [迁移] 创建复合唯一索引失败（可能存在残留重复数据，不影响运行）: %v", result.Error)
		log.Printf("⚠️  [迁移] 建议手动执行: DELETE FROM mails WHERE id NOT IN (SELECT MIN(id) FROM mails WHERE message_uid > 0 GROUP BY account_id, folder, message_uid) AND message_uid > 0; 然后重启")
	} else {
		log.Printf("✅ [迁移] 复合唯一索引 idx_mails_account_folder_uid 已就绪")
	}
}
