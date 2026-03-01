package client

import (
	"fmt"
	"net"
	// "github.com/ravirraj/shat/internal/hub"
	// "github.com/ravirraj/shat/internal/hub"
)

type Client struct {
	Name           string
	Conn           net.Conn
	Send           chan string
	RegisterChan   chan<- *Client
	UnregisterChan chan<- *Client
	Broadcast      chan<- string
}

func (c *Client) ReadLoop() {
	b := make([]byte, 1024)

	for {
		n, err := c.Conn.Read(b)
		if err != nil {
			fmt.Println("err", err)
			c.UnregisterChan <- c
			c.Conn.Close()
			return
		}

		msg := string(b[:n])
		formaterdMessage := fmt.Sprintf("%s: %s", c.Name, msg)

		c.Broadcast <- formaterdMessage
	}
}

func (c *Client) WriteLoop() {
	for msg := range c.Send {
		_, err := c.Conn.Write([]byte(msg))
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}
