package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"github.com/vkh/spacemosquito/pkg/logging"
)

type Store struct {
	filePath string
	log      logging.Sugar
}

func NewStore(filePath string, log logging.Sugar) *Store {
	return &Store{
		filePath: filePath,
		log:      log,
	}
}

func (s *Store) GetLogger() logging.Sugar {
	return s.log
}

func (s *Store) SetLogger(log logging.Sugar) {
	s.log = log
}

func (s *Store) HasSession() bool {
	_, err := os.Stat(s.filePath)
	if err != nil && s.log.Enabled() {
		s.log.Debugw("checking session file", "path", s.filePath, "exists", false)
	} else if s.log.Enabled() {
		s.log.Debugw("session file exists", "path", s.filePath)
	}
	return err == nil
}

func (s *Store) Save(rawSession *Session, key string) error {
	if key == "" {
		if s.log.Enabled() {
			s.log.Error("session save rejected: encryption key is required")
		}
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
		if s.log.Enabled() {
			s.log.Errorw("session save failed: marshal error", "error", err)
		}
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session save failed: cipher creation", "error", err)
		}
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session save failed: GCM creation", "error", err)
		}
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session save failed: nonce generation", "error", err)
		}
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, sessionJSON, nil)

	dir := s.filePath
	if idx := lastSlash(s.filePath); idx != -1 {
		dir = s.filePath[:idx]
	}

	if dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			if s.log.Enabled() {
				s.log.Errorw("session save failed: directory creation",
					"dir", dir,
					"error", err)
			}
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(s.filePath, ciphertext, 0600); err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session save failed: file write",
				"path", s.filePath,
				"error", err)
		}
		return fmt.Errorf("failed to write session file %s: %w", s.filePath, err)
	}

	if s.log.Enabled() {
		s.log.Infow("session saved successfully", "path", s.filePath, "url", rawSession.ConfluenceURL)
	}

	return nil
}

func (s *Store) Load(key string) (*Session, error) {
	if key == "" {
		if s.log.Enabled() {
			s.log.Error("session load rejected: encryption key is required")
		}
		return nil, fmt.Errorf("encryption key is required")
	}

	ciphertext, err := os.ReadFile(s.filePath)
	if err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session load failed: file read", "path", s.filePath, "error", err)
		}
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
		if s.log.Enabled() {
			s.log.Errorw("session load failed: cipher creation", "error", err)
		}
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session load failed: GCM creation", "error", err)
		}
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		if s.log.Enabled() {
			s.log.Errorw("session load failed: ciphertext too short", "len", len(ciphertext), "nonce_size", nonceSize)
		}
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	sessionJSON, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session load failed: decryption error (wrong key or corrupt file)",
				"error", err)
		}
		return nil, fmt.Errorf("failed to decrypt session (wrong key or corrupt file): %w", err)
	}

	var sess Session
	if err := json.Unmarshal(sessionJSON, &sess); err != nil {
		if s.log.Enabled() {
			s.log.Errorw("session load failed: unmarshal error", "error", err)
		}
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	sess.SetLogger(s.log)

	if s.log.Enabled() {
		s.log.Infow("session loaded successfully",
			"path", s.filePath,
			"url", sess.ConfluenceURL,
			"cookie_count", len(sess.Cookies))
	}

	return &sess, nil
}

func (s *Store) Delete() error {
	if !s.HasSession() {
		if s.log.Enabled() {
			s.log.Info("session delete: no session file to delete")
		}
		return nil
	}

	// In Docker, we can't easily remove/rename a volume-mounted file.
	// We truncate it instead to clear the sensitive data.
	if err := os.Truncate(s.filePath, 0); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		if s.log.Enabled() {
			s.log.Errorw("session delete failed: truncate", "path", s.filePath, "error", err)
		}
		return fmt.Errorf("failed to clear session file %s: %w", s.filePath, err)
	}

	if s.log.Enabled() {
		s.log.Infow("session cleared (truncated)", "path", s.filePath)
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
