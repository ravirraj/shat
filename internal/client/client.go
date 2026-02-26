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
			fmt.Println(err)
			c.UnregisterChan <- c
			c.Conn.Close()
			return
		}

		msg := string(b[:n])

		c.Broadcast <- msg
	}
}

func (c *Client) WriteLoop() {
	for msg := range c.Send {
		_,err := c.Conn.Write([]byte(msg + "\n"))
		if err !=nil {
			fmt.Println(err)
			return
		}
	}
}
