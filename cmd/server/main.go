package main

import (
	"context"
	"flag"
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
	scriptSHA string
}

func (s *server) Allow(ctx context.Context, req *pb.AllowRequest) (*pb.AllowResponse, error) {
	key := fmt.Sprintf("rate_limit:%s:%s", req.UserId, req.ActionKey)
	now := time.Now().Unix()

	result, err := s.rdb.EvalSha(ctx, s.scriptSHA, []string{key}, req.Capacity, req.RefillRate, 1, now).Result()

	if err != nil {
		log.Printf("Redis error: %v", err)
		return &pb.AllowResponse{Allowed: true}, nil
	}

	allowed := result.(int64) == 1
	return &pb.AllowResponse{Allowed: allowed}, nil
}

func main() {
	// 1. Parse the port from command line flags
	// If no flag is provided, it defaults to "50051"
	port := flag.String("port", "50051", "The server port")

	// ADD THIS: Allow Redis address to be configured
	redisAddr := flag.String("redis_addr", "localhost:6379", "Address of Redis instance")
	flag.Parse()

	// 2. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: *redisAddr,
	})

	// Test Connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	// 3. Load Lua Script
	sha, err := rdb.ScriptLoad(context.Background(), assets.TokenBucketLua).Result()
	if err != nil {
		log.Fatalf("Failed to load Lua script: %v", err)
	}
	// Log which port we are loading the script on
	log.Printf("Server on port %s: Lua script loaded, SHA: %s", *port, sha)

	// 4. Start gRPC Server on the Dynamic Port
	addr := fmt.Sprintf(":%s", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterRateLimiterServer(s, &server{
		rdb:       rdb,
		scriptSHA: sha,
	})

	log.Printf("Rate Limiter Service running on %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
