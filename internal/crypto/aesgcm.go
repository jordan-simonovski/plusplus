package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// AESEncryptor encrypts at rest with AES-256-GCM (random nonce per encrypt).
type AESEncryptor struct {
	gcm cipher.AEAD
}

// NewAESEncryptor expects a 32-byte AES-256 key.
func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESEncryptor{gcm: gcm}, nil
}

// ParseKeyBase64 decodes a standard base64-encoded 32-byte key (e.g. openssl rand -base64 32).
func ParseKeyBase64(s string) ([]byte, error) {
	if s == "" {
		return nil, fmt.Errorf("empty encryption key")
	}
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("decoded key must be 32 bytes, got %d", len(key))
	}
	return key, nil
}

// Encrypt returns nonce || ciphertext.
func (e *AESEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return e.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt reverses Encrypt.
func (e *AESEncryptor) Decrypt(blob []byte) ([]byte, error) {
	ns := e.gcm.NonceSize()
	if len(blob) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return e.gcm.Open(nil, blob[:ns], blob[ns:], nil)
}
