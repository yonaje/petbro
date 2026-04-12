package routes

import (
	"net/http"

	"github.com/yonaje/userservice/internal/handlers"
)

func RegisterRoutes(mux *http.ServeMux, handler *handlers.UserHandler) {
	UserRoutes(mux, handler)
}
