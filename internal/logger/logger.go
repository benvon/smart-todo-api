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

	// Enable stack traces for error level and above
	// When DisableStacktrace is false, zap automatically includes stack traces
	// for error level and above logs
	config.DisableStacktrace = false

	return config.Build()
}

// Sync flushes any buffered log entries. This should be called before application exit.
// It's safe to call Sync() multiple times.
func Sync(logger *zap.Logger) error {
	if logger == nil {
		return nil
	}
	return logger.Sync()
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
