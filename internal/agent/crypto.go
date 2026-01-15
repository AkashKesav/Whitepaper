// Package agent provides cryptographic utilities for securing sensitive data
package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"go.uber.org/zap"
)

// Crypto handles encryption/decryption of sensitive data using AES-256-GCM
type Crypto struct {
	encryptionKey []byte
	logger        *zap.Logger
}

// NewCrypto creates a new crypto instance using JWT_SECRET as base
// The JWT_SECRET is hashed with SHA-256 to derive a 32-byte key for AES-256
func NewCrypto(jwtSecret string, logger *zap.Logger) (*Crypto, error) {
	if len(jwtSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 16 characters for encryption")
	}

	// Derive 32-byte key from JWT_SECRET using SHA-256
	hash := sha256.Sum256([]byte(jwtSecret))

	return &Crypto{
		encryptionKey: hash[:],
		logger:        logger.Named("crypto"),
	}, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext
// Uses AES-256-GCM for authenticated encryption
func (c *Crypto) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// Create cipher block
	block, err := aes.NewCipher(c.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce (12 bytes for GCM)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode as base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext and returns plaintext
func (c *Crypto) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(c.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, cipherData := data[:nonceSize], data[nonceSize:]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// MaskAPIKey returns a masked version of an API key for logging
// Example: nvapi-3gOq...xYz123
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	if len(key) < 16 {
		return key[:4] + "..." + key[len(key)-4:]
	}
	return key[:8] + "..." + key[len(key)-4:]
}
