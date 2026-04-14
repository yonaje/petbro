package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/yonaje/friendservice/internal/clients"
	"github.com/yonaje/friendservice/internal/logger"
	"github.com/yonaje/friendservice/internal/middleware"
	"github.com/yonaje/friendservice/internal/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("friendservice/handler")

type FriendHandler struct {
	friendRepository repository.FriendRepository
	userClient       clients.UserClient
	log              *zap.Logger
}

func NewFriendHandler(friendRepository repository.FriendRepository, userClient clients.UserClient, log *zap.Logger) *FriendHandler {
	return &FriendHandler{
		friendRepository: friendRepository,
		userClient:       userClient,
		log:              log,
	}
}

func (h *FriendHandler) SendRequest(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "FriendHandler.SendRequest")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	var request struct {
		FromUserID int `json:"-"`
		ToUserID   int `json:"toUserId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		if h.handleBadRequest(w, span, log, "FriendHandler.SendRequest", "decode_request", "Invalid request body", err) {
			return
		}
	}

	authUserID, ok := h.requireAuthenticatedUser(w, span, log, ctx, "FriendHandler.SendRequest")
	if !ok {
		return
	}

	request.FromUserID = authUserID

	span.SetAttributes(
		attribute.Int("friend.from_user_id", request.FromUserID),
		attribute.Int("friend.to_user_id", request.ToUserID),
	)

	if ok := h.ensureUsersExist(w, ctx, span, log, "FriendHandler.SendRequest", request.FromUserID, request.ToUserID); !ok {
		return
	}

	err := h.friendRepository.SendRequest(ctx, request.FromUserID, request.ToUserID)
	if h.handleRepositoryError(w, span, log, "FriendHandler.SendRequest", "send_request", "Failed to send friend request", err,
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	) {
		return
	}

	resp, err := json.Marshal(map[string]string{"message": "friend request sent"})
	if h.handleWriteError(span, log, "FriendHandler.SendRequest", "encode_response", "failed to encode response", err,
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if _, err := w.Write(resp); h.handleWriteError(span, log, "FriendHandler.SendRequest", "write_response", "failed to write response", err,
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	) {
		return
	}

	log.Info("Friend request sent successfully",
		zap.String("operation", "FriendHandler.SendRequest"),
		zap.String("step", "request_sent"),
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	)
	span.SetStatus(codes.Ok, "friend request sent successfully")
}

func (h *FriendHandler) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "FriendHandler.AcceptRequest")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	var request struct {
		FromUserID int `json:"fromUserId"`
		ToUserID   int `json:"-"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		if h.handleBadRequest(w, span, log, "FriendHandler.AcceptRequest", "decode_request", "Invalid request body", err) {
			return
		}
	}

	authUserID, ok := h.requireAuthenticatedUser(w, span, log, ctx, "FriendHandler.AcceptRequest")
	if !ok {
		return
	}

	request.ToUserID = authUserID

	span.SetAttributes(
		attribute.Int("friend.from_user_id", request.FromUserID),
		attribute.Int("friend.to_user_id", request.ToUserID),
	)

	if ok := h.ensureUsersExist(w, ctx, span, log, "FriendHandler.AcceptRequest", request.FromUserID, request.ToUserID); !ok {
		return
	}

	err := h.friendRepository.AcceptRequest(ctx, request.FromUserID, request.ToUserID)
	if h.handleRepositoryError(w, span, log, "FriendHandler.AcceptRequest", "accept_request", "Failed to accept friend request", err,
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	) {
		return
	}

	resp, err := json.Marshal(map[string]string{"message": "friend request accepted"})
	if h.handleWriteError(span, log, "FriendHandler.AcceptRequest", "encode_response", "failed to encode response", err,
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(resp); h.handleWriteError(span, log, "FriendHandler.AcceptRequest", "write_response", "failed to write response", err,
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	) {
		return
	}

	log.Info("Friend request accepted successfully",
		zap.String("operation", "FriendHandler.AcceptRequest"),
		zap.String("step", "request_accepted"),
		zap.Int("from_user_id", request.FromUserID),
		zap.Int("to_user_id", request.ToUserID),
	)
	span.SetStatus(codes.Ok, "friend request accepted successfully")
}

