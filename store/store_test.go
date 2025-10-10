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

	// Set the fileName to the temporary file
	fileName = tmpfile.Name()

	// Create a logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	// Test saving a message
	userID := "testuser"
	message := "hello test"
	err = SaveMessage(ctx, logger, userID, message)
	if err != nil {
		t.Fatalf("SaveMessage failed: %v", err)
	}

	// Test getting messages
	messages, err := GetLast10Messages(ctx, logger)
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
