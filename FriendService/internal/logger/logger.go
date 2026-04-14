package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func New(cfg Config) (*zap.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	encoder := buildEncoder(cfg)
	core, err := buildCore(cfg, level, encoder)
	if err != nil {
		return nil, err
	}

	l := zap.New(
		core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

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

func buildEncoder(cfg Config) zapcore.Encoder {
	encoderCfg := zap.NewProductionEncoderConfig()
	if cfg.Format == "console" {
		encoderCfg = zap.NewDevelopmentEncoderConfig()
	}

	encoderCfg.TimeKey = "timestamp"
	encoderCfg.LevelKey = "level"
	encoderCfg.MessageKey = "message"
	encoderCfg.CallerKey = "caller"
	encoderCfg.StacktraceKey = "stacktrace"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder

	if cfg.Format == "console" {
		return zapcore.NewConsoleEncoder(encoderCfg)
	}

	return zapcore.NewJSONEncoder(encoderCfg)
}

func buildCore(cfg Config, level zapcore.Level, encoder zapcore.Encoder) (zapcore.Core, error) {
	if cfg.Output == "file" {
		infoWriter := zapcore.AddSync(newRollingFile(cfg, false))
		errorWriter := zapcore.AddSync(newRollingFile(cfg, true))

		infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= level && lvl < zapcore.ErrorLevel
		})
		errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= maxLevel(level, zapcore.ErrorLevel)
		})

		return zapcore.NewTee(
			zapcore.NewCore(encoder, infoWriter, infoLevel),
			zapcore.NewCore(encoder, errorWriter, errorLevel),
		), nil
	}

	return zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stdout),
		level,
	), nil
}

func newRollingFile(cfg Config, errorsOnly bool) *lumberjack.Logger {
	filename := cfg.FilePath
	if errorsOnly && cfg.ErrorPath != "" {
		filename = cfg.ErrorPath
	}

	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    defaultInt(cfg.MaxSizeMB, 20),
		MaxBackups: defaultInt(cfg.MaxBackups, 10),
		MaxAge:     defaultInt(cfg.MaxAgeDays, 30),
		Compress:   cfg.CompressFile,
	}
}

func maxLevel(a, b zapcore.Level) zapcore.Level {
	if a > b {
		return a
	}
	return b
}

func defaultInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
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
