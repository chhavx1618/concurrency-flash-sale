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

func (c *Client) AttemptPurchase(ProdID, UserID string) (*PurchaseResp, error) {

}

//readframe, write frame. then attempt purchase, then close
func (c *Client) Close() error {
	return c.conn.Close()
}
