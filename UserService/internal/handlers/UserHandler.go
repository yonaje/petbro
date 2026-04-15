package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/yonaje/userservice/internal/logger"
	"github.com/yonaje/userservice/internal/metrics"
	"github.com/yonaje/userservice/internal/models"
	"github.com/yonaje/userservice/internal/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var tracer = otel.Tracer("userservice/handler")

type UserHandler struct {
	UserRepository repository.UserRepository
	log            *zap.Logger
}

func NewUserHandler(userRepository repository.UserRepository, log *zap.Logger) *UserHandler {
	return &UserHandler{
		UserRepository: userRepository,
		log:            log,
	}
}

func (h *UserHandler) Users(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "UsersHandler.Users")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	sort := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	switch sort {
	case "id", "username":
	default:
		sort = "id"
	}

	if order != "desc" {
		order = "asc"
	}

	sort = sort + " " + order

	users, err := h.UserRepository.Users(ctx, page, limit, sort)
	if h.handleRepositoryError(w, span, log, "UserHandler.Users", "fetch_users", "Failed to get users", err,
		zap.String("parameters", "page="+strconv.Itoa(page)+", limit="+strconv.Itoa(limit)+", sort="+sort),
	) {
		span.SetAttributes(
			attribute.Int("pagination.page", page),
			attribute.Int("pagination.limit", limit),
			attribute.String("pagination.sort", sort),
		)
		return
	}

	resp, err := json.Marshal(users)
	if h.handleWriteError(span, log, "UserHandler.Users", "encode_response", "failed to encode response", err) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(resp); h.handleWriteError(span, log, "UserHandler.Users", "write_response", "failed to write response", err) {
		return
	}

	log.Info("Users retrieved successfully",
		zap.String("operation", "UserHandler.Users"),
		zap.String("step", "users_retrieved"),
		zap.Int("user_count", len(users)),
	)

	metrics.IncUserOperation("list_users", "success")
	span.SetStatus(codes.Ok, "users retrieved successfully")
}

func (h *UserHandler) User(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "UsersHandler.User")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)

	idRaw := r.PathValue("id")
	id, err := strconv.Atoi(idRaw)

	if err != nil || id < 1 {
		span.SetAttributes(attribute.String("user.id.raw", idRaw))
		if h.handleBadRequest(w, span, log, "UserHandler.User", "parse_id", "Invalid ID format", err,
			zap.String("user_id_raw", idRaw),
		) {
			return
		}
	}

	user, err := h.UserRepository.User(ctx, id)
	if h.handleRepositoryError(w, span, log, "UserHandler.User", "fetch_user", "Failed to get user", err,
		zap.Int("user_id", id),
	) {
		span.SetAttributes(attribute.Int("user.id", id))
		return
	}

	w.Header().Set("Content-Type", "application/json")

	span.SetAttributes(attribute.Int("user.id", id))
	if err := json.NewEncoder(w).Encode(user); h.handleWriteError(span, log, "UserHandler.User", "encode_response", "failed to encode response", err,
		zap.Int("user_id", id),
	) {
		return
	}

	log.Info("User retrieved successfully",
		zap.Int("user_id", id),
		zap.String("operation", "UserHandler.User"),
		zap.String("step", "user_retrieved"),
	)

	metrics.IncUserOperation("get_user", "success")
	span.SetStatus(codes.Ok, "user retrieved successfully")
}

