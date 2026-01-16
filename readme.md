# Flash Sale Engine - Complete Implementation

High-throughput, zero-oversell flash sale system built in Go.

## Features

✓ **Zero Overselling** - Atomic stock management via Redis Lua  
✓ **Sub-millisecond Latency** - Direct TCP protocol, no HTTP overhead  
✓ **Linear Scalability** - Stateless servers, horizontal scaling ready  
✓ **Production Ready** - Graceful shutdown, connection pooling, error handling  

## Architecture

```
┌─────────┐                 ┌──────────┐                 ┌───────┐
│ Client  │────TCP TLV─────▶│  Server  │────Lua Script──▶│ Redis │
│ (Go)    │◀────JSON────────│  (Go)    │◀────Result──────│       │
└─────────┘                 └──────────┘                 └───────┘
                                   │
                                   ▼
                            ┌─────────────┐
                            │  Pub/Sub    │
                            │  (Events)   │
                            └─────────────┘
```

## Prerequisites

- **Go 1.21+**
- **Redis 6.0+**

## Installation

### 1. Clone and Setup

```bash
# Create project directory
mkdir flash-sale-engine
cd flash-sale-engine

# Initialize Go module
go mod init flash-sale-engine

# Install dependencies
go get github.com/redis/go-redis/v9
```

### 2. Project Structure

```
flash-sale-engine/
├── cmd/
│   ├── server/
│   │   └── main.go          # Server implementation
│   ├── client/
│   │   └── main.go          # Client/benchmark tool
│   └── setup/
│       └── main.go          # Admin tool
├── go.mod
└── README.md
```

### 3. Copy Code

Save the three code artifacts:
- **Server**: `cmd/server/main.go`
- **Client**: `cmd/client/main.go`
- **Setup**: `cmd/setup/main.go`

### 4. Start Redis

```bash
# Using Docker
docker run -d -p 6379:6379 redis:7-alpine

# Or install locally (macOS)
brew install redis
redis-server
```

## Quick Start

### Step 1: Initialize Product

```bash
go run cmd/setup/main.go init iphone15 100
```

Output:
```
✓ Product 'iphone15' initialized with 100 units
```

### Step 2: Start Server

```bash
go run cmd/server/main.go
```

Output:
```
Server initialized - Listening on :8080, Redis: localhost:6379
```

### Step 3: Run Benchmark

```bash
go run cmd/client/main.go
```

Output:
```
=== Benchmark Results ===
Duration:          1.234s
Total Requests:    10000
Successful:        100
Sold Out:          9900
Errors:            0
Throughput:        8100 req/sec
Avg Latency:       0.12 ms
Oversell Check:    ✓ PASS
```

## Protocol Specification

### Message Format (TLV)

```
┌──────┬─────────┬──────────────┐
│ TYPE │ LENGTH  │   PAYLOAD    │
│ 1B   │  4B BE  │   N bytes    │
└──────┴─────────┴──────────────┘
```

### Message Types

| Type | Value | Description |
|------|-------|-------------|
| ATTEMPT_PURCHASE | 0x01 | Purchase attempt |

### Request Payload

```json
{
  "product_id": "iphone15",
  "user_id": "user_123"
}
```

### Response Payload

**Success:**
```json
{
  "status": "SUCCESS",
  "remaining_stock": 42
}
```

**Sold Out:**
```json
{
  "status": "SOLD_OUT"
}
```

**Error:**
```json
{
  "status": "ERROR",
  "error": "invalid json"
}
```

## Redis Data Model

### Keys

```
product:{id}:stock     → Integer (remaining stock)
product:{id}:buyers    → List (successful user IDs)
```

### Example

```redis
SET product:iphone15:stock 100
LPUSH product:iphone15:buyers user_123
```

## Admin Commands

### Check Product Status

```bash
go run cmd/setup/main.go status iphone15
```

Output:
```
=== Product Status: iphone15 ===
Remaining Stock:   42
Successful Buyers: 58
```

### List All Buyers

```bash
go run cmd/setup/main.go buyers iphone15
```

Output:
```
=== Buyers for iphone15 (58 total) ===
1. user_0_0
2. user_1_0
...
```

### Reset Product

```bash
go run cmd/setup/main.go reset iphone15
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | localhost:6379 | Redis server address |
| `LISTEN_ADDR` | :8080 | TCP listen address |

### Example

```bash
# Server with custom config
REDIS_ADDR=redis.prod.com:6379 LISTEN_ADDR=:9000 go run cmd/server/main.go

