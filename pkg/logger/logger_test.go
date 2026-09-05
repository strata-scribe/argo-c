package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name          string
		cfg           Config
		expectErr     bool
		logLevelCheck slog.Level
		logFormatJSON bool
	}{
		{
			name: "default values",
			cfg: Config{
				Level:  "",
				Format: "",
			},
			expectErr:     false,
			logLevelCheck: slog.LevelInfo,
			logFormatJSON: false,
		},
		{
			name: "json debug",
			cfg: Config{
				Level:  "debug",
				Format: "json",
			},
			expectErr:     false,
			logLevelCheck: slog.LevelDebug,
			logFormatJSON: true,
		},
		{
			name: "text warn",
			cfg: Config{
				Level:  "warn",
				Format: "text",
			},
			expectErr:     false,
			logLevelCheck: slog.LevelWarn,
			logFormatJSON: false,
		},
		{
			name: "json error",
			cfg: Config{
				Level:  "error",
				Format: "json",
			},
			expectErr:     false,
			logLevelCheck: slog.LevelError,
			logFormatJSON: true,
		},
		{
			name: "invalid level",
			cfg: Config{
				Level:  "invalid",
				Format: "text",
			},
			expectErr: true,
		},
		{
			name: "invalid format",
			cfg: Config{
				Level:  "info",
				Format: "invalid",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger, err := New(&buf, tt.cfg)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if logger == nil {
				t.Fatalf("Logger is nil")
			}

			// Verify level by checking if enabled
			if logger.Enabled(context.Background(), tt.logLevelCheck) == false {
				t.Errorf("Expected level %s to be enabled", tt.logLevelCheck)
			}
			// Verify level by checking a lower level is not enabled (if not debug)
			if tt.logLevelCheck > slog.LevelDebug {
				if logger.Enabled(context.Background(), tt.logLevelCheck-1) {
					t.Errorf("Expected level %s to be disabled", tt.logLevelCheck-1)
				}
			}

			// Verify format by logging a message and parsing the output
			msg := "test message"
			logger.Log(context.Background(), tt.logLevelCheck, msg)
			output := buf.String()

			if tt.logFormatJSON {
				// Parse JSON to verify format
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(output), &data); err != nil {
					t.Errorf("Expected JSON output, could not parse: %v. Output was: %s", err, output)
				}
				if data["msg"] != msg {
					t.Errorf("Expected msg %s in JSON output, got %v", msg, data["msg"])
				}
			} else {
				// Simple check for text format
				if !strings.Contains(output, msg) {
					t.Errorf("Expected text output to contain %s, output was: %s", msg, output)
				}
				if strings.HasPrefix(output, "{") {
					t.Errorf("Expected text output, got something that looks like JSON: %s", output)
				}
			}
		})
	}
}

func TestInitSetsDefault(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Level:  "info",
		Format: "text",
	}

	err := InitSetsDefault(&buf, cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify that the default logger has been changed
	msg := "default logger test message"
	slog.Info(msg)
	output := buf.String()

	if !strings.Contains(output, msg) {
		t.Errorf("Expected default logger output to contain %s, output was: %s", msg, output)
	}
}
