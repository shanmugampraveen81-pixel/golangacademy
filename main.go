package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"unified-go-prog/proto"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds application configuration
type Config struct {
	MessageFile    string
	RateLimitRPS   float64
	RateLimitBurst int
}

// LoadConfig loads config from environment variables or uses defaults
func LoadConfig() *Config {
	cfg := &Config{
		MessageFile:    getEnv("MESSAGE_FILE", "messages.txt"),
		RateLimitRPS:   getEnvFloat("RATE_LIMIT_RPS", 1.0),
		RateLimitBurst: getEnvInt("RATE_LIMIT_BURST", 5),
	}
	return cfg
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getEnvFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

// rateLimiterMap holds a map of IP addresses to rate limiters
var rateLimiterMap = struct {
	sync.Mutex
	m map[string]*rate.Limiter
}{m: make(map[string]*rate.Limiter)}

// rateLimiterConfig holds the global rate limiting settings
var rateLimiterConfig = struct {
	rps   float64
	burst int
}{rps: 1.0, burst: 5}

// getRateLimiter returns a rate limiter for the given IP address
func getRateLimiter(ip string) *rate.Limiter {
	rateLimiterMap.Lock()
	defer rateLimiterMap.Unlock()
	limiter, exists := rateLimiterMap.m[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(rateLimiterConfig.rps), rateLimiterConfig.burst) // Dynamic RPS and burst
		rateLimiterMap.m[ip] = limiter
	}
	return limiter
}

// rateLimitMiddleware enforces rate limiting per IP
func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if ipHeader := r.Header.Get("X-Real-IP"); ipHeader != "" {
			ip = ipHeader
		} else if ipHeader = r.Header.Get("X-Forwarded-For"); ipHeader != "" {
			ip = ipHeader
		}
		limiter := getRateLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type contextKey string

const (
	traceIDKey contextKey = "traceID"
	loggerKey  contextKey = "logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Only allow requests from the same host (adjust as needed)
		origin := r.Header.Get("Origin")
		allowed := "http://" + r.Host
		return origin == allowed || origin == "https://"+r.Host
	},
}

func main() {
	// Define flags
	msg := flag.String("message", "", "The message to be saved")
	user := flag.String("userID", "defaultUser", "The user ID")
	flag.Parse()

	// Set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Set up a connection to the gRPC server.
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := proto.NewStoreClient(conn)

	hub := newHub(logger)
	go hub.run()

	// If a message is provided via flags, run in CLI mode
	if *msg != "" {
		runCLI(logger, client, user, msg)
		return
	}

	// Otherwise, start the HTTP server
	runServer(logger, client, hub)
}

func runCLI(logger *slog.Logger, client proto.StoreClient, user, msg *string) {

	traceID := uuid.New().String()
	ctx := context.WithValue(context.Background(), traceIDKey, traceID)
	logger = logger.With("traceID", traceID)

	_, err := client.Save(ctx, &proto.SaveRequest{UserID: *user, Message: *msg})
	if err != nil {
		logger.Error("Failed to save message", "error", err)
		os.Exit(1)
	}
	fmt.Println("Message saved successfully.")

	resp, err := client.GetLast10(ctx, &proto.GetLast10Request{})
	if err != nil {
		logger.Error("Failed to get messages", "error", err)
		os.Exit(1)
	}

	fmt.Println("\nLast 10 messages:")
	for _, message := range resp.Messages {
		fmt.Println(message)
	}
}

func runServer(logger *slog.Logger, client proto.StoreClient, hub *Hub) {

	mux := http.NewServeMux()
	mux.Handle("/message", rateLimitMiddleware(traceIDMiddleware(logger, messageHandler(logger, client, hub))))
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "about.html")
	})
	mux.Handle("/list", rateLimitMiddleware(traceIDMiddleware(logger, listHandler(logger, client))))
	mux.Handle("/ws", rateLimitMiddleware(traceIDMiddleware(logger, wsHandler(logger, client, hub))))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Health check endpoint hit")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		logger.Info("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Could not start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for an interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Create a context with a timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Gracefully shut down the hub
	hub.Shutdown()
	logger.Info("Hub shutdown initiated")

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("Server exited properly")
}

func traceIDMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := uuid.New().String()
		ctx := context.WithValue(r.Context(), traceIDKey, traceID)
		// Add traceID to logger
		middlewareLogger := logger.With("traceID", traceID)
		// Add logger to context using typed key
		ctx = context.WithValue(ctx, loggerKey, middlewareLogger)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type MessageRequest struct {
	UserID  string `json:"userID"`
	Message string `json:"message"`
}

func messageHandler(logger *slog.Logger, client proto.StoreClient, hub *Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			logger.Warn("Invalid method for /message", "method", r.Method)
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var req MessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Warn("Invalid request body", "error", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Input validation
		if len(req.UserID) == 0 || len(req.UserID) > 64 {
			logger.Warn("Invalid userID length", "userID", req.UserID)
			http.Error(w, "Invalid userID", http.StatusBadRequest)
			return
		}
		if len(req.Message) == 0 || len(req.Message) > 1024 {
			logger.Warn("Invalid message length", "userID", req.UserID, "length", len(req.Message))
			http.Error(w, "Invalid message length", http.StatusBadRequest)
			return
		}
		// Optionally, add more content validation (e.g., allowed chars)

		// Retrieve the logger from the context
		ctxLogger, ok := r.Context().Value(loggerKey).(*slog.Logger)
		if !ok {
			// Fallback to the base logger if not found
			ctxLogger = logger
		}

		_, err := client.Save(r.Context(), &proto.SaveRequest{UserID: req.UserID, Message: req.Message})
		if err != nil {
			ctxLogger.Error("Failed to save message", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		jsonMsg, err := json.Marshal(req)
		if err != nil {
			ctxLogger.Error("Failed to marshal message", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		hub.broadcast <- jsonMsg

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Message saved successfully"))
		ctxLogger.Info("Message saved successfully")
	})
}

func listHandler(logger *slog.Logger, client proto.StoreClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the logger from the context
		ctxLogger, ok := r.Context().Value(loggerKey).(*slog.Logger)
		if !ok {
			// Fallback to the base logger if not found
			ctxLogger = logger
		}

		resp, err := client.GetLast10(r.Context(), &proto.GetLast10Request{})
		if err != nil {
			ctxLogger.Error("Failed to get messages", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		tmpl, err := template.ParseFiles("list.html")
		if err != nil {
			ctxLogger.Error("Failed to parse template", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, resp.Messages); err != nil {
			ctxLogger.Error("Failed to execute template", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	})
}

func wsHandler(logger *slog.Logger, client proto.StoreClient, hub *Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Failed to upgrade connection", "error", err)
			return
		}
		wsClient := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), ctx: ctx, cancel: cancel}
		wsClient.hub.register <- wsClient

		// Send last 10 messages to the client
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Recovered in wsClient send", "error", r)
				}
			}()
			resp, err := client.GetLast10(ctx, &proto.GetLast10Request{})
			if err != nil {
				logger.Error("Failed to get messages", "error", err)
				return
			}
			for _, msg := range resp.Messages {
				select {
				case wsClient.send <- []byte(msg):
				case <-ctx.Done():
					return
				}
			}
		}()

		// Ensure goroutines are cleaned up on disconnect
		go wsClient.writePump()
		go wsClient.readPump()
	})
}
