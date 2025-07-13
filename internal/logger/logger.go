package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	instance *Logger
	once     sync.Once
)

// Logger provides TUI-safe logging functionality
type Logger struct {
	fileLogger *log.Logger
	logFile    *os.File
	mu         sync.Mutex
}

// Init initializes the global logger instance
func Init() error {
	var err error
	once.Do(func() {
		instance, err = newLogger()
	})
	return err
}

// newLogger creates a new logger instance
func newLogger() (*Logger, error) {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Open log file
	logPath := filepath.Join(logsDir, "reapo.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create file logger
	fileLogger := log.New(logFile, "", log.LstdFlags|log.Lshortfile)

	return &Logger{
		fileLogger: fileLogger,
		logFile:    logFile,
	}, nil
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	if instance != nil {
		instance.log("INFO", format, args...)
	}
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	if instance != nil {
		instance.log("ERROR", format, args...)
	}
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	if instance != nil {
		instance.log("DEBUG", format, args...)
	}
}

// Tool logs a tool execution message
func Tool(name string, input string) {
	if instance != nil {
		instance.log("TOOL", "%s(%s)", name, input)
	}
}

// log writes a formatted message to the log file
func (l *Logger) log(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	message := fmt.Sprintf(format, args...)
	l.fileLogger.Printf("[%s] %s", level, message)
}

// Close closes the log file
func Close() error {
	if instance != nil && instance.logFile != nil {
		return instance.logFile.Close()
	}
	return nil
}

// SetOutput allows changing the output destination (useful for testing)
func SetOutput(w io.Writer) {
	if instance != nil {
		instance.mu.Lock()
		defer instance.mu.Unlock()
		instance.fileLogger.SetOutput(w)
	}
}