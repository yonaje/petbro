package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yonaje/authservice/internal/clients"
	"github.com/yonaje/authservice/internal/jwt"
	"github.com/yonaje/authservice/internal/logger"
	"github.com/yonaje/authservice/internal/metrics"
	"github.com/yonaje/authservice/internal/models"
	"github.com/yonaje/authservice/internal/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var tracer = otel.Tracer("authservice/handler")

type AuthHandler struct {
	AuthRepository repository.AuthRepository
	UserClient     clients.UserClient
	log            *zap.Logger
}

func NewAuthHandler(authRepository repository.AuthRepository, userClient clients.UserClient, log *zap.Logger) *AuthHandler {
	return &AuthHandler{
		AuthRepository: authRepository,
		UserClient:     userClient,
		log:            log,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "AuthHandler.Register")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Username    string `json:"name"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.IncAuthEvent("register", "bad_request")
		log.Warn("Invalid register request body",
			zap.String("operation", "AuthHandler.Register"),
			zap.String("step", "decode_request"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" || req.Username == "" {
		metrics.IncAuthEvent("register", "bad_request")
		span.SetAttributes(
			attribute.Bool("auth.email_present", req.Email != ""),
			attribute.Bool("auth.password_present", req.Password != ""),
			attribute.Bool("user.username_present", req.Username != ""),
		)
		log.Warn("Missing required register fields",
			zap.String("operation", "AuthHandler.Register"),
			zap.String("step", "validate_request"),
		)
		span.SetStatus(codes.Error, "missing required fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}
	span.SetAttributes(
		attribute.Bool("auth.email_present", req.Email != ""),
		attribute.Bool("user.username_present", req.Username != ""),
		attribute.Bool("user.avatar_present", req.Avatar != ""),
		attribute.Bool("user.description_present", req.Description != ""),
	)

	_, err := h.AuthRepository.GetByEmail(ctx, req.Email)
	if err == nil {
		metrics.IncAuthEvent("register", "conflict")
		log.Warn("Register rejected because email already exists",
			zap.String("operation", "AuthHandler.Register"),
			zap.String("step", "check_email"),
		)
		span.SetStatus(codes.Error, "email already exists")
		http.Error(w, "Email already exists", http.StatusConflict)
		return
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		metrics.IncAuthEvent("register", "error")
		log.Error("Failed to check existing auth account",
			zap.String("operation", "AuthHandler.Register"),
			zap.String("step", "check_email"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to check auth account")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userID, err := h.UserClient.CreateUser(ctx, req.Username, req.Description, req.Avatar)
	if err != nil {
		metrics.IncAuthEvent("register", "error")
		log.Error("Failed to create user profile",
			zap.String("operation", "AuthHandler.Register"),
			zap.String("step", "create_user"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create user")
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	span.SetAttributes(attribute.Int("auth.user_id", userID))

	registrationCompleted := false
	defer func() {
		if registrationCompleted {
			return
		}

		compensationCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.UserClient.DeleteUser(compensationCtx, userID); err != nil {
			logger.WithTrace(ctx, h.log).Error("Failed to rollback user after register failure",
				zap.String("operation", "AuthHandler.Register"),
				zap.String("step", "rollback_user"),
				zap.Int("user_id", userID),
				zap.Error(err),
			)
			span.RecordError(err)
		}
	}()

	hash, err := jwt.GenerateHash(req.Password)
	if err != nil {
		metrics.IncAuthEvent("register", "error")
		log.Error("Failed to hash password",
			zap.String("operation", "AuthHandler.Register"),
			zap.String("step", "generate_hash"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to hash password")
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	auth := &models.AuthAccount{
		UserID:       uint(userID),
		Email:        req.Email,
		PasswordHash: hash,
		Status:       true,
	}

	if err := h.AuthRepository.Create(ctx, auth); err != nil {
		metrics.IncAuthEvent("register", "error")
		log.Error("Failed to create auth account",
			zap.String("operation", "AuthHandler.Register"),
			zap.String("step", "create_auth_account"),
			zap.Int("user_id", userID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create auth account")
		http.Error(w, "Failed to create auth account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	log.Info("Auth account registered successfully",
		zap.String("operation", "AuthHandler.Register"),
		zap.String("step", "register_success"),
		zap.Int("user_id", userID),
	)
	metrics.IncAuthEvent("register", "success")
	registrationCompleted = true
	span.SetStatus(codes.Ok, "auth account registered successfully")
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "AuthHandler.Login")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		metrics.IncAuthEvent("login", "bad_request")
		log.Warn("Invalid login request body",
			zap.String("operation", "AuthHandler.Login"),
			zap.String("step", "decode_request"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	span.SetAttributes(
		attribute.Bool("auth.email_present", req.Email != ""),
		attribute.Bool("auth.password_present", req.Password != ""),
	)

	auth, err := h.AuthRepository.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			metrics.IncAuthEvent("login", "invalid_credentials")
			log.Warn("Login failed because account was not found",
				zap.String("operation", "AuthHandler.Login"),
				zap.String("step", "get_by_email"),
			)
			span.SetStatus(codes.Error, "account not found")
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		metrics.IncAuthEvent("login", "error")
		log.Error("Failed to load auth account",
			zap.String("operation", "AuthHandler.Login"),
			zap.String("step", "get_by_email"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to load auth account")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !auth.Status {
		metrics.IncAuthEvent("login", "blocked")
		log.Warn("Login rejected because account is blocked",
			zap.String("operation", "AuthHandler.Login"),
			zap.String("step", "check_status"),
			zap.Uint("user_id", auth.UserID),
		)
		span.SetStatus(codes.Error, "account is blocked")
		http.Error(w, "Account is blocked", http.StatusForbidden)
		return
	}

	if jwt.CompareHash(auth.PasswordHash, req.Password) != nil {
		metrics.IncAuthEvent("login", "invalid_credentials")
		log.Warn("Login failed because password is invalid",
			zap.String("operation", "AuthHandler.Login"),
			zap.String("step", "compare_password"),
			zap.Uint("user_id", auth.UserID),
		)
		span.SetStatus(codes.Error, "invalid credentials")
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	userID := strconv.Itoa(int(auth.UserID))
	span.SetAttributes(attribute.String("auth.user_id", userID))

	accessToken, err := jwt.GenerateJWT(userID)
	if err != nil {
		metrics.IncAuthEvent("login", "error")
		log.Error("Failed to generate access token",
			zap.String("operation", "AuthHandler.Login"),
			zap.String("step", "generate_access_token"),
			zap.String("user_id", userID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to generate access token")
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := jwt.GenerateRefreshToken()
	if err != nil {
		metrics.IncAuthEvent("login", "error")
		log.Error("Failed to generate refresh token",
			zap.String("operation", "AuthHandler.Login"),
			zap.String("step", "generate_refresh_token"),
			zap.String("user_id", userID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to generate refresh token")
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	session := &models.Session{
		ID:           uuid.NewString(),
		UserID:       userID,
		RefreshToken: jwt.HashRefreshToken(refreshToken),
		UserAgent:    r.UserAgent(),
		IP:           r.RemoteAddr,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}
	span.SetAttributes(
		attribute.String("session.id", session.ID),
		attribute.String("session.user_id", session.UserID),
	)

	if err := h.AuthRepository.SaveSession(ctx, session); err != nil {
		metrics.IncAuthEvent("login", "error")
		log.Error("Failed to save session",
			zap.String("operation", "AuthHandler.Login"),
			zap.String("step", "save_session"),
			zap.String("user_id", userID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to save session")
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})

	if h.writeJSON(w, span, log, http.StatusOK, map[string]string{
		"access_token": accessToken,
	}, "AuthHandler.Login",
		zap.String("user_id", userID),
	) {
		return
	}

	log.Info("User logged in successfully",
		zap.String("operation", "AuthHandler.Login"),
		zap.String("step", "login_success"),
		zap.String("user_id", userID),
	)
	metrics.IncAuthEvent("login", "success")
	span.SetStatus(codes.Ok, "user logged in successfully")
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "AuthHandler.Logout")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		metrics.IncAuthEvent("logout", "bad_request")
		log.Warn("Missing refresh token on logout",
			zap.String("operation", "AuthHandler.Logout"),
			zap.String("step", "read_cookie"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "missing refresh token")
		http.Error(w, "Missing refresh token", http.StatusBadRequest)
		return
	}
	span.SetAttributes(attribute.Bool("session.refresh_token_present", cookie.Value != ""))

	if err := h.AuthRepository.DeleteSessionByRefreshToken(ctx, cookie.Value); err != nil {
		metrics.IncAuthEvent("logout", "error")
		log.Error("Failed to delete session",
			zap.String("operation", "AuthHandler.Logout"),
			zap.String("step", "delete_session"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete session")
		http.Error(w, "Failed to logout", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	log.Info("User logged out successfully",
		zap.String("operation", "AuthHandler.Logout"),
		zap.String("step", "logout_success"),
	)
	metrics.IncAuthEvent("logout", "success")
	span.SetStatus(codes.Ok, "user logged out successfully")
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "AuthHandler.Refresh")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		metrics.IncAuthEvent("refresh", "bad_request")
		log.Warn("Missing refresh token on refresh",
			zap.String("operation", "AuthHandler.Refresh"),
			zap.String("step", "read_cookie"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "missing refresh token")
		http.Error(w, "Missing refresh token", http.StatusBadRequest)
		return
	}
	span.SetAttributes(attribute.Bool("session.refresh_token_present", cookie.Value != ""))

	session, err := h.AuthRepository.GetSessionByRefreshToken(ctx, cookie.Value)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			metrics.IncAuthEvent("refresh", "invalid_token")
			log.Warn("Refresh rejected because session was not found",
				zap.String("operation", "AuthHandler.Refresh"),
				zap.String("step", "get_session"),
			)
			span.SetStatus(codes.Error, "invalid refresh token")
			http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
			return
		}
		metrics.IncAuthEvent("refresh", "error")
		log.Error("Failed to get session by refresh token",
			zap.String("operation", "AuthHandler.Refresh"),
			zap.String("step", "get_session"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get session")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if time.Now().After(session.ExpiresAt) {
		metrics.IncAuthEvent("refresh", "expired")
		_ = h.AuthRepository.DeleteSessionByRefreshToken(ctx, cookie.Value)
		log.Warn("Refresh token expired",
			zap.String("operation", "AuthHandler.Refresh"),
			zap.String("step", "check_expiry"),
			zap.String("user_id", session.UserID),
		)
		span.SetStatus(codes.Error, "refresh token expired")
		http.Error(w, "Refresh token expired", http.StatusUnauthorized)
		return
	}
	span.SetAttributes(
		attribute.String("session.id", session.ID),
		attribute.String("session.user_id", session.UserID),
	)

	accessToken, err := jwt.GenerateJWT(session.UserID)
	if err != nil {
		metrics.IncAuthEvent("refresh", "error")
		log.Error("Failed to generate access token during refresh",
			zap.String("operation", "AuthHandler.Refresh"),
			zap.String("step", "generate_access_token"),
			zap.String("user_id", session.UserID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to generate access token")
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	newRefreshToken, err := jwt.GenerateRefreshToken()
	if err != nil {
		metrics.IncAuthEvent("refresh", "error")
		log.Error("Failed to generate rotated refresh token",
			zap.String("operation", "AuthHandler.Refresh"),
			zap.String("step", "generate_refresh_token"),
			zap.String("user_id", session.UserID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to generate refresh token")
		http.Error(w, "Failed to refresh session", http.StatusInternalServerError)
		return
	}

	session.RefreshToken = jwt.HashRefreshToken(newRefreshToken)
	session.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)

	if err := h.AuthRepository.UpdateSession(ctx, session); err != nil {
		metrics.IncAuthEvent("refresh", "error")
		log.Error("Failed to rotate refresh token",
			zap.String("operation", "AuthHandler.Refresh"),
			zap.String("step", "update_session"),
			zap.String("user_id", session.UserID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to rotate refresh token")
		http.Error(w, "Failed to refresh session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})

	if h.writeJSON(w, span, log, http.StatusOK, map[string]string{
		"access_token": accessToken,
	}, "AuthHandler.Refresh",
		zap.String("user_id", session.UserID),
	) {
		return
	}

	log.Info("Access token refreshed successfully",
		zap.String("operation", "AuthHandler.Refresh"),
		zap.String("step", "refresh_success"),
		zap.String("user_id", session.UserID),
	)
	metrics.IncAuthEvent("refresh", "success")
	span.SetStatus(codes.Ok, "access token refreshed successfully")
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "AuthHandler.Me")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	authHeader := r.Header.Get("Authorization")
	span.SetAttributes(attribute.Bool("http.authorization_present", authHeader != ""))
	if authHeader == "" {
		metrics.IncAuthEvent("me", "unauthorized")
		log.Warn("Missing authorization header",
			zap.String("operation", "AuthHandler.Me"),
			zap.String("step", "read_authorization_header"),
		)
		span.SetStatus(codes.Error, "missing access token")
		http.Error(w, "Missing access token", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		metrics.IncAuthEvent("me", "unauthorized")
		log.Warn("Invalid authorization header format",
			zap.String("operation", "AuthHandler.Me"),
			zap.String("step", "parse_authorization_header"),
		)
		span.SetStatus(codes.Error, "invalid access token")
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	tokenString := parts[1]

	claims, err := jwt.ValidateJWT(tokenString)
	if err != nil {
		metrics.IncAuthEvent("me", "unauthorized")
		log.Warn("Failed to validate access token",
			zap.String("operation", "AuthHandler.Me"),
			zap.String("step", "validate_access_token"),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid access token")
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		metrics.IncAuthEvent("me", "unauthorized")
		log.Warn("Access token does not contain string user_id",
			zap.String("operation", "AuthHandler.Me"),
			zap.String("step", "extract_user_id"),
		)
		span.SetStatus(codes.Error, "invalid access token claims")
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}
	span.SetAttributes(attribute.String("auth.user_id", userID))

	auth, err := h.AuthRepository.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			metrics.IncAuthEvent("me", "not_found")
			log.Warn("Auth account not found for current user",
				zap.String("operation", "AuthHandler.Me"),
				zap.String("step", "get_by_user_id"),
				zap.String("user_id", userID),
			)
			span.SetStatus(codes.Error, "user not found")
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		metrics.IncAuthEvent("me", "error")
		log.Error("Failed to load current auth account",
			zap.String("operation", "AuthHandler.Me"),
			zap.String("step", "get_by_user_id"),
			zap.String("user_id", userID),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to load user")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if h.writeJSON(w, span, log, http.StatusOK, map[string]any{
		"user_id": auth.UserID,
		"email":   auth.Email,
		"status":  auth.Status,
	}, "AuthHandler.Me",
		zap.String("user_id", userID),
	) {
		return
	}

	log.Info("Current user loaded successfully",
		zap.String("operation", "AuthHandler.Me"),
		zap.String("step", "me_success"),
		zap.String("user_id", userID),
	)
	metrics.IncAuthEvent("me", "success")
	span.SetStatus(codes.Ok, "current user loaded successfully")
}

func (h *AuthHandler) handleWriteError(
	span trace.Span,
	log *zap.Logger,
	operation string,
	step string,
	message string,
	err error,
	fields ...zap.Field,
) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		metrics.IncAuthEvent(step, "canceled")
		log.Warn("Request canceled during response write", append(fields,
			zap.String("operation", operation),
			zap.String("step", step),
			zap.Error(err),
		)...)

		span.RecordError(err)
		span.SetStatus(codes.Error, "request canceled")
		return true
	}

	log.Error(message, append(fields,
		zap.String("operation", operation),
		zap.String("step", step),
		zap.Error(err),
	)...)

	metrics.IncAuthEvent(step, "write_error")
	span.RecordError(err)
	span.SetStatus(codes.Error, message)
	return true
}

func (h *AuthHandler) writeJSON(
	w http.ResponseWriter,
	span trace.Span,
	log *zap.Logger,
	status int,
	payload any,
	operation string,
	fields ...zap.Field,
) bool {
	resp, err := json.Marshal(payload)
	if h.handleWriteError(span, log, operation, "encode_response", "failed to encode response", err, fields...) {
		return true
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(resp); h.handleWriteError(span, log, operation, "write_response", "failed to write response", err, fields...) {
		return true
	}

	return false
}
