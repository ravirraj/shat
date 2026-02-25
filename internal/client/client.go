package client

import (
	"net"

	"github.com/ravirraj/shat/internal/hub"
	// "github.com/ravirraj/shat/internal/hub"
)

type Client struct {
	Name string
	Conn net.Conn
	Send chan string
	Hub  *hub.Hub
}
