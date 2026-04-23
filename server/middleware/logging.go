package middleware

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
)

// UnaryLogger logs gRPC unary calls with method, duration, and any error.
func UnaryLogger(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	slog.Info("rpc",
		"method", info.FullMethod,
		"duration", time.Since(start),
		"error", err,
	)
	return resp, err
}

// StreamLogger logs gRPC streaming calls with method and duration.
func StreamLogger(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	slog.Info("stream",
		"method", info.FullMethod,
		"duration", time.Since(start),
		"error", err,
	)
	return err
}
