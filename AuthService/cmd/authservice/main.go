package main

import (
	"context"
	"log"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/yonaje/authservice/internal/clients"
	"github.com/yonaje/authservice/internal/database"
	"github.com/yonaje/authservice/internal/handlers"
	"github.com/yonaje/authservice/internal/jwt"
	"github.com/yonaje/authservice/internal/logger"
	"github.com/yonaje/authservice/internal/middleware"
	"github.com/yonaje/authservice/internal/repository"
	"github.com/yonaje/authservice/internal/routes"
	"github.com/yonaje/authservice/internal/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	log := logger.Must(logger.New(logger.Config{
		Service: getEnv("SERVICE"),
		Env:     getEnv("LOG_ENV"),
		Version: getEnv("APP_VERSION"),
		Level:   getEnv("LOG_LEVEL"),
		Format:  getEnv("LOG_FORMAT"),
	}))

	defer logger.Sync(log)

	if err := jwt.SetSigningKey(getEnv("JWT_SECRET")); err != nil {
		log.Fatal("Failed to initialize jwt signing key",
			zap.String("operation", "main"),
			zap.String("step", "init_jwt"),
			zap.Error(err),
		)
	}

	shutdown, err := tracing.Init(ctx, tracing.Config{
		Service: getEnv("SERVICE"),
		Env:     getEnv("LOG_ENV"),
		Version: getEnv("APP_VERSION"),
	})
	if err != nil {
		log.Fatal("Failed to initialize tracing",
			zap.String("operation", "main"),
			zap.String("step", "init_tracing"),
			zap.Error(err),
		)
	}

	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Error("Failed to shutdown tracing",
				zap.String("operation", "main"),
				zap.String("step", "shutdown_tracing"),
				zap.Error(err),
			)
		}
	}()

	tracer := otel.Tracer("authservice/main")
	ctx, span := tracer.Start(ctx, "startup")
	defer span.End()

	logger.WithTrace(ctx, log).Info("service started",
		zap.String("operation", "main"),
		zap.String("step", "startup"),
	)

	db, err := database.Connect(
		logger.WithTrace(ctx, log),
		getEnv("DB_HOST"),
		getEnv("DB_USER"),
		getEnv("DB_PASSWORD"),
		getEnv("DB_NAME"),
	)

	if err != nil {
		log.Fatal("Failed to connect to database",
			zap.String("operation", "main"),
			zap.String("step", "database connection"),
			zap.Error(err),
		)
		return
	}

	authRepository := repository.NewAuthRepository(db)

	userClient := clients.NewUserClient(getEnv("USER_SERVICE_URL"))

	authHandler := handlers.NewAuthHandler(authRepository, userClient, log)

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux, authHandler)

	var handler http.Handler = mux
	handler = otelhttp.NewHandler(handler, "http-server")
	handler = middleware.Logging(log, handler)

	addr := ":" + getEnv("APP_PORT")
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal("Failed to start server",
			zap.String("operation", "main"),
			zap.String("step", "start_server"),
			zap.String("address", addr),
			zap.Error(err),
		)
	}
}

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
