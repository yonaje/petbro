package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

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

	// initialize tracing
	shutdown, err := tracing.Init(ctx, tracing.Config{
		Service: getEnv("SERVICE_NAME"),
		Env:     getEnv("APP_ENV"),
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
		getEnv("POSTGRES_HOST"),
		getEnv("POSTGRES_USER"),
		getEnv("POSTGRES_PASSWORD"),
		getEnv("POSTGRES_DB"),
	)

	if err != nil {
		log.Fatal("Failed to connect to database: " + err.Error())
	}

	// initialize repository
	userRepository := repository.NewUserRepository(db)

	// initialize handler
	userHandler := handlers.NewUserHandler(userRepository, log)

	authMiddleware, err := middleware.NewAuthMiddleware(getEnv("JWT_SECRET"), log)
	if err != nil {
		log.Fatal("Failed to initialize auth middleware",
			zap.String("operation", "main"),
			zap.String("step", "init_auth_middleware"),
			zap.Error(err),
		)
	}

	// initialize router
	mux := http.NewServeMux()

	// register routes
	routes.RegisterRoutes(mux, userHandler, authMiddleware)

	// wrap with middleware
	var handler http.Handler = mux
	handler = otelhttp.NewHandler(mux, "http-server") // add tracing middleware
	handler = middleware.Logging(log, handler)        // add logging middleware

	// start server
	addr := getEnv("SERVICE_PORT")

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
