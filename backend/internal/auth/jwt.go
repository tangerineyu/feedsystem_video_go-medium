// internal/auth/jwt.go
package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)
const (
	TokenTypeAccess = "access"
	TokenTypeRefresh = "refresh"
	accessTokenExpiry  = 15 * time.Minute
	refreshTokenExpiry = 7 * 24 * time.Hour
)

func AccessTokenTTL() time.Duration {
	return accessTokenExpiry
}

func RefreshTokenTTL() time.Duration {
	return refreshTokenExpiry
}

func jwtSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "change-me-in-env"
	}
	return []byte(secret)
}

type Claims struct {
	AccountID uint   `json:"account_id"`
	Username  string `json:"username"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(accountID uint, username string) (string, error) {
	return generateToken(accountID, username, TokenTypeAccess, accessTokenExpiry)
}

func GenerateRefreshToken(accountID uint, username string) (string, error) {
	return generateToken(accountID, username, TokenTypeRefresh, refreshTokenExpiry)
}

func GenerateTokenPair(accountID uint, username string) (string, string, error) {
	accessToken, err := GenerateAccessToken(accountID, username)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := GenerateRefreshToken(accountID, username)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
func generateToken(accountID uint, username string, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()

	claims := Claims{
		AccountID: accountID,
		Username:  username,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtSecret())
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return jwtSecret(), nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func ParseAccessToken(tokenString string) (*Claims, error) {
	return parseToken(tokenString, TokenTypeAccess)
}

func ParseRefreshToken(tokenString string) (*Claims, error) {
	return parseToken(tokenString, TokenTypeRefresh)
}

func parseToken(tokenString string, expectedType string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return jwtSecret(), nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	if claims.TokenType != expectedType {
		return nil, errors.New("unexpected token type")
	}

	return claims, nil
}