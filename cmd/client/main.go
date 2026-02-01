package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "github.com/mmagesh/rate-limiter/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
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

	clients := []pb.RateLimiterClient{client1, client2}
	serverNames := []string{"Server A (50051)", "Server B (50052)"}

	userID := "distributed_test_user"
	capacity := int64(5)
	refillRate := int64(1)

	log.Printf("--- STARTING DISTRIBUTED TEST for %s ---", userID)
	log.Printf("Traffic will be split between %v", serverNames)

	sendRequest := func(requestIndex int) {
		// Manual Load Balancer: Randomly pick a server (between 50051 and 50052)
		idx := rand.Intn(len(clients))
		client := clients[idx]
		serverName := serverNames[idx]

		// Make the gRPC Call
		resp, err := client.Allow(context.Background(), &pb.AllowRequest{
			UserId:     userID,
			ActionKey:  "login",
			Capacity:   capacity,
			RefillRate: refillRate,
		})

		// Log the result
		if err != nil {
			log.Printf("Request %d -> %s: RPC Error: %v", requestIndex, serverName, err)
		} else {
			status := "DENIED"
			if resp.Allowed {
				status = "ALLOWED"
			}
			fmt.Printf("Request %d -> %s: %s\n", requestIndex, serverName, status)
		}
	}

	for i := 1; i <= 9; i++ {
		sendRequest(i)
	}

	time.Sleep(1000 * time.Millisecond)
	sendRequest(11)
}
