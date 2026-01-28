package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewProductionLogger creates a production-ready logger with JSON encoding
func NewProductionLogger(debugMode bool) (*zap.Logger, error) {
	config := zap.NewProductionConfig()

	// Set log level based on debug mode
	if debugMode {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// Configure encoder for JSON output
	config.Encoding = "json"
	config.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Disable stack traces unless error level
	config.DisableStacktrace = true

	return config.Build()
}

// NewDevelopmentLogger creates a development logger with console encoding (for local dev)
func NewDevelopmentLogger(debugMode bool) (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()

	if debugMode {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	return config.Build()
}
