package hub

import (
	"fmt"
	"time"

	"github.com/ravirraj/shat/internal/client"
	"github.com/ravirraj/shat/internal/types"
)

type Hub struct {
	Clients        map[string]*client.Client
	Rooms          map[string]*types.Room
	RegisterChan   chan *client.Client
	UnregisterChan chan *client.Client
	Broadcast      chan *types.RoomMessage
	DMChan         chan *types.Message
}

func NewHub() *Hub {
	return &Hub{
		Clients:        make(map[string]*client.Client),
		Rooms:          make(map[string]*types.Room),
		RegisterChan:   make(chan *client.Client),
		UnregisterChan: make(chan *client.Client),
		Broadcast:      make(chan *types.RoomMessage),
		DMChan:         make(chan *types.Message),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.RegisterChan:
			h.Clients[c.Name] = c
			fmt.Printf("%s joined\n", c.Name)

			h.broadcastPresence(c.Name, "online")

			h.sendSystemMessage(c, "Welcome to Shat! Type /help for commands.")

		case c := <-h.UnregisterChan:
			fmt.Printf("%s left\n", c.Name)
			if c.Room != "" {
				if room, ok := h.Rooms[c.Room]; ok {
					delete(room.Clients, c.Name)
				}
			}
			h.broadcastPresence(c.Name, "offline")
			delete(h.Clients, c.Name)
			close(c.Send)

		case rm := <-h.Broadcast:
			h.handleBroadcast(rm)

		case dm := <-h.DMChan:
			h.handleDM(dm)
		}
	}
}

func (h *Hub) handleBroadcast(rm *types.RoomMessage) {
	msg := rm.Message
	roomName := rm.Room

	if roomName == "" {
		for _, c := range h.Clients {
			c.SendMessage(msg)
		}
		return
	}

	room, ok := h.Rooms[roomName]
	if !ok {
		return
	}

	for username := range room.Clients {
		if c, ok := h.Clients[username]; ok {
			c.SendMessage(msg)
		}
	}
}

func (h *Hub) handleDM(msg *types.Message) {
	if target, ok := h.Clients[msg.To]; ok {
		target.SendMessage(msg)
	}
}

func (h *Hub) sendSystemMessage(c *client.Client, text string) {
	msg := &types.Message{
		Type:    types.MsgSystem,
		Payload: text,
		From:    "system",
		Ts:      time.Now(),
	}
	c.SendMessage(msg)
}

func (h *Hub) broadcastPresence(username, status string) {
	msg := &types.Message{
		Type:    types.MsgPresence,
		From:    username,
		Payload: status,
		Ts:      time.Now(),
	}

	for _, c := range h.Clients {
		if c.Name != username {
			c.SendMessage(msg)
		}
	}
}

func (h *Hub) broadcastSystem(roomName, text string) {
	msg := &types.Message{
		Type:    types.MsgSystem,
		Payload: text,
		From:    "system",
		Ts:      time.Now(),
	}

	room, ok := h.Rooms[roomName]
	if !ok {
		return
	}

	for username := range room.Clients {
		if c, ok := h.Clients[username]; ok {
			c.SendMessage(msg)
		}
	}
}

func (h *Hub) CreateRoom(name string) error {
	if _, exists := h.Rooms[name]; exists {
		return fmt.Errorf("room %q already exists", name)
	}
	h.Rooms[name] = &types.Room{
		Name:    name,
		Clients: make(map[string]bool),
	}
	return nil
}

func (h *Hub) JoinRoom(username, roomName string) error {
	room, ok := h.Rooms[roomName]
	if !ok {
		return fmt.Errorf("room %q does not exist", roomName)
	}
	room.Clients[username] = true
	if c, ok := h.Clients[username]; ok {
		c.Room = roomName
	}
	return nil
}

func (h *Hub) LeaveRoom(username string) error {
	c, ok := h.Clients[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	if c.Room == "" {
		return fmt.Errorf("not in any room")
	}

	roomName := c.Room
	if room, ok := h.Rooms[roomName]; ok {
		delete(room.Clients, username)
		h.broadcastSystem(roomName, fmt.Sprintf("%s left the room", username))
	}
	c.Room = ""
	return nil
}

func (h *Hub) ListRooms() []string {
	rooms := make([]string, 0, len(h.Rooms))
	for name := range h.Rooms {
		rooms = append(rooms, name)
	}
	return rooms
}

func (h *Hub) ListUsers(roomName string) []string {
	room, ok := h.Rooms[roomName]
	if !ok {
		return nil
	}
	users := make([]string, 0, len(room.Clients))
	for u := range room.Clients {
		users = append(users, u)
	}
	return users
}

func (h *Hub) ListOnline() []string {
	users := make([]string, 0, len(h.Clients))
	for u := range h.Clients {
		users = append(users, u)
	}
	return users
}
