package routes

import (
	"net/http"

	"github.com/yonaje/userservice/internal/handlers"
)

func UserRoutes(mux *http.ServeMux, handler *handlers.UserHandler) {

	// public routes
	mux.HandleFunc("GET /users", handler.Users)         // get all users with pagination and sorting
	mux.HandleFunc("GET /user/{id}", handler.User)      // get user by id
	mux.HandleFunc("PUT /user/{id}", handler.Update)    // update user by id
	mux.HandleFunc("POST /user", handler.Create)        // create new user
	mux.HandleFunc("DELETE /user/{id}", handler.Delete) // delete user by id

	// internal exists
	mux.HandleFunc("GET /internal/user/{id}", handler.Exists) // check if user exists by id
	mux.HandleFunc("POST /internal/user", handler.Create)     // create new user (internal)
}
