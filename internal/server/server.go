package server

import (
	"crypto/rsa"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ravirraj/shat/internal/auth"
	"github.com/ravirraj/shat/internal/client"
	scrypto "github.com/ravirraj/shat/internal/crypto"
	"github.com/ravirraj/shat/internal/db"
	"github.com/ravirraj/shat/internal/hub"
	"github.com/ravirraj/shat/internal/protocol"
	"github.com/ravirraj/shat/internal/ratelimit"
	"github.com/ravirraj/shat/internal/types"
)

type Server struct {
	Addr       string
	Hub        *hub.Hub
	DB         *db.DB
	Auth       *auth.Auth
	RateLimit  *ratelimit.Manager
	publicKeys map[string]*rsa.PublicKey
	mu         sync.RWMutex
}

func NewServer(addr string, h *hub.Hub, database *db.DB) *Server {
	return &Server{
		Addr:       addr,
		Hub:        h,
		DB:         database,
		Auth:       auth.New(database),
		RateLimit:  ratelimit.NewManager(10, 20),
		publicKeys: make(map[string]*rsa.PublicKey),
	}
}

func (s *Server) Start() {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		fmt.Println("listen error:", err)
		return
	}
	fmt.Printf("Server starting at %s\n", s.Addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	fc := protocol.NewFrameConn(conn)

	msg, err := fc.ReadMsg()
	if err != nil {
		fmt.Printf("handshake read error: %v\n", err)
		fc.Close()
		return
	}

	var username string
	switch msg.Type {
	case types.MsgAuth:
		if _, err := s.Auth.Login(msg.From, msg.Password); err != nil {
			fc.WriteMsg(&types.Message{
				Type:    types.MsgAuthError,
				Payload: err.Error(),
				Ts:      time.Now(),
			})
			fc.Close()
			return
		}
		username = msg.From

	case types.MsgRegister:
		if err := s.Auth.Register(msg.From, msg.Password); err != nil {
			fc.WriteMsg(&types.Message{
				Type:    types.MsgRegisterError,
				Payload: err.Error(),
				Ts:      time.Now(),
			})
			fc.Close()
			return
		}
		username = msg.From

	default:
		fc.WriteMsg(&types.Message{
			Type:    types.MsgAuthError,
			Payload: "expected AUTH or REGISTER message",
			Ts:      time.Now(),
		})
		fc.Close()
		return
	}

	if _, exists := s.Hub.Clients[username]; exists {
		fc.WriteMsg(&types.Message{
			Type:    types.MsgAuthError,
			Payload: "username already taken",
			Ts:      time.Now(),
		})
		fc.Close()
		return
	}

	fc.WriteMsg(&types.Message{
		Type:    types.MsgAuthOK,
		Payload: "authenticated",
		Ts:      time.Now(),
	})

	c := &client.Client{
		Name:           username,
		Conn:           fc,
		Send:           make(chan *types.Message, 256),
		RegisterChan:   s.Hub.RegisterChan,
		UnregisterChan: s.Hub.UnregisterChan,
		Broadcast:      s.Hub.Broadcast,
	}

	s.Hub.RegisterChan <- c

	go c.WriteLoop()
	s.handleClientMessages(c)
}

func (s *Server) handleClientMessages(c *client.Client) {
	var closeOnce sync.Once
	defer func() {
		closeOnce.Do(func() {
			s.Hub.UnregisterChan <- c
			c.Conn.Close()
		})
	}()

	s.deliverOfflineMessages(c)

	for {
		msg, err := c.Conn.ReadMsg()
		if err != nil {
			return
		}
		s.processMessage(c, msg)
	}
}

func (s *Server) processMessage(c *client.Client, msg *types.Message) {
	msg.From = c.Name
	msg.Ts = time.Now()

	if !s.RateLimit.Allow(c.Name) {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "rate limit exceeded, slow down",
			Ts:      time.Now(),
		})
		return
	}

	switch msg.Type {
	case types.MsgChat:
		s.handleChat(c, msg)
	case types.MsgDM:
		s.handleDM(c, msg)
	case types.MsgRoomCreate:
		s.handleRoomCreate(c, msg)
	case types.MsgRoomJoin:
		s.handleRoomJoin(c, msg)
	case types.MsgRoomLeave:
		s.handleRoomLeave(c, msg)
	case types.MsgRoomList:
		s.handleRoomList(c)
	case types.MsgUserList:
		s.handleUserList(c)
	case types.MsgTyping:
		s.handleTyping(c, msg)
	case types.MsgPublicKey:
		s.handlePublicKey(c, msg)
	case types.MsgKeyExchange:
		s.handleKeyExchange(c, msg)
	case types.MsgEncrypted:
		s.handleEncrypted(c, msg)
	default:
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: fmt.Sprintf("unknown message type: %s", msg.Type),
			Ts:      time.Now(),
		})
	}
}

