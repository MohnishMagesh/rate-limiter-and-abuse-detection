package main

import (
	"context"
	"log"
	"time"

	pb "github.com/mmagesh/rate-limiter/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 1. Connect to the Rate Limiter Service
	// We use "insecure" because we are on localhost and haven't set up TLS certificates
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewRateLimiterClient(conn)

	// 2. Define our test constraints
	userID := "test_user_123"
	capacity := int64(5)   // Max 5 tokens
	refillRate := int64(1) // Add 1 token every second

	log.Printf("--- STARTING BURST TEST for %s ---", userID)
	log.Printf("Capacity: %d, Rate: %d/sec", capacity, refillRate)

	// 3. Send 10 requests rapidly
	for i := 1; i <= 10; i++ {
		resp, err := client.Allow(context.Background(), &pb.AllowRequest{
			UserId:     userID,
			ActionKey:  "login",
			Capacity:   capacity,
			RefillRate: refillRate,
		})

		if err != nil {
			log.Printf("RPC Error: %v", err)
		} else {
			if resp.Allowed {
				log.Printf("Request %d: ✅ ALLOWED", i)
			} else {
				log.Printf("Request %d: ❌ DENIED", i)
			}
		}
		// Minimal sleep to ensure we don't hit network race conditions on localhost
		time.Sleep(10 * time.Millisecond)
	}

	// 4. Test the Refill
	log.Println("--- WAITING 3 SECONDS (Refilling...) ---")
	time.Sleep(3 * time.Second)

	log.Println("--- SENDING NEW REQUEST ---")
	resp, _ := client.Allow(context.Background(), &pb.AllowRequest{
		UserId:     userID,
		ActionKey:  "login",
		Capacity:   capacity,
		RefillRate: refillRate,
	})

	if resp.Allowed {
		log.Println("Request 11: ✅ ALLOWED (Bucket refilled!)")
	} else {
		log.Println("Request 11: ❌ DENIED (Something is wrong)")
	}
}
