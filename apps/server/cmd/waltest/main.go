package main

import (
	"fmt"

	"magicmail/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("data/magicmail.db?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		panic(err)
	}
	return db
}

// 模拟 EnsureVAPIDKeys 的读取判定逻辑
func loadVAPID(db *gorm.DB) string {
	var cfg models.AppConfig
	if err := db.First(&cfg).Error; err != nil {
		return "ERR:" + err.Error()
	}
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		return "EMPTY -> would regenerate"
	}
	return "LOADED len(pub)=" + fmt.Sprint(len(cfg.VAPIDPublicKey))
}

func main() {
	// 启动一：应读到空 -> 生成并写入
	db := openDB()
	fmt.Println("[run1] read:", loadVAPID(db))
	db.Model(&models.AppConfig{}).Where("id = 1").Updates(map[string]interface{}{
		"vapid_public_key":  "REALKEY_PUB_1234567890",
		"vapid_private_key": "REALKEY_PRIV_1234567890",
	})
	sqlDB, _ := db.DB()
	sqlDB.Close()

	// 启动二：应读到已写入的密钥，不再重新生成
	db2 := openDB()
	fmt.Println("[run2] read:", loadVAPID(db2))
	sqlDB2, _ := db2.DB()
	sqlDB2.Close()
}
