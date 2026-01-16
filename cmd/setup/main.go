package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// Admin tool for managing flash sale products

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	ctx := context.Background()

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}

	command := os.Args[1]

	switch command {
	case "init":
		if len(os.Args) != 4 {
			fmt.Println("Usage: setup init <product_id> <stock>")
			os.Exit(1)
		}
		productID := os.Args[2]
		stock := os.Args[3]
		initProduct(ctx, client, productID, stock)

	case "status":
		if len(os.Args) != 3 {
			fmt.Println("Usage: setup status <product_id>")
			os.Exit(1)
		}
		productID := os.Args[2]
		showStatus(ctx, client, productID)

	case "reset":
		if len(os.Args) != 3 {
			fmt.Println("Usage: setup reset <product_id>")
			os.Exit(1)
		}
		productID := os.Args[2]
		resetProduct(ctx, client, productID)

	case "buyers":
		if len(os.Args) != 3 {
			fmt.Println("Usage: setup buyers <product_id>")
			os.Exit(1)
		}
		productID := os.Args[2]
		showBuyers(ctx, client, productID)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func initProduct(ctx context.Context, client *redis.Client, productID, stock string) {
	stockKey := fmt.Sprintf("product:%s:stock", productID)
	buyersKey := fmt.Sprintf("product:%s:buyers", productID)

	// Set stock
	if err := client.Set(ctx, stockKey, stock, 0).Err(); err != nil {
		log.Fatalf("Failed to set stock: %v", err)
	}

	// Clear buyers list
	client.Del(ctx, buyersKey)

	fmt.Printf("✓ Product '%s' initialized with %s units\n", productID, stock)
}

func showStatus(ctx context.Context, client *redis.Client, productID string) {
	stockKey := fmt.Sprintf("product:%s:stock", productID)
	buyersKey := fmt.Sprintf("product:%s:buyers", productID)

	stock, err := client.Get(ctx, stockKey).Result()
	if err == redis.Nil {
		fmt.Printf("Product '%s' not found\n", productID)
		return
	} else if err != nil {
		log.Fatalf("Failed to get stock: %v", err)
	}

	buyerCount, err := client.LLen(ctx, buyersKey).Result()
	if err != nil {
		log.Fatalf("Failed to get buyer count: %v", err)
	}

	fmt.Printf("\n=== Product Status: %s ===\n", productID)
	fmt.Printf("Remaining Stock:   %s\n", stock)
	fmt.Printf("Successful Buyers: %d\n", buyerCount)
}

func resetProduct(ctx context.Context, client *redis.Client, productID string) {
	stockKey := fmt.Sprintf("product:%s:stock", productID)
	buyersKey := fmt.Sprintf("product:%s:buyers", productID)

	client.Del(ctx, stockKey)
	client.Del(ctx, buyersKey)

	fmt.Printf("✓ Product '%s' reset (deleted)\n", productID)
}

func showBuyers(ctx context.Context, client *redis.Client, productID string) {
	buyersKey := fmt.Sprintf("product:%s:buyers", productID)

	buyers, err := client.LRange(ctx, buyersKey, 0, -1).Result()
	if err != nil {
		log.Fatalf("Failed to get buyers: %v", err)
	}

	fmt.Printf("\n=== Buyers for %s (%d total) ===\n", productID, len(buyers))
	for i, buyer := range buyers {
		fmt.Printf("%d. %s\n", i+1, buyer)
	}
}

func printUsage() {
	fmt.Println(`Flash Sale Setup & Admin Tool

Commands:
  init <product_id> <stock>    Initialize a product with stock
  status <product_id>          Show product status
  reset <product_id>           Reset (delete) product data
  buyers <product_id>          List all successful buyers

Environment:
  REDIS_ADDR                   Redis address (default: localhost:6379)

Examples:
  setup init iphone15 100
  setup status iphone15
  setup buyers iphone15
  setup reset iphone15
`)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}