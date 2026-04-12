package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/yonaje/friendservice/internal/clients"
	"github.com/yonaje/friendservice/internal/repository"
)

type FriendHandler struct {
	friendRepository repository.FriendRepository
	userClient       clients.UserClient
}

func NewFriendHandler(friendRepository repository.FriendRepository, userClient clients.UserClient) *FriendHandler {
	return &FriendHandler{
		friendRepository: friendRepository,
		userClient:       userClient,
	}
}

func (h *FriendHandler) SendRequest(w http.ResponseWriter, r *http.Request) {
	var request struct {
		FromUserID int `json:"fromUserId"`
		ToUserID   int `json:"toUserId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if ok := h.ensureUsersExist(w, r, request.FromUserID, request.ToUserID); !ok {
		return
	}

	err := h.friendRepository.SendRequest(r.Context(), request.FromUserID, request.ToUserID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "friend request sent",
	})
}

func (h *FriendHandler) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	var request struct {
		FromUserID int `json:"fromUserId"`
		ToUserID   int `json:"toUserId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if ok := h.ensureUsersExist(w, r, request.FromUserID, request.ToUserID); !ok {
		return
	}

	err := h.friendRepository.AcceptRequest(r.Context(), request.FromUserID, request.ToUserID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "friend request accepted",
	})
}

func (h *FriendHandler) Friends(w http.ResponseWriter, r *http.Request) {
	userID, err := parseIDFromPath(r.URL.Path, "/friends/")
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if ok := h.ensureUsersExist(w, r, userID); !ok {
		return
	}

	friends, err := h.friendRepository.ListFriends(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to get friends", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(friends)
}

func (h *FriendHandler) IncomingRequests(w http.ResponseWriter, r *http.Request) {
	userID, err := parseIDFromPath(r.URL.Path, "/friend-requests/incoming/")
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if ok := h.ensureUsersExist(w, r, userID); !ok {
		return
	}

	requests, err := h.friendRepository.ListIncomingRequests(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to get incoming requests", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(requests)
}

func (h *FriendHandler) Recommendations(w http.ResponseWriter, r *http.Request) {
	userID, err := parseIDFromPath(r.URL.Path, "/friend-recommendations/")
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if ok := h.ensureUsersExist(w, r, userID); !ok {
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = 10
	}

	recommendations, err := h.friendRepository.Recommendations(r.Context(), userID, limit)
	if err != nil {
		http.Error(w, "Failed to get recommendations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(recommendations)
}

func parseIDFromPath(path string, prefix string) (int, error) {
	id, err := strconv.Atoi(strings.TrimPrefix(path, prefix))
	if err != nil || id < 1 {
		return 0, err
	}

	return id, nil
}

func (h *FriendHandler) ensureUsersExist(w http.ResponseWriter, r *http.Request, userIDs ...int) bool {
	checked := make(map[int]struct{}, len(userIDs))

	for _, userID := range userIDs {
		if userID < 1 {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return false
		}

		if _, exists := checked[userID]; exists {
			continue
		}

		exists, err := h.userClient.UserExists(r.Context(), userID)
		if err != nil {
			http.Error(w, "Failed to validate user", http.StatusBadGateway)
			return false
		}

		if !exists {
			http.Error(w, "User not found", http.StatusNotFound)
			return false
		}

		checked[userID] = struct{}{}
	}

	return true
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrSelfRequest):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, repository.ErrAlreadyFriends),
		errors.Is(err, repository.ErrRequestAlreadyExists),
		errors.Is(err, repository.ErrIncomingRequestExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, repository.ErrRequestNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