func (s *Server) handleChat(c *client.Client, msg *types.Message) {
	if c.Room == "" {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "you are not in a room. Use /join <room> first",
			Ts:      time.Now(),
		})
		return
	}

	s.DB.StoreMessage(c.Room, c.Name, msg.Payload, "chat")

	s.Hub.Broadcast <- &types.RoomMessage{
		Room:    c.Room,
		Message: msg,
	}
}

func (s *Server) handleDM(c *client.Client, msg *types.Message) {
	if msg.To == "" {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "usage: /dm <user> <message>",
			Ts:      time.Now(),
		})
		return
	}

	if _, ok := s.Hub.Clients[msg.To]; !ok {
		s.DB.QueueOfflineMessage(msg.To, msg.Payload, "dm", c.Name)
		c.SendMessage(&types.Message{
			Type:    types.MsgSystem,
			Payload: fmt.Sprintf("%s is offline, message queued", msg.To),
			Ts:      time.Now(),
		})
		return
	}

	s.Hub.DMChan <- msg
}

func (s *Server) handleRoomCreate(c *client.Client, msg *types.Message) {
	roomName := strings.TrimSpace(msg.Payload)
	if roomName == "" {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "usage: /create <room>",
			Ts:      time.Now(),
		})
		return
	}

	if err := s.DB.CreateRoom(roomName); err != nil {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: fmt.Sprintf("room already exists or error: %v", err),
			Ts:      time.Now(),
		})
		return
	}

	s.Hub.CreateRoom(roomName)
	s.Hub.JoinRoom(c.Name, roomName)
	s.DB.JoinRoom(c.Name, roomName)

	c.SendMessage(&types.Message{
		Type:    types.MsgOK,
		Payload: fmt.Sprintf("created and joined room %q", roomName),
		Ts:      time.Now(),
	})
}

func (s *Server) handleRoomJoin(c *client.Client, msg *types.Message) {
	roomName := strings.TrimSpace(msg.Payload)
	if roomName == "" {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "usage: /join <room>",
			Ts:      time.Now(),
		})
		return
	}

	if err := s.Hub.JoinRoom(c.Name, roomName); err != nil {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: err.Error(),
			Ts:      time.Now(),
		})
		return
	}

	s.DB.JoinRoom(c.Name, roomName)
	s.Hub.Broadcast <- &types.RoomMessage{
		Room: roomName,
		Message: &types.Message{
			Type:    types.MsgSystem,
			Payload: fmt.Sprintf("%s joined the room", c.Name),
			From:    "system",
			Ts:      time.Now(),
		},
	}

	c.SendMessage(&types.Message{
		Type:    types.MsgOK,
		Payload: fmt.Sprintf("joined room %q", roomName),
		Ts:      time.Now(),
	})

	s.sendRoomListToAll()
	s.sendUserListToRoom(roomName)
}

func (s *Server) handleRoomLeave(c *client.Client, msg *types.Message) {
	roomName := c.Room
	if err := s.Hub.LeaveRoom(c.Name); err != nil {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: err.Error(),
			Ts:      time.Now(),
		})
		return
	}

	s.DB.LeaveRoom(c.Name, roomName)
	c.SendMessage(&types.Message{
		Type:    types.MsgOK,
		Payload: "left the room",
		Ts:      time.Now(),
	})
}

func (s *Server) handleRoomList(c *client.Client) {
	rooms := s.Hub.ListRooms()
	if len(rooms) == 0 {
		c.SendMessage(&types.Message{
			Type:    types.MsgRoomListResp,
			Payload: "no rooms available",
			Ts:      time.Now(),
		})
		return
	}
	c.SendMessage(&types.Message{
		Type:    types.MsgRoomListResp,
		Payload: strings.Join(rooms, "\n"),
		Ts:      time.Now(),
	})
}

func (s *Server) handleUserList(c *client.Client) {
	var users []string
	if c.Room == "" {
		users = s.Hub.ListOnline()
	} else {
		users = s.Hub.ListUsers(c.Room)
	}
	c.SendMessage(&types.Message{
		Type:    types.MsgUserListResp,
		Payload: strings.Join(users, "\n"),
		Ts:      time.Now(),
	})
}

