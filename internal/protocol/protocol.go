package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/ravirraj/shat/internal/types"
)

const MaxMessageSize = 1 << 20 // 1MB

type FrameConn struct {
	conn    net.Conn
	mu      sync.Mutex
	reader  io.Reader
	writeMu sync.Mutex
}

func NewFrameConn(conn net.Conn) *FrameConn {
	return &FrameConn{
		conn:   conn,
		reader: conn,
	}
}

func (fc *FrameConn) WriteMsg(msg *types.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	fc.writeMu.Lock()
	defer fc.writeMu.Unlock()

	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))

	if _, err := fc.conn.Write(length); err != nil {
		return fmt.Errorf("write length: %w", err)
	}
	if _, err := fc.conn.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

func (fc *FrameConn) ReadMsg() (*types.Message, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(fc.reader, lengthBuf); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(fc.reader, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	var msg types.Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}

	return &msg, nil
}

func (fc *FrameConn) Close() error {
	return fc.conn.Close()
}

func (fc *FrameConn) RemoteAddr() net.Addr {
	return fc.conn.RemoteAddr()
}
