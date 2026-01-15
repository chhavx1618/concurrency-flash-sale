package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MSG_ATTEMPT_PURCHASE byte = 0x01
)

type PurchaseRequest struct {
	ProductID string `json:"product_id"`
	UserID    string `json:"user_id"`
}

type PurchaseResponse struct {
	Status         string `json:"status"`
	RemainingStock int64  `json:"remaining_stock,omitempty"`
	Error          string `json:"error,omitempty"`
}

type Client struct {
	conn net.Conn
	mu   sync.Mutex
}

func NewClient(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) writeFrame(msgType byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TYPE
	if _, err := c.conn.Write([]byte{msgType}); err != nil {
		return err
	}

	// LENGTH
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))
	if _, err := c.conn.Write(lenBuf); err != nil {
		return err
	}

	// PAYLOAD
	_, err := c.conn.Write(payload)
	return err
}

func (c *Client) readFrame() (byte, []byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TYPE
	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(c.conn, typeBuf); err != nil {
		return 0, nil, err
	}

	// LENGTH
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, lenBuf); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	// PAYLOAD
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return 0, nil, err
	}

	return typeBuf[0], payload, nil
}

func (c *Client) AttemptPurchase(productID, userID string) (*PurchaseResponse, error) {
	req := PurchaseRequest{
		ProductID: productID,
		UserID:    userID,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if err := c.writeFrame(MSG_ATTEMPT_PURCHASE, payload); err != nil {
		return nil, err
	}

	_, respPayload, err := c.readFrame()
	if err != nil {
		return nil, err
	}

	var resp PurchaseResponse
	if err := json.Unmarshal(respPayload, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Benchmark runs a concurrent load test
func Benchmark(serverAddr, productID string, numClients, numAttempts int) {
	var (
		successCount int64
		failCount    int64
		errorCount   int64
		totalLatency int64
	)

	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			client, err := NewClient(serverAddr)
			if err != nil {
				log.Printf("Client %d: connection failed: %v", clientID, err)
				atomic.AddInt64(&errorCount, int64(numAttempts))
				return
			}
			defer client.Close()

			for j := 0; j < numAttempts; j++ {
				userID := fmt.Sprintf("user_%d_%d", clientID, j)

				reqStart := time.Now()
				resp, err := client.AttemptPurchase(productID, userID)
				latency := time.Since(reqStart)

				atomic.AddInt64(&totalLatency, latency.Microseconds())

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				switch resp.Status {
				case "SUCCESS":
					atomic.AddInt64(&successCount, 1)
				case "SOLD_OUT":
					atomic.AddInt64(&failCount, 1)
				default:
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Results
	totalReqs := successCount + failCount + errorCount
	fmt.Println("\n=== Benchmark Results ===")
	fmt.Printf("Duration:          %v\n", duration)
	fmt.Printf("Total Requests:    %d\n", totalReqs)
	fmt.Printf("Successful:        %d\n", successCount)
	fmt.Printf("Sold Out:          %d\n", failCount)
	fmt.Printf("Errors:            %d\n", errorCount)
	fmt.Printf("Throughput:        %.0f req/sec\n", float64(totalReqs)/duration.Seconds())
	fmt.Printf("Avg Latency:       %.2f ms\n", float64(totalLatency)/float64(totalReqs)/1000)
	fmt.Printf("Oversell Check:    %s\n", checkOversell(successCount))
}

func checkOversell(successCount int64) string {
	if successCount <= 100 {
		return "✓ PASS"
	}
	return fmt.Sprintf("✗ FAIL (oversold by %d)", successCount-100)
}

func main() {
	serverAddr := "localhost:8080"
	productID := "iphone15"

	fmt.Println("Flash Sale Client - Benchmark Mode")
	fmt.Printf("Server: %s\n", serverAddr)
	fmt.Printf("Product: %s\n", productID)
	fmt.Println("\nStarting benchmark...")

	// Run benchmark: 1000 clients, 10 attempts each
	Benchmark(serverAddr, productID, 10000, 10)
}