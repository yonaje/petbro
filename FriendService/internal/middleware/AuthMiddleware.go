package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yonaje/friendservice/internal/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

var authTracer = otel.Tracer("friendservice/middleware/auth")

type contextKey string

const userIDContextKey contextKey = "auth.user_id"

type AuthMiddleware struct {
	signingKey []byte
	log        *zap.Logger
}

func NewAuthMiddleware(secret string, log *zap.Logger) (*AuthMiddleware, error) {
	if secret == "" {
		return nil, errors.New("jwt signing key is not configured")
	}

	return &AuthMiddleware{
		signingKey: []byte(secret),
		log:        log,
	}, nil
}

func (m *AuthMiddleware) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := authTracer.Start(r.Context(), "AuthMiddleware.Protect")
		defer span.End()

		log := logger.WithTrace(ctx, m.log)
		authHeader := r.Header.Get("Authorization")
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.URL.Path),
			attribute.Bool("http.authorization_present", authHeader != ""),
		)

		if authHeader == "" {
			log.Warn("Missing authorization header",
				zap.String("operation", "AuthMiddleware.Protect"),
				zap.String("step", "read_authorization_header"),
			)
			span.SetStatus(codes.Error, "missing access token")
			http.Error(w, "Missing access token", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Warn("Invalid authorization header format",
				zap.String("operation", "AuthMiddleware.Protect"),
				zap.String("step", "parse_authorization_header"),
			)
			span.SetStatus(codes.Error, "invalid access token")
			http.Error(w, "Invalid access token", http.StatusUnauthorized)
			return
		}

		claims, err := m.validateJWT(parts[1])
		if err != nil {
			log.Warn("Failed to validate access token",
				zap.String("operation", "AuthMiddleware.Protect"),
				zap.String("step", "validate_access_token"),
				zap.Error(err),
			)
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid access token")
			http.Error(w, "Invalid access token", http.StatusUnauthorized)
			return
		}

		userID, ok := claims["user_id"].(string)
		if !ok || userID == "" {
			log.Warn("Access token does not contain string user_id",
				zap.String("operation", "AuthMiddleware.Protect"),
				zap.String("step", "extract_user_id"),
			)
			span.SetStatus(codes.Error, "invalid access token claims")
			http.Error(w, "Invalid access token", http.StatusUnauthorized)
			return
		}

		span.SetAttributes(attribute.String("auth.user_id", userID))
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, userIDContextKey, userID)))
	})
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}

func (m *AuthMiddleware) validateJWT(tokenString string) (map[string]any, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode jwt header: %w", err)
	}

	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("unmarshal jwt header: %w", err)
	}

	if header.Alg != "HS256" {
		return nil, fmt.Errorf("unexpected signing method: %s", header.Alg)
	}

	mac := hmac.New(sha256.New, m.signingKey)
	if _, err := mac.Write([]byte(parts[0] + "." + parts[1])); err != nil {
		return nil, fmt.Errorf("sign jwt payload: %w", err)
	}

	expectedSignature := mac.Sum(nil)
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode jwt signature: %w", err)
	}

	if !hmac.Equal(signature, expectedSignature) {
		return nil, errors.New("invalid token signature")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode jwt claims: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal jwt claims: %w", err)
	}

	if err := validateExpClaim(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

func validateExpClaim(claims map[string]any) error {
	exp, ok := claims["exp"]
	if !ok {
		return errors.New("missing exp claim")
	}

	switch value := exp.(type) {
	case float64:
		if time.Now().Unix() > int64(value) {
			return errors.New("token expired")
		}
	case json.Number:
		expUnix, err := value.Int64()
		if err != nil {
			return fmt.Errorf("invalid exp claim: %w", err)
		}
		if time.Now().Unix() > expUnix {
			return errors.New("token expired")
		}
	case string:
		expUnix, err := json.Number(value).Int64()
		if err != nil {
			return fmt.Errorf("invalid exp claim: %w", err)
		}
		if time.Now().Unix() > expUnix {
			return errors.New("token expired")
		}
	default:
		return errors.New("invalid exp claim type")
	}

	return nil
}
