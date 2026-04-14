package routes

import (
	"net/http"

	"github.com/yonaje/friendservice/internal/handlers"
	"github.com/yonaje/friendservice/internal/middleware"
)

func RegisterRoutes(mux *http.ServeMux, handler *handlers.FriendHandler, authMiddleware *middleware.AuthMiddleware) {
	FriendRoutes(mux, handler, authMiddleware)
}
