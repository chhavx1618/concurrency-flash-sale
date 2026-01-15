package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

const ( ATTMP_PURCHASE byte = 0x01)

type PurchaseRequest struct {
	ProdID string `json:"prod_id"`
	UserID string `json:"user_id"`
}

type PurchaseResp struct {
	Status string `json:"status"`
	Remainings int64 `json:"remaining_stock,omitempty"`
	Error string `json:"error,omitempty"`
}

type Client struct {
	conn net.Conn
	mu sync.Mutex
}

func NewClient(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err!=nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) writeFrame(msgType byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.conn.Write([]byte{msgType}); err != nil {
		return err
	}

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))
	if _,err := c.conn.Write(lenBuf); err!= nil {
		return err
	}

	_, err := c.conn.Write(payload)
	return err
}

//readframe, write frame. then attempt purchase, then close
