package main

import (
	"context"
	"fmt"
	"log"
	"math/rand" // <--- Added for random load balancing
	"time"

	pb "github.com/mmagesh/rate-limiter/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1. Connect to Multiple Rate Limiters (Simulating a Distributed Fleet)

	// Server A (Port 50051)
	conn1, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to 50051: %v", err)
	}
	defer conn1.Close()
	client1 := pb.NewRateLimiterClient(conn1)

	// Server B (Port 50052)
	conn2, err := grpc.NewClient("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to 50052: %v", err)
	}
	defer conn2.Close()
	client2 := pb.NewRateLimiterClient(conn2)

	// Create a list of available clients
	clients := []pb.RateLimiterClient{client1, client2}
	serverNames := []string{"Server A (50051)", "Server B (50052)"}

	// 2. Define test constraints
	userID := "distributed_test_user"
	capacity := int64(5)
	refillRate := int64(1)

	log.Printf("--- STARTING DISTRIBUTED TEST for %s ---", userID)
	log.Printf("Traffic will be split between %v", serverNames)

	// 3. Send 10 requests rapidly
	for i := 1; i <= 10; i++ {
		// --- MANUAL LOAD BALANCER ---
		// Randomly pick index 0 or 1
		targetIndex := rand.Intn(len(clients))
		selectedClient := clients[targetIndex]
		selectedServerName := serverNames[targetIndex]

		resp, err := selectedClient.Allow(context.Background(), &pb.AllowRequest{
			UserId:     userID,
			ActionKey:  "login",
			Capacity:   capacity,
			RefillRate: refillRate,
		})

		if err != nil {
			log.Printf("Request %d -> %s: RPC Error: %v", i, selectedServerName, err)
		} else {
			status := "❌ DENIED"
			if resp.Allowed {
				status = "✅ ALLOWED"
			}
			// We format the log to show WHICH server gave the answer
			fmt.Printf("Request %d -> %s: %s\n", i, selectedServerName, status)
		}

		time.Sleep(100 * time.Millisecond)
	}
}
