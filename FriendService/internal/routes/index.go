package routes

import (
	"net/http"

	"github.com/yonaje/friendservice/internal/handlers"
)

func RegisterRoutes(mux *http.ServeMux, handler *handlers.FriendHandler) {
	FriendRoutes(mux, handler)
}
