package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
}

// New creates a new logger
func New() *Logger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	
	logger, _ := config.Build()
	return &Logger{Logger: logger}
}

// NoOp returns a no-op logger
func NoOp() *Logger {
	return &Logger{Logger: zap.NewNop()}
}

// NoLog is a no-op logger instance
var NoLog = Logger{Logger: zap.NewNop()}

// NewLogger creates a new logger with a name
func NewLogger(name string) *Logger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	
	logger, _ := config.Build()
	return &Logger{Logger: logger.Named(name)}
}

// With creates a child logger with the given fields
func (l *Logger) With(fields ...zapcore.Field) *Logger {
	return &Logger{Logger: l.Logger.With(fields...)}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...zapcore.Field) {
	l.Logger.Debug(msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...zapcore.Field) {
	l.Logger.Info(msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...zapcore.Field) {
	l.Logger.Warn(msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...zapcore.Field) {
	l.Logger.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...zapcore.Field) {
	l.Logger.Fatal(msg, fields...)
}