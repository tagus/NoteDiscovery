package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	secret       []byte
	passwordHash string
	enabled      bool
	maxAge       int
}

func NewService(secretKey string, passwordHash string, enabled bool, maxAge int) *Service {
	return &Service{secret: []byte(secretKey), passwordHash: passwordHash, enabled: enabled, maxAge: maxAge}
}

func (s *Service) Enabled() bool {
	return s.enabled
}

func (s *Service) MaxAge() int {
	return s.maxAge
}

func (s *Service) VerifyPassword(password string) bool {
	if s.passwordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(s.passwordHash), []byte(password)) == nil
}

func HashPassword(password string) (string, error) {
	buf, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (s *Service) NewCookieValue() string {
	expiry := time.Now().Add(time.Duration(s.maxAge) * time.Second).Unix()
	payload := fmt.Sprintf("1:%d", expiry)
	sig := s.sign(payload)
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + sig))
}

func (s *Service) IsAuthenticated(raw string) bool {
	if !s.enabled {
		return true
	}
	if raw == "" {
		return false
	}

	buf, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return false
	}
	parts := strings.Split(string(buf), ":")
	if len(parts) != 3 || parts[0] != "1" {
		return false
	}
	payload := strings.Join(parts[:2], ":")
	if !hmac.Equal([]byte(parts[2]), []byte(s.sign(payload))) {
		return false
	}

	var expiry int64
	if _, err := fmt.Sscanf(parts[1], "%d", &expiry); err != nil {
		return false
	}
	return time.Now().Unix() < expiry
}

func (s *Service) sign(payload string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
