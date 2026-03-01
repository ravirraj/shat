package server

import (
	"fmt"
	"net"

	"github.com/ravirraj/shat/internal/client"
	"github.com/ravirraj/shat/internal/hub"
)

type Server struct {
	Addr string
	Hub  *hub.Hub
}

func NewServer(addr string, hub *hub.Hub) *Server {
	return &Server{
		Addr: addr,
		Hub:  hub,
	}

}
func (s *Server) Start() {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Server Starting at Port %v \n", s.Addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("THis errr", err)
			return
		}
		defer conn.Close()

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	buffer := make([]byte, 1024)
	n, _ := conn.Read(buffer)
	client := &client.Client{
		Name:           string(buffer[:n]),
		Conn:           conn,
		Send:           make(chan string),
		RegisterChan:   s.Hub.RegisterChan,
		UnregisterChan: s.Hub.UnregisterChan,
		Broadcast:      s.Hub.Broadcast,
	}
	s.Hub.RegisterChan <- client

	go client.WriteLoop()
	go client.ReadLoop()
}
