package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/yonaje/userservice/internal/database"
	"github.com/yonaje/userservice/internal/handlers"
	"github.com/yonaje/userservice/internal/logger"
	"github.com/yonaje/userservice/internal/middleware"
	"github.com/yonaje/userservice/internal/repository"
	"github.com/yonaje/userservice/internal/routes"
	"github.com/yonaje/userservice/internal/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// initialize context
	ctx := context.Background()

	// initialize logger
	log := logger.Must(logger.New(logger.Config{
		Service: getEnv("SERVICE"),
		Env:     getEnv("LOG_ENV"),
		Version: getEnv("APP_VERSION"),
		Level:   getEnv("LOG_LEVEL"),
		Format:  getEnv("LOG_FORMAT"),
	}))

	defer logger.Sync(log)

	// initialize tracing
	shutdown, err := tracing.Init(ctx, tracing.Config{
		Service: getEnv("SERVICE"),
		Env:     getEnv("LOG_ENV"),
		Version: getEnv("APP_VERSION"),
	})

	// If tracing initialization fails, log the error and exit the application
	if err != nil {
		log.Fatal("Failed to initialize tracing: " + err.Error())
	}

	defer shutdown(ctx)

	tracer := otel.Tracer("userservice/main")
	ctx, span := tracer.Start(ctx, "startup")
	defer span.End()

	logger.WithTrace(ctx, log).Info("service started")

	// connect to database
	db, err := database.Connect(
		log,
		getEnv("DB_HOST"),
		getEnv("DB_USER"),
		getEnv("DB_PASSWORD"),
		getEnv("DB_NAME"),
	)

	if err != nil {
		log.Fatal("Failed to connect to database: " + err.Error())
	}

	// initialize repository
	userRepository := repository.NewUserRepository(db)

	// initialize handler
	userHandler := handlers.NewUserHandler(userRepository, log)

	// initialize router
	mux := http.NewServeMux()

	// register routes
	routes.RegisterRoutes(mux, userHandler)

	// wrap with middleware
	var handler http.Handler = mux
	handler = otelhttp.NewHandler(mux, "http-server") // add tracing middleware
	handler = middleware.Logging(log, handler)        // add logging middleware

	// start server
	addr := getEnv("APP_PORT")

	if err := http.ListenAndServe(":"+addr, handler); err != nil {
		log.Fatal("Failed to start server: ",
			zap.String("operation", "main"),
			zap.String("step", "start_server"),
			zap.String("address", addr),
			zap.Error(err),
		)
	}
}

// getEnv retrieves the value of the environment variable named by the key.
// If the variable is not present, it logs a fatal error and exits the application.
func getEnv(key string) string {
	val := os.Getenv(key)

	if val == "" {
		log.Fatal("Environment variable not set",
			zap.String("operation", "main"),
			zap.String("step", "getEnv"),
			zap.String("key", key),
		)
	}

	return val
}