# Setup with remote Redis
REDIS_ADDR=redis.prod.com:6379 go run cmd/setup/main.go init product123 500
```

## Production Deployment

### Build Binaries

```bash
# Server
go build -o bin/flashsale-server cmd/server/main.go

# Client
go build -o bin/flashsale-client cmd/client/main.go

# Setup
go build -o bin/flashsale-setup cmd/setup/main.go
```

### Run Server

```bash
./bin/flashsale-server
```

### Docker Deployment

**Dockerfile:**
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
```

**Build and Run:**
```bash
docker build -t flashsale-server .
docker run -p 8080:8080 -e REDIS_ADDR=host.docker.internal:6379 flashsale-server
```

## Performance Tuning

### Redis Configuration

```conf
# redis.conf
maxclients 10000
timeout 0
tcp-keepalive 300
```

### Server Tuning

```go
// Increase connection pool
redis.NewClient(&redis.Options{
    Addr:         redisAddr,
    PoolSize:     200,      // Increase for more concurrency
    MinIdleConns: 50,       // Keep connections warm
})
```

### OS Tuning (Linux)

```bash
# Increase file descriptors
ulimit -n 65536

# Tune TCP settings
sysctl -w net.core.somaxconn=4096
sysctl -w net.ipv4.tcp_max_syn_backlog=4096
```

## Monitoring

### Pub/Sub Events

Subscribe to real-time events:

```bash
redis-cli SUBSCRIBE flashsale_events
```

Event format:
```json
{
  "product_id": "iphone15",
  "buyer": "user_123",
  "remaining": 42,
  "timestamp": 1705089600
}
```

### Metrics to Track

- **Throughput**: Requests per second
- **Latency**: P50, P95, P99 response times
- **Error Rate**: Failed requests / total requests
- **Stock Accuracy**: Verify remaining_stock + buyers_count = initial_stock

## Testing

### Unit Test

```go
// server_test.go
func TestAtomicPurchase(t *testing.T) {
    // Setup Redis
    client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    ctx := context.Background()
    
    // Initialize product
    client.Set(ctx, "product:test:stock", 10, 0)
    
    // Execute Lua script
    result, err := client.Eval(ctx, luaScript, 
        []string{"product:test:stock", "product:test:buyers"},
        "user_test").Result()
    
    assert.NoError(t, err)
    arr := result.([]interface{})
    assert.Equal(t, int64(1), arr[0])
    assert.Equal(t, int64(9), arr[1])
}
```

### Load Test

```bash
# Custom benchmark
go run cmd/client/main.go

# Or use hey
hey -n 10000 -c 100 -m POST -d '{"product_id":"test","user_id":"user"}' http://localhost:8080
```

## Troubleshooting

### "Connection refused"

**Solution:** Ensure Redis is running
```bash
redis-cli ping
# Should return: PONG
```

### "Too many open files"

**Solution:** Increase file descriptor limit
```bash
ulimit -n 65536
```

### High latency

**Possible causes:**
- Network latency to Redis
- Redis under high load
- Too few connection pool workers

**Solution:** Scale Redis (Cluster) or increase connection pool size

## Scaling Strategy

### Vertical Scaling

1. Increase server CPU cores
2. Increase Redis memory
3. Use Redis Cluster

### Horizontal Scaling

1. Deploy multiple server instances
2. Use load balancer (TCP)
3. Point all servers to same Redis

```
              ┌──────────┐
              │   LB     │
              └────┬─────┘
           ┌───────┼───────┐
           │       │       │
      ┌────▼──┐ ┌──▼───┐ ┌▼────┐
      │Server1│ │Server2│ │Server3│
      └───┬───┘ └──┬───┘ └┬────┘
          └────────┼──────┘
                   │
              ┌────▼────┐
              │  Redis  │
              └─────────┘
```

## Security Considerations

### Rate Limiting (Per User)

```go
// Add rate limiting middleware
func (s *Server) checkRateLimit(userID string) bool {
    key := fmt.Sprintf("ratelimit:%s", userID)
    count, _ := s.redis.Incr(s.ctx, key).Result()
    if count == 1 {
        s.redis.Expire(s.ctx, key, time.Minute)
    }
    return count <= 10 // Max 10 requests per minute
}
```

### Authentication

Add JWT or API key validation before processing purchase attempts.

### TLS

Wrap TCP server with TLS:
```go
cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
config := &tls.Config{Certificates: []tls.Certificate{cert}}
listener = tls.NewListener(listener, config)
```

## License

MIT

## Support

For issues or questions:
- GitHub Issues: [github.com/yourrepo/flash-sale-engine]
- Email: support@example.com