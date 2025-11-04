package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"

	"unified-go-prog/proto"
	"unified-go-prog/store"
)

// server is used to implement proto.StoreServer.
type server struct {
	proto.UnimplementedStoreServer
	logger *slog.Logger
	store  *store.FileStore
}

// Save implements proto.StoreServer
func (s *server) Save(ctx context.Context, req *proto.SaveRequest) (*proto.SaveResponse, error) {
	s.logger.Info("Saving message", "userID", req.UserID, "message", req.Message)
	if err := s.store.SaveMessage(ctx, s.logger, req.UserID, req.Message); err != nil {
		return nil, err
	}
	return &proto.SaveResponse{}, nil
}

// GetLast10 implements proto.StoreServer
func (s *server) GetLast10(ctx context.Context, req *proto.GetLast10Request) (*proto.GetLast10Response, error) {
	s.logger.Info("Getting last 10 messages")
	messages, err := s.store.GetLast10Messages(ctx, s.logger)
	if err != nil {
		return nil, err
	}
	return &proto.GetLast10Response{Messages: messages}, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	storeInst := &store.FileStore{FileName: "messages.txt"}
	proto.RegisterStoreServer(s, &server{logger: logger, store: storeInst})
	logger.Info("gRPC server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
