package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

type AccessClaims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID    string   `json:"user_id"`
	Roles     []string `json:"roles"`
	SessionID string   `json:"session_id"`
	jwt.RegisteredClaims
}

func NewJWTManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (m *JWTManager) NewAccessToken(userID uuid.UUID, roles []string, now time.Time) (string, time.Time, error) {
	exp := now.Add(m.accessTTL)
	claims := AccessClaims{
		UserID: userID.String(),
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.accessSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, exp, nil
}

func (m *JWTManager) NewRefreshToken(sessionID uuid.UUID, userID uuid.UUID, roles []string, now time.Time) (string, time.Time, error) {
	exp := now.Add(m.refreshTTL)
	claims := RefreshClaims{
		UserID:    userID.String(),
		Roles:     roles,
		SessionID: sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.refreshSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign refresh token: %w", err)
	}
	return signed, exp, nil
}

func (m *JWTManager) ParseAccess(tokenString string) (AccessClaims, error) {
	var claims AccessClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.accessSecret, nil
	}, jwt.WithLeeway(30*time.Second))
	if err != nil {
		return AccessClaims{}, err
	}
	return claims, nil
}

func (m *JWTManager) ParseRefresh(tokenString string) (RefreshClaims, error) {
	var claims RefreshClaims
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.refreshSecret, nil
	}, jwt.WithLeeway(30*time.Second))
	if err != nil {
		return RefreshClaims{}, err
	}
	return claims, nil
}
