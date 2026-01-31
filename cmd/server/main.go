package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/mmagesh/rate-limiter/internal/assets"
	pb "github.com/mmagesh/rate-limiter/proto"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedRateLimiterServer
	rdb       *redis.Client
	scriptSHA string // The ID of the script in Redis
}

func (s *server) Allow(ctx context.Context, req *pb.AllowRequest) (*pb.AllowResponse, error) {
	// Create a unique key for this user + action combo
	key := fmt.Sprintf("rate_limit:%s:%s", req.UserId, req.ActionKey)

	// Use current time in Unix Seconds
	now := time.Now().Unix()

	// Execute the Lua Script
	// Keys: [key]
	// Args: [capacity, refill_rate, requested_tokens, current_time]
	result, err := s.rdb.EvalSha(ctx, s.scriptSHA, []string{key}, req.Capacity, req.RefillRate, 1, now).Result()

	if err != nil {
		// FAIL OPEN strategy: If Redis is down, allow traffic to prevent outage
		log.Printf("Redis error: %v", err)
		return &pb.AllowResponse{Allowed: true}, nil
	}

	// 1 means allowed, 0 means denied
	allowed := result.(int64) == 1
	return &pb.AllowResponse{Allowed: allowed}, nil
}

func main() {
	// 1. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Test Connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	// 2. Load Lua Script (Pre-load for performance)
	// We load it once, get the SHA, and use the SHA later.
	sha, err := rdb.ScriptLoad(context.Background(), assets.TokenBucketLua).Result()
	if err != nil {
		log.Fatalf("Failed to load Lua script: %v", err)
	}
	log.Printf("Lua script loaded, SHA: %s", sha)

	// 3. Start gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterRateLimiterServer(s, &server{
		rdb:       rdb,
		scriptSHA: sha,
	})

	log.Println("Rate Limiter Service running on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