func (h *UserHandler) Exists(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "UsersHandler.Exists")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)

	idRaw := r.PathValue("id")
	id, err := strconv.Atoi(idRaw)
	if err != nil || id < 1 {
		span.SetStatus(codes.Error, "invalid ID format in request")
		span.SetAttributes(attribute.String("user.id.raw", idRaw))
		if h.handleBadRequest(w, span, log, "UserHandler.Exists", "parse_id", "Invalid ID format", err,
			zap.String("user_id_raw", idRaw),
		) {
			return
		}
	}

	exists, err := h.UserRepository.Exists(ctx, id)
	if h.handleRepositoryError(w, span, log, "UserHandler.Exists", "check_existence", "Failed to check user existence", err,
		zap.Int("user_id", id),
	) {
		span.SetAttributes(attribute.Int("user.id", id))
		return
	}

	if !exists {
		log.Info("User does not exist",
			zap.Int("user_id", id),
			zap.String("operation", "UserHandler.Exists"),
			zap.String("step", "user_not_found"),
		)

		metrics.IncUserOperation("check_user_exists", "not_found")
		span.SetStatus(codes.Ok, "user does not exist")
		span.SetAttributes(attribute.Int("user.id", id))

		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if r.Method != "HEAD" {
		resp, err := json.Marshal(map[string]bool{"exists": true})
		if h.handleWriteError(span, log, "UserHandler.Exists", "encode_response", "failed to encode response", err,
			zap.Int("user_id", id),
		) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write(resp); h.handleWriteError(span, log, "UserHandler.Exists", "write_response", "failed to write response", err,
			zap.Int("user_id", id),
		) {
			return
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}

	log.Info("User existence checked successfully",
		zap.Int("user_id", id),
		zap.String("operation", "UserHandler.Exists"),
		zap.String("step", "existence_checked"),
	)

	metrics.IncUserOperation("check_user_exists", "success")
	span.SetStatus(codes.Ok, "user existence checked successfully")
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "UsersHandler.Create")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)

	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		if h.handleBadRequest(w, span, log, "UserHandler.Create", "decode_request", "Invalid request body", err) {
			return
		}
	}

	if err := h.UserRepository.Create(ctx, &user); h.handleRepositoryError(w, span, log, "UserHandler.Create", "insert_user", "Failed to create user", err) {
		return
	}

	resp, err := json.Marshal(map[string]any{"id": user.ID})
	span.SetAttributes(attribute.Int("user.id", int(user.ID)))
	if h.handleWriteError(span, log, "UserHandler.Create", "encode_response", "failed to encode response", err,
		zap.Int("user_id", int(user.ID)),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	span.SetAttributes(attribute.Int("user.id", int(user.ID)))
	if _, err := w.Write(resp); h.handleWriteError(span, log, "UserHandler.Create", "write_response", "failed to write response", err,
		zap.Int("user_id", int(user.ID)),
	) {
		return
	}

	log.Info("User created successfully",
		zap.Int("user_id", int(user.ID)),
		zap.String("operation", "UserHandler.Create"),
		zap.String("step", "user_created"),
	)

	metrics.IncUserOperation("create_user", "success")
	span.SetStatus(codes.Ok, "user created successfully")
	span.SetAttributes(attribute.Int("user.id", int(user.ID)))
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "UsersHandler.Update")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)
	idRaw := r.PathValue("id")

	id, err := strconv.Atoi(idRaw)
	if err != nil || id < 1 {
		span.SetStatus(codes.Error, "invalid ID format in request")
		span.SetAttributes(attribute.String("user.id.raw", idRaw))
		if h.handleBadRequest(w, span, log, "UserHandler.Update", "parse_id", "Invalid ID format", err,
			zap.String("user_id_raw", idRaw),
		) {
			return
		}
	}

	var newUser map[string]any
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		span.SetAttributes(attribute.Int("user.id", id))
		if h.handleBadRequest(w, span, log, "UserHandler.Update", "decode_request", "Invalid request body", err,
			zap.Int("user_id", id),
		) {
			return
		}
	}

	if err := h.UserRepository.Update(ctx, id, newUser); h.handleRepositoryError(w, span, log, "UserHandler.Update", "update_user", "Failed to update user", err,
		zap.Int("user_id", id),
	) {
		span.SetAttributes(attribute.Int("user.id", id))
		return
	}

	resp, err := json.Marshal(map[string]bool{"updated": true})
	span.SetAttributes(attribute.Int("user.id", id))
	if h.handleWriteError(span, log, "UserHandler.Update", "encode_response", "failed to encode response", err,
		zap.Int("user_id", id),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	span.SetAttributes(attribute.Int("user.id", id))
	if _, err := w.Write(resp); h.handleWriteError(span, log, "UserHandler.Update", "write_response", "failed to write response", err,
		zap.Int("user_id", id),
	) {
		return
	}

	log.Info("User updated successfully",
		zap.Int("user_id", id),
		zap.String("operation", "UserHandler.Update"),
		zap.String("step", "user_updated"),
	)

	metrics.IncUserOperation("update_user", "success")
	span.SetStatus(codes.Ok, "user updated successfully")
	span.SetAttributes(attribute.Int("user.id", id))

}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "UsersHandler.Delete")
	defer span.End()

	log := logger.WithTrace(ctx, h.log)

	idRaw := r.PathValue("id")
	id, err := strconv.Atoi(idRaw)
	if err != nil || id < 1 {
		span.SetStatus(codes.Error, "invalid ID format in request")
		span.SetAttributes(attribute.String("user.id.raw", idRaw))
		if h.handleBadRequest(w, span, log, "UserHandler.Delete", "parse_id", "Invalid ID format", err,
			zap.String("user_id_raw", idRaw),
		) {
			return
		}
	}

	if err := h.UserRepository.Delete(ctx, id); h.handleRepositoryError(w, span, log, "UserHandler.Delete", "delete_user", "Failed to delete user", err,
		zap.Int("user_id", id),
	) {
		span.SetAttributes(attribute.Int("user.id", id))
		return
	}

	resp, err := json.Marshal(map[string]bool{"deleted": true})
	span.SetAttributes(attribute.Int("user.id", id))
	if h.handleWriteError(span, log, "UserHandler.Delete", "encode_response", "failed to encode response", err,
		zap.Int("user_id", id),
	) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	span.SetAttributes(attribute.Int("user.id", id))
	if _, err := w.Write(resp); h.handleWriteError(span, log, "UserHandler.Delete", "write_response", "failed to write response", err,
		zap.Int("user_id", id),
	) {
		return
	}

	log.Info("User deleted successfully",
		zap.Int("user_id", id),
		zap.String("operation", "UserHandler.Delete"),
		zap.String("step", "user_deleted"),
	)

	metrics.IncUserOperation("delete_user", "success")
	span.SetStatus(codes.Ok, "user deleted successfully")
	span.SetAttributes(attribute.Int("user.id", id))
}

