package main

import (
	"context"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/yonaje/friendservice/internal/clients"
	"github.com/yonaje/friendservice/internal/database"
	"github.com/yonaje/friendservice/internal/handlers"
	"github.com/yonaje/friendservice/internal/repository"
	"github.com/yonaje/friendservice/internal/routes"
)

func main() {
	driver := database.Connect(
		os.Getenv("NEO4J_URI"),
		os.Getenv("NEO4J_USER"),
		os.Getenv("NEO4J_PASSWORD"),
	)
	defer driver.Close(context.Background())

	friendRepository := repository.NewFriendRepository(driver)
	userClient := clients.NewUserClient(os.Getenv("USER_SERVICE_URL"))
	friendHandler := handlers.NewFriendHandler(friendRepository, userClient)

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux, friendHandler)

	addr := ":8082"
	if err := http.ListenAndServe(addr, mux); err != nil {
		panic("failed to start server: " + err.Error())
	}
}
