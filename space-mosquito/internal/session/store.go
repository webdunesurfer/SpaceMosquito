package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
)

type Store struct {
	filePath string
}

func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
	}
}

func (s *Store) HasSession() bool {
	_, err := os.Stat(s.filePath)
	return err == nil
}

func (s *Store) Save(rawSession *Session, key string) error {
	if key == "" {
		return fmt.Errorf("encryption key is required")
	}

	keyBytes := []byte(key)
	if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	} else if len(keyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	}

	sessionJSON, err := json.Marshal(rawSession)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, sessionJSON, nil)

	dir := s.filePath
	if idx := lastSlash(s.filePath); idx != -1 {
		dir = s.filePath[:idx]
	}

	if dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(s.filePath, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write session file %s: %w", s.filePath, err)
	}

	return nil
}

func (s *Store) Load(key string) (*Session, error) {
	if key == "" {
		return nil, fmt.Errorf("encryption key is required")
	}

	ciphertext, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file %s: %w", s.filePath, err)
	}

	keyBytes := []byte(key)
	if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	} else if len(keyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	sessionJSON, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt session (wrong key or corrupt file): %w", err)
	}

	var sess Session
	if err := json.Unmarshal(sessionJSON, &sess); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &sess, nil
}

func (s *Store) Delete() error {
	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file %s: %w", s.filePath, err)
	}
	return nil
}

func lastSlash(path string) int {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return i
		}
	}
	return -1
}
