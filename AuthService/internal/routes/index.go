package routes

import (
	"net/http"

	"github.com/yonaje/authservice/internal/handlers"
)

func RegisterRoutes(mux *http.ServeMux, handler *handlers.AuthHandler) {
	AuthRoutes(mux, handler)
}
