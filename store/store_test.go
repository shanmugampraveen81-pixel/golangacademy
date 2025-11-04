package store

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestSaveAndGetMessages(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test_messages.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// Use a dedicated FileStore for this test
	testStore := &FileStore{FileName: tmpfile.Name()}

	// Create a logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	// Test saving a message
	userID := "testuser"
	message := "hello test"
	err = testStore.SaveMessage(ctx, logger, userID, message)
	if err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}

	// Test getting messages
	messages, err := testStore.GetLast10Messages(ctx, logger)
	if err != nil {
		t.Fatalf("GetLast10Messages failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if !strings.Contains(messages[0], message) {
		t.Errorf("Expected message to contain '%s', got '%s'", message, messages[0])
	}
}

func TestGetLast10Messages_FileNotExist(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	ts := &FileStore{FileName: "nonexistent_file.txt"}
	msgs, err := ts.GetLast10Messages(ctx, logger)
	if err != nil {
		t.Fatalf("Expected no error for missing file, got: %v", err)
	}
	if msgs != nil && len(msgs) != 0 {
		t.Errorf("Expected no messages, got: %v", msgs)
	}
}

func TestGetLast10Messages_InvalidJSON(t *testing.T) {
	// Create a temp file with invalid JSON lines
	tmpfile, err := os.CreateTemp("", "test_invalid_json.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	_, _ = tmpfile.WriteString("not-json\nsecond-bad-line\n")
	tmpfile.Close()

	ts := &FileStore{FileName: tmpfile.Name()}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	msgs, err := ts.GetLast10Messages(ctx, logger)
	if err != nil {
		t.Fatalf("Expected no error for invalid JSON lines, got: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(msgs))
	}
}

func TestSaveMessage_FileError(t *testing.T) {
	// Use an invalid file path
	ts := &FileStore{FileName: "/invalid/path/forbidden.txt"}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	err := ts.SaveMessage(ctx, logger, "user", "msg")
	if err == nil {
		t.Error("Expected error for invalid file path, got nil")
	}
}
