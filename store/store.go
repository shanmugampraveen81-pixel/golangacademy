package store

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

var fileName = "messages.txt"

// SaveMessage appends a message to the file
func SaveMessage(ctx context.Context, logger *slog.Logger, userID, message string) error {
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("failed to open file", "error", err)
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	timestamp := time.Now().Format(time.RFC3339)
	logMessage := fmt.Sprintf("{\"timestamp\": \"%s\", \"userID\": \"%s\", \"message\": \"%s\"}\n", timestamp, userID, message)

	_, err = file.WriteString(logMessage)
	if err != nil {
		logger.Error("failed to write to file", "error", err)
		return fmt.Errorf("failed to write to file: %w", err)
	}

	logger.Info("message saved to disk")
	return nil
}

// GetLast10Messages retrieves the last 10 messages from the file
func GetLast10Messages(ctx context.Context, logger *slog.Logger) ([]string, error) {
	file, err := os.Open(fileName)
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
