package hub

import (
	"fmt"

	"github.com/ravirraj/shat/internal/client"
)

type Hub struct {
	Clients        map[*client.Client]bool
	RegisterChan   chan *client.Client
	UnregisterChan chan *client.Client
	Broadcast      chan string
}

func NewHub() *Hub {

	return &Hub{
		Clients:        make(map[*client.Client]bool),
		RegisterChan:   make(chan *client.Client),
		UnregisterChan: make(chan *client.Client),
		Broadcast:      make(chan string),
	}

}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.RegisterChan:
			h.Clients[c] = true
			fmt.Printf("%v joined \n", c.Name)

		case c := <-h.UnregisterChan:
			fmt.Printf("%v Left", c.Name)
			delete(h.Clients, c)
			close(c.Send)

		case b := <-h.Broadcast:
			for client := range h.Clients {
				client.Send <- b
			}
		}
	}
}
