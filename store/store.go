package store

import (
	"bufio"
	"bytes"
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

	// Efficiently read last 10 lines
	const maxLines = 10
	var lines []string
	stat, err := file.Stat()
	if err != nil {
		logger.Error("failed to stat file", "error", err)
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	var (
		size   = stat.Size()
		offset = int64(0)
		chunk  = int64(4096)
		buf    []byte
	)
	if size == 0 {
		return nil, nil
	}
	for {
		if size-chunk < 0 {
			chunk = size
			offset = 0
		} else {
			offset = size - chunk
		}
		tmp := make([]byte, chunk)
		_, err := file.ReadAt(tmp, offset)
		if err != nil {
			logger.Error("failed to read chunk", "error", err)
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}
		buf = append(tmp, buf...)
		// Count newlines
		count := 0
		for i := len(buf) - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				count++
				if count >= maxLines+1 {
					buf = buf[i+1:]
					break
				}
			}
		}
		if count >= maxLines+1 || offset == 0 {
			break
		}
		size = offset
	}
	// Split into lines
	scanner := bufio.NewScanner(bufio.NewReaderSize(bytes.NewReader(buf), len(buf)))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logger.Error("failed to scan lines", "error", err)
		return nil, fmt.Errorf("failed to scan lines: %w", err)
	}
	// Only return the last 10 lines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
