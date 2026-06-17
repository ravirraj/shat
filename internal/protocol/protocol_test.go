package protocol

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ravirraj/shat/internal/types"
)

func TestFrameConnRoundTrip(t *testing.T) {
	server, client := net.Pipe()

	serverFC := NewFrameConn(server)
	clientFC := NewFrameConn(client)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		msg, err := clientFC.ReadMsg()
		if err != nil {
			t.Errorf("ReadMsg failed: %v", err)
			return
		}
		if msg.Type != types.MsgChat {
			t.Errorf("expected type CHAT, got %s", msg.Type)
		}
		if msg.Payload != "hello" {
			t.Errorf("expected payload 'hello', got %q", msg.Payload)
		}
	}()

	err := serverFC.WriteMsg(&types.Message{
		Type:    types.MsgChat,
		Payload: "hello",
		Ts:      time.Now(),
	})
	if err != nil {
		t.Fatalf("WriteMsg failed: %v", err)
	}

	wg.Wait()
}

func TestFrameConnMultipleMessages(t *testing.T) {
	server, client := net.Pipe()

	serverFC := NewFrameConn(server)
	clientFC := NewFrameConn(client)

	messages := []types.MsgType{
		types.MsgChat,
		types.MsgDM,
		types.MsgRoomCreate,
		types.MsgRoomJoin,
		types.MsgTyping,
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i, expectedType := range messages {
			msg, err := clientFC.ReadMsg()
			if err != nil {
				t.Errorf("message %d: ReadMsg failed: %v", i, err)
				return
			}
			if msg.Type != expectedType {
				t.Errorf("message %d: expected type %s, got %s", i, expectedType, msg.Type)
			}
		}
	}()

	for _, msgType := range messages {
		err := serverFC.WriteMsg(&types.Message{
			Type: msgType,
			Ts:   time.Now(),
		})
		if err != nil {
			t.Fatalf("WriteMsg failed: %v", err)
		}
	}

	wg.Wait()
}

func TestFrameConnBinaryData(t *testing.T) {
	server, client := net.Pipe()

	serverFC := NewFrameConn(server)
	clientFC := NewFrameConn(client)

	testData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		msg, err := clientFC.ReadMsg()
		if err != nil {
			t.Errorf("ReadMsg failed: %v", err)
			return
		}
		if len(msg.Data) != len(testData) {
			t.Errorf("expected data length %d, got %d", len(testData), len(msg.Data))
			return
		}
		for i := range testData {
			if msg.Data[i] != testData[i] {
				t.Errorf("data mismatch at index %d", i)
				return
			}
		}
	}()

	err := serverFC.WriteMsg(&types.Message{
		Type: types.MsgEncrypted,
		Data: testData,
		Ts:   time.Now(),
	})
	if err != nil {
		t.Fatalf("WriteMsg failed: %v", err)
	}

	wg.Wait()
}

func TestFrameConnLargeMessage(t *testing.T) {
	server, client := net.Pipe()

	serverFC := NewFrameConn(server)
	clientFC := NewFrameConn(client)

	largePayload := make([]byte, 100000)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		msg, err := clientFC.ReadMsg()
		if err != nil {
			t.Errorf("ReadMsg failed: %v", err)
			return
		}
		if len(msg.Data) != len(largePayload) {
			t.Errorf("expected data length %d, got %d", len(largePayload), len(msg.Data))
		}
	}()

	err := serverFC.WriteMsg(&types.Message{
		Type: types.MsgEncrypted,
		Data: largePayload,
		Ts:   time.Now(),
	})
	if err != nil {
		t.Fatalf("WriteMsg failed: %v", err)
	}

	wg.Wait()
}

func TestFrameConnConcurrent(t *testing.T) {
	server, client := net.Pipe()

	serverFC := NewFrameConn(server)
	clientFC := NewFrameConn(client)

	const numMessages = 100

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < numMessages; i++ {
			msg, err := clientFC.ReadMsg()
			if err != nil {
				t.Errorf("ReadMsg failed: %v", err)
				return
			}
			if msg.Payload != "test" {
				t.Errorf("expected payload 'test', got %q", msg.Payload)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < numMessages; i++ {
			err := serverFC.WriteMsg(&types.Message{
				Type:    types.MsgChat,
				Payload: "test",
				Ts:      time.Now(),
			})
			if err != nil {
				t.Errorf("WriteMsg failed: %v", err)
				return
			}
		}
	}()

	wg.Wait()
}

func BenchmarkWriteMsg(b *testing.B) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	serverFC := NewFrameConn(server)
	clientFC := NewFrameConn(client)

	msg := &types.Message{
		Type:    types.MsgChat,
		Payload: "benchmark message",
		Ts:      time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serverFC.WriteMsg(msg)
		clientFC.ReadMsg()
	}
}
