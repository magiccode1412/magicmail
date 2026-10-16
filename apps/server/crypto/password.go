// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026  magiccode (魔法代码)

// Package crypto 提供 AES-256-GCM 对称加解密功能，用于邮箱密码的安全存储。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"
)

var (
	// encryptionKey 当前生效的加密密钥，由 Init() 设置。
	encryptionKey []byte
	// ErrNotInitialized 密钥未初始化错误。
	ErrNotInitialized = errors.New("加密模块未初始化，请先调用 crypto.Init()")
	// ErrInvalidCiphertext 密文格式无效或解密失败。
	ErrInvalidCiphertext = errors.New("密文格式无效或解密失败")
)

const (
	// nonceLength GCM Nonce 长度（标准 12 字节）。
	nonceLength = 12
	// prefix 加密密文的标识前缀，用于区分明文和已加密内容。
	prefix = "ENC:"
)

// Init 初始化加密模块，设置 AES-256 密钥（hex 编码或原始字符串均可）。
func Init(key string) error {
	if key == "" {
		return errors.New("加密密钥不能为空")
	}
	k := []byte(key)
	// AES-256 需要 32 字节密钥；如果长度不是 32，取 SHA256 哈希以确保安全长度
	if len(k) != 32 {
		hash := sha256.Sum256(k)
		log.Printf("⚠️  加密密钥长度 %d ≠ 32 字节，已使用 SHA256 标准化", len(key))
		k = hash[:]
	}
	encryptionKey = k
	log.Println("🔐 加密模块已初始化")
	return nil
}

// GetKey 返回当前加密密钥的副本。
func GetKey() []byte {
	return append([]byte{}, encryptionKey...)
}

// IsEncrypted 判断字符串是否为已加密的密文（以 ENC: 前缀标识）。
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, prefix)
}

// Encrypt 使用 AES-256-GCM 加密明文字符串，返回 Base64 编码的密文（带 ENC: 前缀）。
// 如果输入已经是加密密文，直接原样返回（不重复加密）。
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil // 已经是密文，不重复加密
	}
	if len(encryptionKey) == 0 {
		return "", ErrNotInitialized
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("创建 AES Cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	nonce := make([]byte, nonceLength)
	if _, err = rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成 Nonce 失败: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	// 输出格式：ENC:Base64(nonce + ciphertext)
	result := prefix + base64.StdEncoding.EncodeToString(append(nonce, ciphertext...))
	return result, nil
}

// Decrypt 解密 ENC: 前缀的密文字符串，返回原始明文。
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	if !IsEncrypted(ciphertext) {
		return "", ErrInvalidCiphertext
	}
	if len(encryptionKey) == 0 {
		return "", ErrNotInitialized
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	if len(data) < nonceLength {
		return "", fmt.Errorf("密文数据过短（%d < %d）", len(data), nonceLength)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("创建 AES Cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	nonce := data[:nonceLength]
	ct := data[nonceLength:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	return string(plaintext), nil
}

// MustEncrypt 加密失败时 panic（仅在确定有密钥的场景使用）。
func MustEncrypt(plaintext string) string {
	s, err := Encrypt(plaintext)
	if err != nil {
		panic(fmt.Sprintf("crypto.Encrypt failed: %v", err))
	}
	return s
}

// MustDecrypt 解密失败时 panic（仅在确定有密钥的场景使用）。
func MustDecrypt(ciphertext string) string {
	s, err := Decrypt(ciphertext)
	if err != nil {
		panic(fmt.Sprintf("crypto.Decrypt failed: %v", err))
	}
	return s
}
