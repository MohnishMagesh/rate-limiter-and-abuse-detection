package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/mmagesh/rate-limiter/internal/assets"
	pb "github.com/mmagesh/rate-limiter/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

// --- METRICS DEFINITION ---
var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimiter_requests_total",
			Help: "Total number of rate limit requests processed",
		},
		[]string{"status", "action_key"},
	)

	requestDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ratelimiter_duration_seconds",
			Help:    "Time taken to process a rate limit check",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestDuration)
}

type server struct {
	pb.UnimplementedRateLimiterServer
	rdb       *redis.Client
	scriptSHA string
}

func (s *server) Allow(ctx context.Context, req *pb.AllowRequest) (*pb.AllowResponse, error) {
	timer := prometheus.NewTimer(requestDuration)
	defer timer.ObserveDuration()

	key := fmt.Sprintf("rate_limit:%s:%s", req.UserId, req.ActionKey)
	now := time.Now().Unix()

	// --- ABUSE CONFIGURATION ---
	// If a user gets "Denied" 5 times in a row, they are banned for 60 seconds.
	maxViolations := 5
	jailTime := 60

	// Execute Lua Script (Passing 6 arguments)
	// ARGV[1]: Capacity
	// ARGV[2]: Refill Rate
	// ARGV[3]: Requested (1)
	// ARGV[4]: Now (Unix Time)
	// ARGV[5]: Max Violations
	// ARGV[6]: Jail Time
	result, err := s.rdb.EvalSha(ctx, s.scriptSHA, []string{key},
		req.Capacity, req.RefillRate, 1, now, maxViolations, jailTime).Result()

	if err != nil {
		log.Printf("Redis error: %v", err)
		requestsTotal.WithLabelValues("error", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: true}, nil // Fail Open
	}

	code := result.(int64)

	switch code {
	case 1: // Allowed
		requestsTotal.WithLabelValues("allowed", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: true}, nil

	case -1: // BANNED (Abuse Detected)
		// We log this prominently
		log.Printf("⛔ ABUSE DETECTED: User %s is in JAIL", req.UserId)
		requestsTotal.WithLabelValues("banned", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: false}, nil

	default: // 0 = Denied (Standard Rate Limit)
		requestsTotal.WithLabelValues("denied", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: false}, nil
	}
}

func main() {
	port := flag.String("port", "50051", "The server port")
	redisAddr := flag.String("redis_addr", "localhost:6379", "Address of Redis instance")
	metricsPort := flag.String("metrics_port", "2112", "Port to expose Prometheus metrics")
	flag.Parse()

	// 1. Start Metrics Server
	go func() {
		addr := fmt.Sprintf(":%s", *metricsPort)
		log.Printf("📊 Metrics server listening on %s/metrics", addr)
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("Failed to start metrics server: %v", err)
		}
	}()

	// 2. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: *redisAddr,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	// 3. Load Lua Script
	// NOTE: Since the script content changed, this will generate a NEW SHA hash.
	sha, err := rdb.ScriptLoad(context.Background(), assets.TokenBucketLua).Result()
	if err != nil {
		log.Fatalf("Failed to load Lua script: %v", err)
	}
	log.Printf("Server on port %s: Lua script loaded (SHA: %s)", *port, sha)

	// 4. Start gRPC Server
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
