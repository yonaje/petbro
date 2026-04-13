package jwt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	signingKey          []byte
	errJWTNotConfigured = errors.New("jwt signing key is not configured")
)

func SetSigningKey(secret string) error {
	if secret == "" {
		return errJWTNotConfigured
	}

	signingKey = []byte(secret)
	return nil
}

func GenerateJWT(userID string) (string, error) {
	if len(signingKey) == 0 {
		return "", errJWTNotConfigured
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	})

	return token.SignedString(signingKey)
}

func ValidateJWT(tokenString string) (jwt.MapClaims, error) {
	if len(signingKey) == 0 {
		return nil, errJWTNotConfigured
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}

		return signingKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func CompareRefreshToken(hash, token string) bool {
	expected := HashRefreshToken(token)
	return hash == expected
}