func (h *FriendHandler) Friends(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "FriendHandler.Friends")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	userID, ok := h.requireAuthenticatedUser(w, span, log, ctx, "FriendHandler.Friends")
	if !ok {
		return
	}

	span.SetAttributes(attribute.Int("user.id", userID))

	if ok := h.ensureUsersExist(w, ctx, span, log, "FriendHandler.Friends", userID); !ok {
		return
	}

	friends, err := h.friendRepository.ListFriends(ctx, userID)
	if h.handleRepositoryError(w, span, log, "FriendHandler.Friends", "list_friends", "Failed to get friends", err,
		zap.Int("user_id", userID),
	) {
		return
	}

	resp, err := json.Marshal(friends)
	if h.handleWriteError(span, log, "FriendHandler.Friends", "encode_response", "failed to encode response", err,
		zap.Int("user_id", userID),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(resp); h.handleWriteError(span, log, "FriendHandler.Friends", "write_response", "failed to write response", err,
		zap.Int("user_id", userID),
	) {
		return
	}

	log.Info("Friends retrieved successfully",
		zap.String("operation", "FriendHandler.Friends"),
		zap.String("step", "friends_retrieved"),
		zap.Int("user_id", userID),
		zap.Int("friend_count", len(friends)),
	)
	span.SetStatus(codes.Ok, "friends retrieved successfully")
}

func (h *FriendHandler) IncomingRequests(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "FriendHandler.IncomingRequests")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	userID, ok := h.requireAuthenticatedUser(w, span, log, ctx, "FriendHandler.IncomingRequests")
	if !ok {
		return
	}

	span.SetAttributes(attribute.Int("user.id", userID))

	if ok := h.ensureUsersExist(w, ctx, span, log, "FriendHandler.IncomingRequests", userID); !ok {
		return
	}

	requests, err := h.friendRepository.ListIncomingRequests(ctx, userID)
	if h.handleRepositoryError(w, span, log, "FriendHandler.IncomingRequests", "list_incoming_requests", "Failed to get incoming requests", err,
		zap.Int("user_id", userID),
	) {
		return
	}

	resp, err := json.Marshal(requests)
	if h.handleWriteError(span, log, "FriendHandler.IncomingRequests", "encode_response", "failed to encode response", err,
		zap.Int("user_id", userID),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(resp); h.handleWriteError(span, log, "FriendHandler.IncomingRequests", "write_response", "failed to write response", err,
		zap.Int("user_id", userID),
	) {
		return
	}

	log.Info("Incoming requests retrieved successfully",
		zap.String("operation", "FriendHandler.IncomingRequests"),
		zap.String("step", "incoming_requests_retrieved"),
		zap.Int("user_id", userID),
		zap.Int("request_count", len(requests)),
	)
	span.SetStatus(codes.Ok, "incoming requests retrieved successfully")
}

func (h *FriendHandler) Recommendations(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "FriendHandler.Recommendations")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.route", r.URL.Path),
	)

	userID, ok := h.requireAuthenticatedUser(w, span, log, ctx, "FriendHandler.Recommendations")
	if !ok {
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = 10
	}

	span.SetAttributes(
		attribute.Int("user.id", userID),
		attribute.Int("recommendations.limit", limit),
	)

	if ok := h.ensureUsersExist(w, ctx, span, log, "FriendHandler.Recommendations", userID); !ok {
		return
	}

	recommendations, err := h.friendRepository.Recommendations(ctx, userID, limit)
	if h.handleRepositoryError(w, span, log, "FriendHandler.Recommendations", "get_recommendations", "Failed to get recommendations", err,
		zap.Int("user_id", userID),
		zap.Int("limit", limit),
	) {
		return
	}

	resp, err := json.Marshal(recommendations)
	if h.handleWriteError(span, log, "FriendHandler.Recommendations", "encode_response", "failed to encode response", err,
		zap.Int("user_id", userID),
		zap.Int("limit", limit),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(resp); h.handleWriteError(span, log, "FriendHandler.Recommendations", "write_response", "failed to write response", err,
		zap.Int("user_id", userID),
		zap.Int("limit", limit),
	) {
		return
	}

	log.Info("Friend recommendations retrieved successfully",
		zap.String("operation", "FriendHandler.Recommendations"),
		zap.String("step", "recommendations_retrieved"),
		zap.Int("user_id", userID),
		zap.Int("limit", limit),
		zap.Int("recommendation_count", len(recommendations)),
	)
	span.SetStatus(codes.Ok, "friend recommendations retrieved successfully")
}

