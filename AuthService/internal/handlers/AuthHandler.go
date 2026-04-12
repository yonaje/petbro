package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yonaje/authservice/internal/clients"
	"github.com/yonaje/authservice/internal/models"
	"github.com/yonaje/authservice/internal/repository"
	"github.com/yonaje/authservice/internal/utils"
	"gorm.io/gorm"
)

type AuthHandler struct {
	AuthRepository repository.AuthRepository
	UserClient     clients.UserClient
}

func NewAuthHandler(authRepository repository.AuthRepository, userClient clients.UserClient) *AuthHandler {
	return &AuthHandler{
		AuthRepository: authRepository,
		UserClient:     userClient,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Username    string `json:"name"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" || req.Username == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	_, err := h.AuthRepository.GetByEmail(ctx, req.Email)
	if err == nil {
		http.Error(w, "Email already exists", http.StatusConflict)
		return
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userID, err := h.UserClient.CreateUser(ctx, req.Username, req.Description, req.Avatar)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	hash, err := utils.GenerateHash(req.Password)
	if err != nil {
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
		http.Error(w, "Failed to create auth account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	auth, err := h.AuthRepository.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !auth.Status {
		http.Error(w, "Account is blocked", http.StatusForbidden)
		return
	}

	if utils.CompareHash(auth.PasswordHash, req.Password) != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	userID := strconv.Itoa(int(auth.UserID))

	accessToken, err := utils.GenerateJWT(userID)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := utils.GenerateRefreshToken()
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	session := &models.Session{
		ID:           uuid.NewString(),
		UserID:       userID,
		RefreshToken: refreshToken,
		UserAgent:    r.UserAgent(),
		IP:           r.RemoteAddr,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	if err := h.AuthRepository.SaveSession(ctx, session); err != nil {
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "Missing refresh token", http.StatusBadRequest)
		return
	}

	if err := h.AuthRepository.DeleteSessionByRefreshToken(ctx, cookie.Value); err != nil {
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
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "Missing refresh token", http.StatusBadRequest)
		return
	}

	session, err := h.AuthRepository.GetSessionByRefreshToken(ctx, cookie.Value)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if time.Now().After(session.ExpiresAt) {
		_ = h.AuthRepository.DeleteSessionByRefreshToken(ctx, cookie.Value)
		http.Error(w, "Refresh token expired", http.StatusUnauthorized)
		return
	}

	accessToken, err := utils.GenerateJWT(session.UserID)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing access token", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	tokenString := parts[1]

	claims, err := utils.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	auth, err := h.AuthRepository.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]any{
		"user_id": auth.UserID,
		"email":   auth.Email,
		"status":  auth.Status,
	})
}