func (h *UserHandler) handleRepositoryError(
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

	baseFields := append(fields,
		zap.String("operation", operation),
		zap.String("step", step),
		zap.Error(err),
	)

	switch {
	case errors.Is(err, context.Canceled):
		metrics.IncUserOperation(step, "canceled")
		log.Warn("Request canceled", baseFields...)
		span.RecordError(err)
		span.SetStatus(codes.Error, "request canceled")
		return true

	case errors.Is(err, context.DeadlineExceeded):
		metrics.IncUserOperation(step, "timeout")
		log.Error("Request timeout", baseFields...)
		span.RecordError(err)
		span.SetStatus(codes.Error, "request timeout")
		http.Error(w, "Request timeout", http.StatusGatewayTimeout)
		return true

	case errors.Is(err, gorm.ErrRecordNotFound):
		metrics.IncUserOperation(step, "not_found")
		log.Warn("Resource not found", baseFields...)
		span.RecordError(err)
		span.SetStatus(codes.Error, "record not found")
		http.Error(w, "User not found", http.StatusNotFound)
		return true

	case errors.Is(err, repository.ErrUsernameAlreadyTaken):
		metrics.IncUserOperation(step, "conflict")
		log.Warn("Username already taken", baseFields...)
		span.RecordError(err)
		span.SetStatus(codes.Error, "username already taken")
		http.Error(w, "Username already taken", http.StatusConflict)
		return true

	default:
		metrics.IncUserOperation(step, "error")
		log.Error(message, baseFields...)
		span.RecordError(err)
		span.SetStatus(codes.Error, message)
		http.Error(w, message, http.StatusInternalServerError)
		return true
	}
}

func (h *UserHandler) handleBadRequest(
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

	log.Warn(message, append(fields,
		zap.String("operation", operation),
		zap.String("step", step),
		zap.Error(err),
	)...)

	metrics.IncUserOperation(step, "bad_request")
	span.RecordError(err)
	span.SetStatus(codes.Error, message)

	http.Error(w, message, http.StatusBadRequest)
	return true
}

func (h *UserHandler) handleWriteError(
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
		metrics.IncUserOperation(step, "canceled")
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

	metrics.IncUserOperation(step, "write_error")
	span.RecordError(err)
	span.SetStatus(codes.Error, message)
	return true
}
