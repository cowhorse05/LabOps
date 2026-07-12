package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const encryptedSecretPrefix = "enc:v1:"

func (s *Store) ConfigureEncryptionKey(raw string) error {
	if strings.TrimSpace(raw) == "" {
		s.encryptionKey = nil
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(key) != 32 {
		return fmt.Errorf("LABOPS_ENCRYPTION_KEY must be standard base64 encoding of exactly 32 random bytes")
	}
	s.encryptionKey = key
	return nil
}

func (s *Store) encryptSecret(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	if len(s.encryptionKey) == 0 {
		return value, nil
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, ciphertext...)
	return encryptedSecretPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (s *Store) decryptSecret(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	if len(s.encryptionKey) == 0 {
		return "", fmt.Errorf("encrypted value exists but LABOPS_ENCRYPTION_KEY is not configured")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, encryptedSecretPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted payload is truncated")
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *Store) ProtectStoredLLMSecret(ctx context.Context) error {
	if len(s.encryptionKey) == 0 {
		return nil
	}
	var value string
	if err := s.db.QueryRowContext(ctx, "SELECT api_key FROM llm_config WHERE id = 1").Scan(&value); err != nil {
		return err
	}
	if value == "" || strings.HasPrefix(value, encryptedSecretPrefix) {
		return nil
	}
	protected, err := s.encryptSecret(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE llm_config SET api_key = ?, updated_at = ? WHERE id = 1", protected, nowString())
	return err
}