func (h *FriendHandler) ensureUsersExist(
	w http.ResponseWriter,
	ctx context.Context,
	span trace.Span,
	log *zap.Logger,
	operation string,
	userIDs ...int,
) bool {
	checked := make(map[int]struct{}, len(userIDs))

	for _, userID := range userIDs {
		if userID < 1 {
			if h.handleBadRequest(w, span, log, operation, "validate_user_id", "Invalid user ID", nil,
				zap.Int("user_id", userID),
			) {
				return false
			}
		}

		if _, exists := checked[userID]; exists {
			continue
		}

		exists, err := h.userClient.UserExists(ctx, userID)
		if err != nil {
			log.Error("Failed to validate user via user service",
				zap.String("operation", operation),
				zap.String("step", "validate_user"),
				zap.Int("user_id", userID),
				zap.Error(err),
			)
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to validate user")
			http.Error(w, "Failed to validate user", http.StatusBadGateway)
			return false
		}

		if !exists {
			log.Warn("User not found during validation",
				zap.String("operation", operation),
				zap.String("step", "validate_user"),
				zap.Int("user_id", userID),
			)
			span.SetStatus(codes.Error, "user not found")
			http.Error(w, "User not found", http.StatusNotFound)
			return false
		}

		checked[userID] = struct{}{}
	}

	return true
}

func (h *FriendHandler) handleBadRequest(
	w http.ResponseWriter,
	span trace.Span,
	log *zap.Logger,
	operation string,
	step string,
	message string,
	err error,
	fields ...zap.Field,
) bool {
	logFields := append([]zap.Field{
		zap.String("operation", operation),
		zap.String("step", step),
	}, fields...)
	if err != nil {
		logFields = append(logFields, zap.Error(err))
		log.Warn(message, logFields...)
		span.RecordError(err)
	} else {
		log.Warn(message, logFields...)
	}

	span.SetStatus(codes.Error, message)
	http.Error(w, message, http.StatusBadRequest)
	return true
}

func (h *FriendHandler) requireAuthenticatedUser(
	w http.ResponseWriter,
	span trace.Span,
	log *zap.Logger,
	ctx context.Context,
	operation string,
) (int, bool) {
	userIDRaw, ok := middleware.UserIDFromContext(ctx)
	if !ok || userIDRaw == "" {
		log.Error("Authenticated user is missing from request context",
			zap.String("operation", operation),
			zap.String("step", "read_authenticated_user"),
		)
		span.SetStatus(codes.Error, "authenticated user missing")
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return 0, false
	}

	userID, err := strconv.Atoi(userIDRaw)
	if err != nil || userID < 1 {
		log.Error("Authenticated user id is invalid",
			zap.String("operation", operation),
			zap.String("step", "parse_authenticated_user"),
			zap.String("auth_user_id", userIDRaw),
			zap.Error(err),
		)
		if err != nil {
			span.RecordError(err)
		}
		span.SetStatus(codes.Error, "invalid authenticated user")
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return 0, false
	}

	span.SetAttributes(attribute.Int("auth.user_id", userID))
	return userID, true
}

func (h *FriendHandler) handleRepositoryError(
	w http.ResponseWriter,
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

	status := http.StatusInternalServerError
	responseMessage := "Internal server error"
	level := log.Error

	switch {
	case errors.Is(err, repository.ErrSelfRequest):
		status = http.StatusBadRequest
		responseMessage = err.Error()
		level = log.Warn
	case errors.Is(err, repository.ErrAlreadyFriends),
		errors.Is(err, repository.ErrRequestAlreadyExists),
		errors.Is(err, repository.ErrIncomingRequestExists):
		status = http.StatusConflict
		responseMessage = err.Error()
		level = log.Warn
	case errors.Is(err, repository.ErrRequestNotFound):
		status = http.StatusNotFound
		responseMessage = err.Error()
		level = log.Warn
	}

	logFields := append([]zap.Field{
		zap.String("operation", operation),
		zap.String("step", step),
	}, fields...)
	logFields = append(logFields, zap.Error(err))

	level(message, logFields...)
	span.RecordError(err)
	span.SetStatus(codes.Error, message)
	http.Error(w, responseMessage, status)
	return true
}

func (h *FriendHandler) handleWriteError(
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

	logFields := append([]zap.Field{
		zap.String("operation", operation),
		zap.String("step", step),
		zap.Error(err),
	}, fields...)
	log.Error(message, logFields...)
	span.RecordError(err)
	span.SetStatus(codes.Error, message)
	return true
}
