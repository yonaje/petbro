package main

import (
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/yonaje/authservice/internal/clients"
	"github.com/yonaje/authservice/internal/database"
	"github.com/yonaje/authservice/internal/handlers"
	"github.com/yonaje/authservice/internal/repository"
	"github.com/yonaje/authservice/internal/routes"
)

func main() {
	var (
		host = os.Getenv("DB_HOST")
		user = os.Getenv("DB_USER")
		pass = os.Getenv("DB_PASSWORD")
		name = os.Getenv("DB_NAME")
	)

	db := database.Connect(host, user, pass, name)
	authRepository := repository.NewAuthRepository(db)
	userClient := clients.NewUserClient(os.Getenv("USER_SERVICE_URL"))
	authHandler := handlers.NewAuthHandler(authRepository, userClient)

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux, authHandler)

	addr := ":8080"
	if err := http.ListenAndServe(addr, mux); err != nil {
		panic("failed to start server" + err.Error())
	}

}