func (s *Server) handleTyping(c *client.Client, msg *types.Message) {
	if c.Room == "" {
		return
	}
	s.Hub.Broadcast <- &types.RoomMessage{
		Room: c.Room,
		Message: &types.Message{
			Type: types.MsgTyping,
			From: c.Name,
			Room: c.Room,
			Ts:   time.Now(),
		},
	}
}

func (s *Server) deliverOfflineMessages(c *client.Client) {
	messages, err := s.DB.GetAndClearOfflineMessages(c.Name)
	if err != nil {
		fmt.Printf("error delivering offline messages for %s: %v\n", c.Name, err)
		return
	}

	for _, offline := range messages {
		c.SendMessage(&types.Message{
			Type:    types.MsgDM,
			From:    offline.From,
			Payload: offline.Content,
			Ts:      offline.CreatedAt,
		})
	}

	if len(messages) > 0 {
		c.SendMessage(&types.Message{
			Type:    types.MsgSystem,
			Payload: fmt.Sprintf("you have %d offline message(s)", len(messages)),
			Ts:      time.Now(),
		})
	}
}

func (s *Server) handleEncrypted(c *client.Client, msg *types.Message) {
	if c.Room == "" {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "you are not in a room. Use /join <room> first",
			Ts:      time.Now(),
		})
		return
	}

	s.DB.StoreMessage(c.Room, c.Name, "[encrypted]", "encrypted")

	s.Hub.Broadcast <- &types.RoomMessage{
		Room:    c.Room,
		Message: msg,
	}
}

func (s *Server) handlePublicKey(c *client.Client, msg *types.Message) {
	if len(msg.Data) == 0 {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "no public key provided",
			Ts:      time.Now(),
		})
		return
	}

	pubKey, err := scrypto.ParsePublicKey(msg.Data)
	if err != nil {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: fmt.Sprintf("invalid public key: %v", err),
			Ts:      time.Now(),
		})
		return
	}

	s.mu.Lock()
	s.publicKeys[c.Name] = pubKey
	s.mu.Unlock()

	c.SendMessage(&types.Message{
		Type:    types.MsgOK,
		Payload: "public key registered",
		Ts:      time.Now(),
	})
}

func (s *Server) handleKeyExchange(c *client.Client, msg *types.Message) {
	if msg.To == "" || len(msg.Data) == 0 {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: "usage: KEY_EXCHANGE with 'to' and 'data' fields",
			Ts:      time.Now(),
		})
		return
	}

	s.mu.RLock()
	targetKey, ok := s.publicKeys[msg.To]
	s.mu.RUnlock()

	if !ok {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: fmt.Sprintf("user %q has not registered a public key", msg.To),
			Ts:      time.Now(),
		})
		return
	}

	encryptedKey, err := scrypto.EncryptWithRSA(targetKey, msg.Data)
	if err != nil {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: fmt.Sprintf("key encryption failed: %v", err),
			Ts:      time.Now(),
		})
		return
	}

	target, ok := s.Hub.Clients[msg.To]
	if !ok {
		c.SendMessage(&types.Message{
			Type:    types.MsgError,
			Payload: fmt.Sprintf("user %q is not online", msg.To),
			Ts:      time.Now(),
		})
		return
	}

	target.SendMessage(&types.Message{
		Type:    types.MsgKeyExchange,
		From:    c.Name,
		To:      msg.To,
		Data:    encryptedKey,
		Ts:      time.Now(),
	})

	c.SendMessage(&types.Message{
		Type:    types.MsgOK,
		Payload: fmt.Sprintf("key exchange sent to %s", msg.To),
		Ts:      time.Now(),
	})
}

func (s *Server) sendRoomListToAll() {
	rooms := s.Hub.ListRooms()
	roomStr := strings.Join(rooms, "\n")
	if roomStr == "" {
		roomStr = "no rooms"
	}
	for _, c := range s.Hub.Clients {
		c.SendMessage(&types.Message{
			Type:    types.MsgRoomListResp,
			Payload: roomStr,
			Ts:      time.Now(),
		})
	}
}

func (s *Server) sendUserListToRoom(roomName string) {
	users := s.Hub.ListUsers(roomName)
	userStr := strings.Join(users, "\n")
	if userStr == "" {
		userStr = "no users"
	}
	for _, username := range users {
		if c, ok := s.Hub.Clients[username]; ok {
			c.SendMessage(&types.Message{
				Type:    types.MsgUserListResp,
				Payload: userStr,
				Ts:      time.Now(),
			})
		}
	}
}
