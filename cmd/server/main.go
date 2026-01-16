package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Message types
	MSG_ATTEMPT_PURCHASE byte = 0x01

	// Response statuses
	STATUS_SUCCESS  = "SUCCESS"
	STATUS_SOLD_OUT = "SOLD_OUT"
	STATUS_ERROR    = "ERROR"
)

// PurchaseRequest represents a purchase attempt
type PurchaseRequest struct {
	ProductID string `json:"product_id"`
	UserID    string `json:"user_id"`
}

// PurchaseResponse represents the result of a purchase attempt
type PurchaseResponse struct {
	Status         string `json:"status"`
	RemainingStock int64  `json:"remaining_stock,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Server manages the flash sale engine
type Server struct {
	redis    *redis.Client
	listener net.Listener
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	luaHash  string
}

// Lua script for atomic purchase
const luaScript = `
local stock = tonumber(redis.call("GET", KEYS[1]))

if stock and stock > 0 then
    redis.call("DECR", KEYS[1])
    redis.call("LPUSH", KEYS[2], ARGV[1])
    return {1, stock - 1}
else
    return {0, 0}
end
`

// NewServer creates a new flash sale server
func NewServer(redisAddr, listenAddr string) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		PoolSize:     100,
		MinIdleConns: 10,
		MaxRetries:   3,
	})

	// Test connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	// Load Lua script
	hash, err := rdb.ScriptLoad(ctx, luaScript).Result()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load lua script: %w", err)
	}

	// Create TCP listener
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	s := &Server{
		redis:    rdb,
		listener: ln,
		ctx:      ctx,
		cancel:   cancel,
		luaHash:  hash,
	}

	log.Printf("Server initialized - Listening on %s, Redis: %s", listenAddr, redisAddr)
	return s, nil
}

// Start begins accepting connections
func (s *Server) Start() {
	s.wg.Add(1)
	go s.acceptLoop()
}

// acceptLoop handles incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection processes a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	log.Printf("New connection from %s", conn.RemoteAddr())

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read TLV frame
		msgType, payload, err := s.readFrame(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}

		// Process message
		response := s.processMessage(msgType, payload)

		// Send response
		if err := s.writeFrame(conn, msgType, response); err != nil {
			log.Printf("Write error to %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

// readFrame reads a TLV frame from the connection
func (s *Server) readFrame(conn net.Conn) (byte, []byte, error) {
	// Read TYPE (1 byte)
	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, typeBuf); err != nil {
		return 0, nil, err
	}

	// Read LENGTH (4 bytes, big-endian)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	// Validate length (max 1MB)
	if length > 1024*1024 {
		return 0, nil, fmt.Errorf("payload too large: %d", length)
	}

	// Read PAYLOAD
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, nil, err
	}

	return typeBuf[0], payload, nil
}

// writeFrame writes a TLV frame to the connection
func (s *Server) writeFrame(conn net.Conn, msgType byte, payload []byte) error {
	// TYPE (1 byte)
	if _, err := conn.Write([]byte{msgType}); err != nil {
		return err
	}

	// LENGTH (4 bytes, big-endian)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))
	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}

	// PAYLOAD
	_, err := conn.Write(payload)
	return err
}

// processMessage handles a single message
func (s *Server) processMessage(msgType byte, payload []byte) []byte {
	switch msgType {
	case MSG_ATTEMPT_PURCHASE:
		return s.handlePurchaseAttempt(payload)
	default:
		resp := PurchaseResponse{
			Status: STATUS_ERROR,
			Error:  "unknown message type",
		}
		data, _ := json.Marshal(resp)
		return data
	}
}

// handlePurchaseAttempt processes a purchase attempt
func (s *Server) handlePurchaseAttempt(payload []byte) []byte {
	var req PurchaseRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		resp := PurchaseResponse{
			Status: STATUS_ERROR,
			Error:  "invalid json",
		}
		data, _ := json.Marshal(resp)
		return data
	}

	// Validate request
	if req.ProductID == "" || req.UserID == "" {
		resp := PurchaseResponse{
			Status: STATUS_ERROR,
			Error:  "missing product_id or user_id",
		}
		data, _ := json.Marshal(resp)
		return data
	}

	// Execute atomic purchase via Lua script
	stockKey := fmt.Sprintf("product:%s:stock", req.ProductID)
	buyersKey := fmt.Sprintf("product:%s:buyers", req.ProductID)

	result, err := s.redis.EvalSha(
		s.ctx,
		s.luaHash,
		[]string{stockKey, buyersKey},
		req.UserID,
	).Result()

	if err != nil {
		resp := PurchaseResponse{
			Status: STATUS_ERROR,
			Error:  fmt.Sprintf("redis error: %v", err),
		}
		data, _ := json.Marshal(resp)
		return data
	}

	// Parse Lua result
	arr, ok := result.([]interface{})
	if !ok || len(arr) != 2 {
		resp := PurchaseResponse{
			Status: STATUS_ERROR,
			Error:  "invalid lua response",
		}
		data, _ := json.Marshal(resp)
		return data
	}

	success := arr[0].(int64)
	remaining := arr[1].(int64)

	var resp PurchaseResponse
	if success == 1 {
		resp = PurchaseResponse{
			Status:         STATUS_SUCCESS,
			RemainingStock: remaining,
		}

		// Publish event (async, best-effort)
		go s.publishEvent(req.ProductID, req.UserID, remaining)
	} else {
		resp = PurchaseResponse{
			Status: STATUS_SOLD_OUT,
		}
	}

	data, _ := json.Marshal(resp)
	return data
}

// publishEvent publishes a purchase event to Redis pub/sub
func (s *Server) publishEvent(productID, userID string, remaining int64) {
	event := map[string]interface{}{
		"product_id": productID,
		"buyer":      userID,
		"remaining":  remaining,
		"timestamp":  time.Now().Unix(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	if err := s.redis.Publish(s.ctx, "flashsale_events", data).Err(); err != nil {
		log.Printf("Failed to publish event: %v", err)
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	log.Println("Shutting down server...")
	s.cancel()
	s.listener.Close()
	s.wg.Wait()
	s.redis.Close()
	log.Println("Server stopped")
}

func main() {
	// Configuration
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	listenAddr := getEnv("LISTEN_ADDR", ":8080")

	// Create server
	server, err := NewServer(redisAddr, listenAddr)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	server.Start()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	server.Shutdown()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}