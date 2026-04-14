package routes

import (
	"net/http"

	"github.com/yonaje/userservice/internal/handlers"
	"github.com/yonaje/userservice/internal/middleware"
)

func RegisterRoutes(mux *http.ServeMux, handler *handlers.UserHandler, authMiddleware *middleware.AuthMiddleware) {
	UserRoutes(mux, handler, authMiddleware)
}
