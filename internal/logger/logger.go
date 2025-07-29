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
	chatLogger *log.Logger
	logFile    *os.File
	chatFile   *os.File
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

	// Open main log file
	logPath := filepath.Join(logsDir, "reapo.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Open chat log file
	chatPath := filepath.Join(logsDir, "chat.log")
	chatFile, err := os.OpenFile(chatPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to open chat log file: %w", err)
	}

	// Create loggers
	fileLogger := log.New(logFile, "", log.LstdFlags|log.Lshortfile)
	chatLogger := log.New(chatFile, "", log.LstdFlags)

	return &Logger{
		fileLogger: fileLogger,
		chatLogger: chatLogger,
		logFile:    logFile,
		chatFile:   chatFile,
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

// Chat logs conversation data to the dedicated chat log file
func Chat(event string, data interface{}) {
	if instance != nil {
		instance.chatLog(event, data)
	}
}

// log writes a formatted message to the main log file
func (l *Logger) log(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	message := fmt.Sprintf(format, args...)
	l.fileLogger.Printf("[%s] %s", level, message)
}

// chatLog writes conversation data to the chat log file
func (l *Logger) chatLog(event string, data interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.chatLogger.Printf("[%s] %+v", event, data)
}

// Close closes both log files
func Close() error {
	if instance != nil {
		var err1, err2 error
		if instance.logFile != nil {
			err1 = instance.logFile.Close()
		}
		if instance.chatFile != nil {
			err2 = instance.chatFile.Close()
		}
		if err1 != nil {
			return err1
		}
		return err2
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
