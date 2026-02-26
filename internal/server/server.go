package server

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"github.com/ravirraj/shat/internal/client"
	"github.com/ravirraj/shat/internal/hub"
)

type Server struct {
	Addr string
	Hub  *hub.Hub
}

func (s *Server) Start() {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	client := &client.Client{
		Name:           name,
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
