package main

import (
	"context"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"log/slog"
	"os"
	"unified-go-prog/proto"
)

// MockStoreClient is a mock implementation of the proto.StoreClient for testing.
// It allows faking the behavior of the gRPC client without needing a running gRPC server.
type MockStoreClient struct {
	// Add any fields needed for your mock's behavior, e.g., a slice of messages.
	messages []string
	mu       sync.Mutex
}

// Save is the mock implementation of the Save method.
// It appends the message to the mock's internal slice.
func (m *MockStoreClient) Save(ctx context.Context, opts ...grpc.CallOption) (proto.Store_SaveClient, error) {
	// You might need a more sophisticated mock for streaming RPCs.
	// For this example, we'll assume a simple success case.
	return &mockSaveClient{mock: m}, nil
}

// GetLast10 is the mock implementation of the GetLast10 method.
// It returns the last 10 messages from the mock's internal slice.
func (m *MockStoreClient) GetLast10(ctx context.Context, in *proto.GetLast10Request, opts ...grpc.CallOption) (proto.Store_GetLast10Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// You might need a more sophisticated mock for streaming RPCs.
	// For this example, we'll assume a simple success case.
	return &mockGetLast10Client{messages: m.messages}, nil
}

// Helper mock types for the streaming RPCs

type mockSaveClient struct {
	grpc.ClientStream
	mock *MockStoreClient
}

func (m *mockSaveClient) Send(req *proto.SaveRequest) error {
	m.mock.mu.Lock()
	defer m.mock.mu.Unlock()
	m.mock.messages = append(m.mock.messages, req.Message)
	return nil
}

func (m *mockSaveClient) CloseAndRecv() (*proto.SaveResponse, error) {
	return &proto.SaveResponse{}, nil
}

type mockGetLast10Client struct {
	grpc.ClientStream
	messages []string
	sent     bool
}

func (m *mockGetLast10Client) Recv() (*proto.GetLast10Response, error) {
	if m.sent {
		return nil, io.EOF
	}
	m.sent = true
	return &proto.GetLast10Response{Messages: m.messages}, nil
}

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
