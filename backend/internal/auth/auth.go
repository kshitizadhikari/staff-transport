// Package auth owns authentication and session/token concerns.
//
// Access tokens are short-lived JWTs. Refresh tokens are opaque random
// values whose live state and revocation are stored in Redis (ADR-007).
// Business modules depend on the Service interface, not on JWT details.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	goredis "github.com/redis/go-redis/v9"

	"staff-transport/internal/config"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type Claims struct {
	UserID string
	Role   string
}

// Service is the authentication boundary consumed by HTTP handlers.
type Service interface {
	Issue(ctx context.Context, userID, role string) (TokenPair, error)
	ConsumeRefresh(ctx context.Context, refreshToken string) (userID string, err error)
	Revoke(ctx context.Context, refreshToken string) error
	ParseAccessToken(accessToken string) (*Claims, error)
}

type service struct {
	cfg *config.Config
	rdb *goredis.Client
}

func NewService(cfg *config.Config, rdb *goredis.Client) Service {
	return &service{cfg: cfg, rdb: rdb}
}

func (s *service) Issue(ctx context.Context, userID, role string) (TokenPair, error) {
	access, err := s.signAccessToken(userID, role)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := s.newRefreshToken(ctx, userID)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

// ConsumeRefresh validates a refresh token, deletes it (rotation), and returns
// the owning user ID. The caller re-resolves the current role from the database
// before issuing new tokens, so role changes take effect on refresh.
func (s *service) ConsumeRefresh(ctx context.Context, refreshToken string) (string, error) {
	if refreshToken == "" {
		return "", ErrInvalidToken
	}
	key := refreshKey(refreshToken)
	userID, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("lookup refresh token: %w", err)
	}
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		return "", fmt.Errorf("rotate refresh token: %w", err)
	}
	return userID, nil
}

func (s *service) Revoke(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.rdb.Del(ctx, refreshKey(refreshToken)).Err()
}

func (s *service) ParseAccessToken(accessToken string) (*Claims, error) {
	parsed, err := jwt.Parse(accessToken, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTAccessSecret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	if sub == "" {
		return nil, ErrInvalidToken
	}
	return &Claims{UserID: sub, Role: role}, nil
}

func (s *service) signAccessToken(userID, role string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"iat":  now.Unix(),
		"exp":  now.Add(s.cfg.AccessTokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTAccessSecret))
}

func (s *service) newRefreshToken(ctx context.Context, userID string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if err := s.rdb.Set(ctx, refreshKey(token), userID, s.cfg.RefreshTokenTTL).Err(); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return token, nil
}

func refreshKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "auth:refresh:" + hex.EncodeToString(sum[:])
}
