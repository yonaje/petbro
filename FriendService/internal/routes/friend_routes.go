package routes

import (
	"net/http"

	"github.com/yonaje/friendservice/internal/handlers"
	"github.com/yonaje/friendservice/internal/middleware"
)

func FriendRoutes(mux *http.ServeMux, handler *handlers.FriendHandler, authMiddleware *middleware.AuthMiddleware) {
	protected := func(next http.HandlerFunc) http.Handler {
		return authMiddleware.Protect(http.HandlerFunc(next))
	}

	mux.Handle("/friend-request", protected(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler.SendRequest(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.Handle("/friend-request/accept", protected(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler.AcceptRequest(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.Handle("/friend-requests/incoming", protected(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.IncomingRequests(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.Handle("/friends", protected(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.Friends(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.Handle("/friend-recommendations", protected(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.Recommendations(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}
