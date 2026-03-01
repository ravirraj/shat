package main

import (
	// "github.com/ravirraj/shat/internal/client"
	"github.com/ravirraj/shat/internal/hub"
	"github.com/ravirraj/shat/internal/server"
)

func main() {

	h := hub.NewHub()

	go h.Run()

	s := server.NewServer(":8000", h)
	s.Start()

}
