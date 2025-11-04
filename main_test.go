package main

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"log/slog"
	"os"
	"unified-go-prog/proto"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// MockStoreClient is a mock implementation of the proto.StoreClient for testing.
// It allows faking the behavior of the gRPC client without needing a running gRPC server.
type MockStoreClient struct {
	messages []string
	mu       sync.Mutex
}

// Save is the mock implementation of the Save method (non-streaming).
func (m *MockStoreClient) Save(ctx context.Context, in *proto.SaveRequest, opts ...grpc.CallOption) (*proto.SaveResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, in.Message)
	return &proto.SaveResponse{}, nil
}

// GetLast10 is the mock implementation of the GetLast10 method (non-streaming).
func (m *MockStoreClient) GetLast10(ctx context.Context, in *proto.GetLast10Request, opts ...grpc.CallOption) (*proto.GetLast10Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &proto.GetLast10Response{Messages: m.messages}, nil
}

// (Removed streaming mock types; not needed for non-streaming interface)

func TestConcurrentWebsocket(t *testing.T) {
	t.Parallel()

	// Set up a mock gRPC client
	mockClient := &MockStoreClient{}

	// Set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Set up hub
	hub := newHub(logger)
	go hub.run()

	// Set up a test server
	server := httptest.NewServer(wsHandler(logger, mockClient, hub))
	defer server.Close()

	// Convert the server's URL to a WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Number of concurrent clients
	numClients := 10

	// Use a WaitGroup to wait for all clients to finish
	var wg sync.WaitGroup
	wg.Add(numClients)

	// Launch concurrent clients
	for i := 0; i < numClients; i++ {
		go func(i int) {
			defer wg.Done()

			// Connect to the WebSocket server
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Errorf("Client %d: failed to connect: %v", i, err)
				return
			}
			defer conn.Close()

			// Send a message
			msg := []byte("hello from client " + strconv.Itoa(i))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				t.Errorf("Client %d: failed to write message: %v", i, err)
				return
			}

			// Read messages
			for j := 0; j < numClients; j++ {
				_, _, err := conn.ReadMessage()
				if err != nil {
					t.Errorf("Client %d: failed to read message: %v", i, err)
					return
				}
			}
		}(i)
	}

	// Wait for all clients to finish
	wg.Wait()
}
