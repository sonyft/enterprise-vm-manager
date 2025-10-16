package logger

import (
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps zap logger with additional functionality
type Logger struct {
	*zap.SugaredLogger
	atom zap.AtomicLevel
}

// Config represents logger configuration
type Config struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	Filename   string `json:"filename"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
}

// New creates a new structured logger
func New(cfg Config) (*Logger, error) {
	// Parse log level
	atom := zap.NewAtomicLevel()
	if err := atom.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("invalid log level %s: %w", cfg.Level, err)
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if cfg.Format == "console" {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// Create encoder
	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Configure output
	var writer zapcore.WriteSyncer
	switch cfg.Output {
	case "stdout":
		writer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writer = zapcore.AddSync(os.Stderr)
	case "file":
		if cfg.Filename == "" {
			return nil, fmt.Errorf("filename is required when output is file")
		}
		writer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		})
	case "both":
		if cfg.Filename == "" {
			return nil, fmt.Errorf("filename is required when output is both")
		}
		writer = zapcore.AddSync(io.MultiWriter(
			os.Stdout,
			&lumberjack.Logger{
				Filename:   cfg.Filename,
				MaxSize:    cfg.MaxSize,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAge,
				Compress:   cfg.Compress,
			},
		))
	default:
		writer = zapcore.AddSync(os.Stdout)
	}

	// Create core and logger
	core := zapcore.NewCore(encoder, writer, atom)
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{
		SugaredLogger: zapLogger.Sugar(),
		atom:          atom,
	}, nil
}

// SetLevel dynamically sets the log level
func (l *Logger) SetLevel(level string) error {
	return l.atom.UnmarshalText([]byte(level))
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() string {
	return l.atom.Level().String()
}

// WithFields returns a logger with predefined fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	var pairs []interface{}
	for k, v := range fields {
		pairs = append(pairs, k, v)
	}
	return &Logger{
		SugaredLogger: l.SugaredLogger.With(pairs...),
		atom:          l.atom,
	}
}

// WithField returns a logger with a predefined field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.With(key, value),
		atom:          l.atom,
	}
}

// WithRequestID returns a logger with request ID
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.WithField("request_id", requestID)
}

// WithUserID returns a logger with user ID
func (l *Logger) WithUserID(userID string) *Logger {
	return l.WithField("user_id", userID)
}

// WithOperation returns a logger with operation name
func (l *Logger) WithOperation(operation string) *Logger {
	return l.WithField("operation", operation)
}

// WithComponent returns a logger with component name
func (l *Logger) WithComponent(component string) *Logger {
	return l.WithField("component", component)
}

// Close syncs the logger
func (l *Logger) Close() error {
	return l.SugaredLogger.Sync()
}

// Global logger instance
var globalLogger *Logger

// Init initializes the global logger
func Init(cfg Config) error {
	logger, err := New(cfg)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		// Fallback to development logger
		logger, _ := New(Config{
			Level:  "debug",
			Format: "console",
			Output: "stdout",
		})
		globalLogger = logger
	}
	return globalLogger
}

// SetGlobalLevel sets the global logger level
func SetGlobalLevel(level string) error {
	return Get().SetLevel(level)
}

// Close closes the global logger
func Close() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

// Convenience functions for global logger
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	Get().Debugf(template, args...)
}

func Info(args ...interface{}) {
	Get().Info(args...)
}

func Infof(template string, args ...interface{}) {
	Get().Infof(template, args...)
}

func Warn(args ...interface{}) {
	Get().Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	Get().Warnf(template, args...)
}

func Error(args ...interface{}) {
	Get().Error(args...)
}

func Errorf(template string, args ...interface{}) {
	Get().Errorf(template, args...)
}

func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	Get().Fatalf(template, args...)
}

func Panic(args ...interface{}) {
	Get().Panic(args...)
}

func Panicf(template string, args ...interface{}) {
	Get().Panicf(template, args...)
}
