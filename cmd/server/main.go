package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http" // <--- Needed to serve metrics
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
	// Counter for total requests processed, labeled by status (allowed/denied/error)
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ratelimiter_requests_total",
			Help: "Total number of rate limit requests processed",
		},
		[]string{"status", "action_key"}, // Labels we will use
	)

	// Histogram to track how fast our Redis/Lua checks are
	requestDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ratelimiter_duration_seconds",
			Help:    "Time taken to process a rate limit check",
			Buckets: prometheus.DefBuckets, // Uses default buckets (.005, .01, .025, .05, .1, etc.)
		},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestDuration)
}

type server struct {
	pb.UnimplementedRateLimiterServer
	rdb       *redis.Client
	scriptSHA string
}

func (s *server) Allow(ctx context.Context, req *pb.AllowRequest) (*pb.AllowResponse, error) {
	// Start timer
	timer := prometheus.NewTimer(requestDuration)
	defer timer.ObserveDuration()

	key := fmt.Sprintf("rate_limit:%s:%s", req.UserId, req.ActionKey)
	now := time.Now().Unix()

	// Hardcoded config for now
	maxViolations := 5
	jailTime := 60

	result, err := s.rdb.EvalSha(ctx, s.scriptSHA, []string{key},
		req.Capacity, req.RefillRate, 1, now, maxViolations, jailTime).Result()

	if err != nil {
		log.Printf("Redis error: %v", err)
		// Record the error metric
		requestsTotal.WithLabelValues("error", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: true}, nil // Fail Open
	}

	code := result.(int64)

	switch code {
	case 1:
		// Record Success
		requestsTotal.WithLabelValues("allowed", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: true}, nil
	case -1:
		// Record Banned
		log.Printf("⛔ ABUSE DETECTED: User %s is in JAIL", req.UserId)
		requestsTotal.WithLabelValues("banned", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: false}, nil
	default:
		// Record Rate Limited
		requestsTotal.WithLabelValues("denied", req.ActionKey).Inc()
		return &pb.AllowResponse{Allowed: false}, nil
	}
}

func main() {
	port := flag.String("port", "50051", "The server port")
	redisAddr := flag.String("redis_addr", "localhost:6379", "Address of Redis instance")
	// Port for Prometheus to scrape
	metricsPort := flag.String("metrics_port", "2112", "Port to expose Prometheus metrics")
	flag.Parse()

	// 1. Start Metrics Server in a background goroutine
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
	sha, err := rdb.ScriptLoad(context.Background(), assets.TokenBucketLua).Result()
	if err != nil {
		log.Fatalf("Failed to load Lua script: %v", err)
	}
	log.Printf("Server on port %s: Lua script loaded", *port)

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
