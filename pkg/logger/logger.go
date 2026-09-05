package logger

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Config holds configuration for the logger
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// New creates a new *slog.Logger based on the given configuration.
func New(out io.Writer, cfg Config) (*slog.Logger, error) {
	var level slog.Level

	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info", "": // default to info if empty
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log level: %s", cfg.Level)
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(out, opts)
	case "text", "": // default to text if empty
		handler = slog.NewTextHandler(out, opts)
	default:
		return nil, fmt.Errorf("invalid log format: %s", cfg.Format)
	}

	return slog.New(handler), nil
}

// InitSetsDefault creates a new logger based on the configuration and sets it as the default slog logger.
func InitSetsDefault(out io.Writer, cfg Config) error {
	logger, err := New(out, cfg)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	return nil
}
