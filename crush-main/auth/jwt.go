package auth

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rolling1314/rolling-crush/pkg/config"
)

var (
	// JWT secret key - loaded lazily from config
	jwtSecret     []byte
	jwtSecretOnce sync.Once

	// Token expiration time in hours
	tokenExpireHour = 24

	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
)

// Claims represents the JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// initJWTSecret initializes the JWT secret from config (called once)
func initJWTSecret() {
	jwtSecretOnce.Do(func() {
		appCfg := config.GetGlobalAppConfig()
		if appCfg != nil && appCfg.Auth.JWTSecret != "" {
			jwtSecret = []byte(appCfg.Auth.JWTSecret)
			if appCfg.Auth.TokenExpireHour > 0 {
				tokenExpireHour = appCfg.Auth.TokenExpireHour
			}
			slog.Info("JWT secret loaded from config.yaml", "expire_hours", tokenExpireHour)
		} else {
			// Default secret for development
			defaultSecret := "crush-dev-jwt-secret-change-in-production-2024"
			jwtSecret = []byte(defaultSecret)
			slog.Warn("Using default JWT secret. Please configure auth.jwt_secret in config.yaml!")
		}
	})
}

// getJWTSecret returns the JWT secret, initializing if needed
func getJWTSecret() []byte {
	initJWTSecret()
	return jwtSecret
}

// GenerateToken generates a new JWT token for a user
func GenerateToken(userID, username string) (string, error) {
	secret := getJWTSecret()
	expirationTime := time.Now().Add(time.Duration(tokenExpireHour) * time.Hour)

	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "crush-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*Claims, error) {
	secret := getJWTSecret()

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSignature
		}
		return secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
