package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(cfg Config) (*zap.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	zapCfg := buildZapConfig(cfg, level)
	l, err := zapCfg.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	l = l.With(
		zap.String("service", cfg.Service),
		zap.String("env", cfg.Env),
		zap.String("version", cfg.Version),
	)

	return l, nil
}

func parseLevel(lvl string) (zapcore.Level, error) {
	switch lvl {
	case "", "info":
		return zapcore.InfoLevel, nil
	case "debug":
		return zapcore.DebugLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", lvl)
	}
}

func buildZapConfig(cfg Config, level zapcore.Level) zap.Config {
	var zapCfg zap.Config

	switch cfg.Format {
	case "console":
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.Encoding = "console"
	default:
		zapCfg = zap.NewProductionConfig()
		zapCfg.Encoding = "json"
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.OutputPaths = []string{"stdout"}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	zapCfg.Sampling = nil

	zapCfg.EncoderConfig.TimeKey = "timestamp"
	zapCfg.EncoderConfig.LevelKey = "level"
	zapCfg.EncoderConfig.MessageKey = "message"
	zapCfg.EncoderConfig.CallerKey = "caller"
	zapCfg.EncoderConfig.StacktraceKey = "stacktrace"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.EncoderConfig.EncodeDuration = zapcore.MillisDurationEncoder

	return zapCfg
}

func Sync(l *zap.Logger) {
	if l == nil {
		return
	}

	err := l.Sync()
	if err == nil {
		return
	}
}

func Must(l *zap.Logger, err error) *zap.Logger {
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger : %v\n", err)
		os.Exit(1)
	}

	return l
}
