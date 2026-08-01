package main

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()

	// Use UTC timestamps.
	cfg.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format(time.RFC3339))
	}

	// Add the service name.
	cfg.InitialFields = map[string]interface{}{
		"service": "gproxy",
	}

	return cfg.Build()
}
