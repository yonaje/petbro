package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	if err := jwt.SetSigningKey(getEnv("JWT_SECRET")); err != nil {
		log.Fatal("Failed to initialize jwt signing key",
			zap.String("operation", "main"),
			zap.String("step", "init_jwt"),
			zap.Error(err),
		)
	}

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

	tracer := otel.Tracer("authservice/main")
	ctx, span := tracer.Start(ctx, "startup")
	defer span.End()

	logger.WithTrace(ctx, log).Info("service started",
		zap.String("operation", "main"),
		zap.String("step", "startup"),
	)

	db, err := database.Connect(
		logger.WithTrace(ctx, log),
		getEnv("POSTGRES_HOST"),
		getEnv("POSTGRES_USER"),
		getEnv("POSTGRES_PASSWORD"),
		getEnv("POSTGRES_DB"),
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

	userClient := clients.NewUserClient(getEnv("USER_SERVICE_BASE_URL"))

	authHandler := handlers.NewAuthHandler(authRepository, userClient, log)

	mux := http.NewServeMux()
	routes.RegisterRoutes(mux, authHandler)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	var handler http.Handler = mux
	handler = otelhttp.NewHandler(handler, "http-server")
	handler = middleware.Logging(log, handler)
	handler = middleware.CORS(handler)

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
