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

git clone & npm i

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


## Protocol Specification


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
