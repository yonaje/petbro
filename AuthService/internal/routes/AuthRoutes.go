package routes

import (
	"net/http"

	"github.com/yonaje/authservice/internal/handlers"
)

func AuthRoutes(mux *http.ServeMux, handler *handlers.AuthHandler) {
	mux.HandleFunc("/register", handler.Register)
	mux.HandleFunc("/login", handler.Login)
	mux.HandleFunc("/logout", handler.Logout)
	mux.HandleFunc("/refresh", handler.Refresh)
	mux.HandleFunc("/me", handler.Me)
}
