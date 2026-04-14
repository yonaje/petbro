package routes

import (
	"net/http"

	"github.com/yonaje/userservice/internal/handlers"
	"github.com/yonaje/userservice/internal/middleware"
)

func UserRoutes(mux *http.ServeMux, handler *handlers.UserHandler, authMiddleware *middleware.AuthMiddleware) {
	protected := func(next http.HandlerFunc) http.Handler {
		return authMiddleware.Protect(http.HandlerFunc(next))
	}

	// authenticated routes
	mux.Handle("GET /users", protected(handler.Users))         // get all users with pagination and sorting
	mux.Handle("GET /user/{id}", protected(handler.User))      // get user by id
	mux.Handle("PUT /user/{id}", protected(handler.Update))    // update user by id
	mux.Handle("POST /user", protected(handler.Create))        // create new user
	mux.Handle("DELETE /user/{id}", protected(handler.Delete)) // delete user by id

	// internal routes
	mux.HandleFunc("GET /internal/user/{id}", handler.Exists) // check if user exists by id
	mux.HandleFunc("POST /internal/user", handler.Create)     // create new user (internal)
	mux.HandleFunc("DELETE /internal/user/{id}", handler.Delete)
}
