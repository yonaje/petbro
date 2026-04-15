package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yonaje/friendservice/internal/clients"
	"github.com/yonaje/friendservice/internal/database"
	"github.com/yonaje/friendservice/internal/handlers"
	"github.com/yonaje/friendservice/internal/logger"
	"github.com/yonaje/friendservice/internal/middleware"
	"github.com/yonaje/friendservice/internal/repository"
	"github.com/yonaje/friendservice/internal/routes"
	"github.com/yonaje/friendservice/internal/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	log := logger.Must(logger.New(logger.Config{
		Service:      getEnv("SERVICE_NAME"),
		Env:          getEnv("APP_ENV"),
		Version:      getEnv("APP_VERSION"),
		Level:        getEnv("LOG_LEVEL"),
		Format:       getEnv("LOG_FORMAT"),
		Output:       getEnv("LOG_OUTPUT"),
		FilePath:     getEnv("LOG_FILE_PATH"),
		ErrorPath:    getEnv("LOG_ERROR_FILE_PATH"),
		MaxSizeMB:    getEnvInt("LOG_MAX_SIZE_MB", 20),
		MaxBackups:   getEnvInt("LOG_MAX_BACKUPS", 10),
		MaxAgeDays:   getEnvInt("LOG_MAX_AGE_DAYS", 30),
		CompressFile: getEnvBool("LOG_COMPRESS", true),
	}))
	defer logger.Sync(log)

	shutdown, err := tracing.Init(ctx, tracing.Config{
		Service: getEnv("SERVICE_NAME"),
		Env:     getEnv("APP_ENV"),
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

	tracer := otel.Tracer("friendservice/main")
	ctx, span := tracer.Start(ctx, "startup")
	defer span.End()

	logger.WithTrace(ctx, log).Info("service started",
		zap.String("operation", "main"),
		zap.String("step", "startup"),
	)

	driver, err := database.Connect(
		logger.WithTrace(ctx, log),
		getEnv("NEO4J_URI"),
		getEnv("NEO4J_USER"),
		getEnv("NEO4J_PASSWORD"),
	)
	if err != nil {
		log.Fatal("Failed to connect to database",
			zap.String("operation", "main"),
			zap.String("step", "database_connection"),
			zap.Error(err),
		)
	}
	defer driver.Close(context.Background())

	friendRepository := repository.NewFriendRepository(driver)
	userClient := clients.NewUserClient(getEnv("USER_SERVICE_BASE_URL"))
	friendHandler := handlers.NewFriendHandler(friendRepository, userClient, log)
	authMiddleware, err := middleware.NewAuthMiddleware(getEnv("JWT_SECRET"), log)
	if err != nil {
		log.Fatal("Failed to initialize auth middleware",
			zap.String("operation", "main"),
			zap.String("step", "init_auth_middleware"),
			zap.Error(err),
		)
	}

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux, friendHandler, authMiddleware)
	mux.Handle("GET /metrics", promhttp.Handler())

	var handler http.Handler = mux
	handler = otelhttp.NewHandler(handler, "http-server")
	handler = middleware.Logging(log, handler)

	addr := ":" + getEnv("SERVICE_PORT")
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

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Fatal("Invalid integer environment variable",
			zap.String("operation", "main"),
			zap.String("step", "getEnvInt"),
			zap.String("key", key),
			zap.String("value", val),
			zap.Error(err),
		)
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		log.Fatal("Invalid boolean environment variable",
			zap.String("operation", "main"),
			zap.String("step", "getEnvBool"),
			zap.String("key", key),
			zap.String("value", val),
			zap.Error(err),
		)
	}

	return parsed
}
