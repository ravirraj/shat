package client

import (
	"fmt"

	"github.com/ravirraj/shat/internal/protocol"
	"github.com/ravirraj/shat/internal/types"
)

type Client struct {
	Name           string
	Conn           *protocol.FrameConn
	Send           chan *types.Message
	RegisterChan   chan<- *Client
	UnregisterChan chan<- *Client
	Broadcast      chan<- *types.RoomMessage
	Room           string
}

func (c *Client) ReadLoop() {
	for {
		msg, err := c.Conn.ReadMsg()
		if err != nil {
			fmt.Printf("read error from %s: %v\n", c.Name, err)
			c.UnregisterChan <- c
			c.Conn.Close()
			return
		}

		msg.From = c.Name
		msg.Room = c.Room

		c.Broadcast <- &types.RoomMessage{
			Room:    msg.Room,
			Message: msg,
		}
	}
}

func (c *Client) WriteLoop() {
	for msg := range c.Send {
		if err := c.Conn.WriteMsg(msg); err != nil {
			fmt.Printf("write error to %s: %v\n", c.Name, err)
			return
		}
	}
}

func (c *Client) SendMessage(msg *types.Message) {
	select {
	case c.Send <- msg:
	default:
		fmt.Printf("send buffer full for %s, dropping message\n", c.Name)
	}
}
