package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type FileStore struct {
	FileName string
	mu       sync.Mutex
}

var defaultStore = &FileStore{FileName: "messages.txt"}

// SaveMessage appends a message to the file with locking and safe JSON
func SaveMessage(ctx context.Context, logger *slog.Logger, userID, message string) error {
	return defaultStore.SaveMessage(ctx, logger, userID, message)
}

func (s *FileStore) SaveMessage(ctx context.Context, logger *slog.Logger, userID, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(s.FileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("failed to open file", "error", err)
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	timestamp := time.Now().Format(time.RFC3339)

	logEntry := map[string]string{
		"timestamp": timestamp,
		"userID":    userID,
		"message":   message,
	}
	logMessage, err := json.Marshal(logEntry)
	if err != nil {
		logger.Error("failed to marshal log entry", "error", err)
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	_, err = file.WriteString(string(logMessage) + "\n")
	if err != nil {
		logger.Error("failed to write to file", "error", err)
		return fmt.Errorf("failed to write to file: %w", err)
	}

	logger.Info("message saved to disk")
	return nil
}

// GetLast10Messages retrieves the last 10 messages from the file with locking
func GetLast10Messages(ctx context.Context, logger *slog.Logger) ([]string, error) {
	return defaultStore.GetLast10Messages(ctx, logger)
}

func (s *FileStore) GetLast10Messages(ctx context.Context, logger *slog.Logger) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.FileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No messages yet, not an error
		}
		logger.Error("failed to open file", "error", err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		logger.Error("failed to read file", "error", err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Get the last 10 lines
	start := 0
	if len(lines) > 10 {
		start = len(lines) - 10
	}

	return lines[start:], nil
}
