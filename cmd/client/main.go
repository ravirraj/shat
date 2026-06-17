package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	scrypto "github.com/ravirraj/shat/internal/crypto"
	"github.com/ravirraj/shat/internal/protocol"
	"github.com/ravirraj/shat/internal/tui"
	"github.com/ravirraj/shat/internal/types"
)

type clientState struct {
	keyPair       *scrypto.KeyPair
	sessionKeys   map[string][]byte
	encryptedWith map[string]bool
	mu            sync.RWMutex
}

func main() {
	args := os.Args

	useTUI := true
	serverAddr := ""
	cliMode := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--cli":
			cliMode = true
			useTUI = false
		case "--tui":
			useTUI = true
			cliMode = false
		case "--help", "-h":
			fmt.Println("usage: ./client [flags] <server-address>")
			fmt.Println("flags:")
			fmt.Println("  --tui    use TUI mode (default)")
			fmt.Println("  --cli    use CLI mode")
			fmt.Println("  --help   show this help")
			return
		default:
			if !strings.HasPrefix(args[i], "-") {
				serverAddr = args[i]
			}
		}
	}

	if serverAddr == "" {
		fmt.Println("usage: ./client <server-address>")
		fmt.Println("example: ./client localhost:8000")
		return
	}

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("connection error:", err)
		return
	}
	defer conn.Close()

	fc := protocol.NewFrameConn(conn)
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	if username == "" {
		fmt.Println("username cannot be empty")
		return
	}

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	fmt.Print("(r)egister or (l)ogin? ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(strings.ToLower(choice))

	var msgType types.MsgType
	switch choice {
	case "r", "register":
		msgType = types.MsgRegister
	case "l", "login", "":
		msgType = types.MsgAuth
	default:
		fmt.Println("invalid choice, defaulting to login")
		msgType = types.MsgAuth
	}

	err = fc.WriteMsg(&types.Message{
		Type:     msgType,
		From:     username,
		Password: password,
		Ts:       time.Now(),
	})
	if err != nil {
		fmt.Println("auth send error:", err)
		return
	}

	resp, err := fc.ReadMsg()
	if err != nil {
		fmt.Println("auth response error:", err)
		return
	}

	if resp.Type == types.MsgAuthError || resp.Type == types.MsgRegisterError {
		fmt.Println("auth failed:", resp.Payload)
		return
	}

	keyPair, err := scrypto.GenerateKeyPair(2048)
	if err != nil {
		fmt.Println("key generation error:", err)
		return
	}

	pubKeyBytes, err := keyPair.MarshalPublicKey()
	if err != nil {
		fmt.Println("public key marshal error:", err)
		return
	}

	err = fc.WriteMsg(&types.Message{
		Type: types.MsgPublicKey,
		Data: pubKeyBytes,
		Ts:   time.Now(),
	})
	if err != nil {
		fmt.Println("public key send error:", err)
		return
	}

	state := &clientState{
		keyPair:       keyPair,
		sessionKeys:   make(map[string][]byte),
		encryptedWith: make(map[string]bool),
	}

	if useTUI && !cliMode {
		runTUI(fc, username, state)
	} else {
		runCLI(fc, reader, username, state)
	}
}

func runTUI(fc *protocol.FrameConn, username string, state *clientState) {
	model := tui.New(fc, username)
	p := tea.NewProgram(model, tea.WithAltScreen())

	go func() {
		for {
			msg, err := fc.ReadMsg()
			if err != nil {
				p.Quit()
				return
			}
			p.Send(tui.ServerMsg{Msg: msg})
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("TUI error:", err)
		os.Exit(1)
	}
}

func runCLI(fc *protocol.FrameConn, reader *bufio.Reader, name string, state *clientState) {
	fmt.Println("connected! Type /help for commands")

	go receiveMessages(fc, name, state)
	sendMessages(fc, reader, name, state)
}

func sendMessages(fc *protocol.FrameConn, reader *bufio.Reader, name string, state *clientState) {
	for {
		fmt.Printf("[%s] > ", name)
		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			handleCommand(fc, input, state)
			continue
		}

		state.mu.RLock()
		encrypted := state.encryptedWith[name]
		aesKey := state.sessionKeys[name]
		state.mu.RUnlock()

		if encrypted && aesKey != nil {
			encryptedData, err := scrypto.EncryptAES(aesKey, []byte(input))
			if err != nil {
				fmt.Println("encryption error:", err)
				continue
			}
			err = fc.WriteMsg(&types.Message{
				Type: types.MsgEncrypted,
				Data: encryptedData,
				Ts:   time.Now(),
			})
			if err != nil {
				fmt.Println("send error:", err)
				return
			}
			continue
		}

		err = fc.WriteMsg(&types.Message{
			Type:    types.MsgChat,
			Payload: input,
			Ts:      time.Now(),
		})
		if err != nil {
			fmt.Println("send error:", err)
			return
		}
	}
}

func handleCommand(fc *protocol.FrameConn, input string, state *clientState) {
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch cmd {
	case "/quit":
		fmt.Println("bye!")
		os.Exit(0)
	case "/help":
		printHelp()
	case "/create":
		fc.WriteMsg(&types.Message{Type: types.MsgRoomCreate, Payload: arg, Ts: time.Now()})
	case "/join":
		fc.WriteMsg(&types.Message{Type: types.MsgRoomJoin, Payload: arg, Ts: time.Now()})
	case "/leave":
		fc.WriteMsg(&types.Message{Type: types.MsgRoomLeave, Ts: time.Now()})
	case "/rooms":
		fc.WriteMsg(&types.Message{Type: types.MsgRoomList, Ts: time.Now()})
	case "/online":
		fc.WriteMsg(&types.Message{Type: types.MsgUserList, Ts: time.Now()})
	case "/dm":
		dmParts := strings.SplitN(arg, " ", 2)
		if len(dmParts) < 2 {
			fmt.Println("usage: /dm <user> <message>")
			return
		}
		fc.WriteMsg(&types.Message{
			Type:    types.MsgDM,
			To:      dmParts[0],
			Payload: dmParts[1],
			Ts:      time.Now(),
		})
	case "/typing":
		fc.WriteMsg(&types.Message{Type: types.MsgTyping, Ts: time.Now()})
	case "/keyex":
		if arg == "" {
			fmt.Println("usage: /keyex <user>")
			return
		}
		handleKeyExchange(fc, arg, state)
	case "/rotate":
		if arg == "" {
			fmt.Println("usage: /rotate <user>")
			return
		}
		handleKeyExchange(fc, arg, state)
	default:
		fmt.Printf("unknown command: %s (type /help for commands)\n", cmd)
	}
}

func handleKeyExchange(fc *protocol.FrameConn, targetUser string, state *clientState) {
	aesKey, err := scrypto.GenerateAESKey()
	if err != nil {
		fmt.Println("AES key generation error:", err)
		return
	}

	state.mu.Lock()
	state.sessionKeys[targetUser] = aesKey
	state.encryptedWith[targetUser] = true
	state.mu.Unlock()

	err = fc.WriteMsg(&types.Message{
		Type: types.MsgKeyExchange,
		To:   targetUser,
		Data: aesKey,
		Ts:   time.Now(),
	})
	if err != nil {
		fmt.Println("key exchange error:", err)
		return
	}

	fmt.Printf("key exchange initiated with %s (AES key sent)\n", targetUser)
}

func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  /help              - show this help")
	fmt.Println("  /quit              - disconnect")
	fmt.Println("  /create <room>     - create a new room")
	fmt.Println("  /join <room>       - join a room")
	fmt.Println("  /leave             - leave current room")
	fmt.Println("  /rooms             - list all rooms")
	fmt.Println("  /online            - list online users")
	fmt.Println("  /dm <user> <msg>   - send direct message")
	fmt.Println("  /typing            - send typing indicator")
	fmt.Println("  /keyex <user>      - initiate key exchange (enables E2E encryption)")
	fmt.Println("  /rotate <user>     - rotate encryption key")
}

func receiveMessages(fc *protocol.FrameConn, name string, state *clientState) {
	for {
		msg, err := fc.ReadMsg()
		if err != nil {
			fmt.Println("\ndisconnected from server")
			os.Exit(0)
		}
		printMessage(msg, name, state)
	}
}

func printMessage(msg *types.Message, name string, state *clientState) {
	ts := msg.Ts.Format("15:04:05")

	switch msg.Type {
	case types.MsgChat:
		prefix := msg.From
		if msg.From == name {
			prefix = "You"
		}
		fmt.Printf("\r\033[K[%s] %s: %s\n", ts, prefix, msg.Payload)
	case types.MsgEncrypted:
		state.mu.RLock()
		aesKey := state.sessionKeys[msg.From]
		state.mu.RUnlock()

		if aesKey != nil {
			plaintext, err := scrypto.DecryptAES(aesKey, msg.Data)
			if err != nil {
				fmt.Printf("\r\033[K[%s] [encrypted from %s] (decryption failed: %v)\n", ts, msg.From, err)
			} else {
				prefix := msg.From
				if msg.From == name {
					prefix = "You"
				}
				fmt.Printf("\r\033[K[%s] %s: %s [encrypted]\n", ts, prefix, string(plaintext))
			}
		} else {
			fmt.Printf("\r\033[K[%s] [encrypted message from %s] (no session key)\n", ts, msg.From)
		}
	case types.MsgDM:
		fmt.Printf("\r\033[K[%s] [DM from %s] %s\n", ts, msg.From, msg.Payload)
	case types.MsgKeyExchange:
		state.mu.Lock()
		aesKey, err := scrypto.DecryptWithRSA(state.keyPair.PrivateKey, msg.Data)
		if err != nil {
			fmt.Printf("\r\033[K[%s] key exchange from %s failed: %v\n", ts, msg.From, err)
		} else {
			state.sessionKeys[msg.From] = aesKey
			state.encryptedWith[msg.From] = true
			fmt.Printf("\r\033[K[%s] * key exchange with %s complete (E2E encryption enabled)\n", ts, msg.From)
		}
		state.mu.Unlock()
	case types.MsgSystem:
		fmt.Printf("\r\033[K[%s] * %s\n", ts, msg.Payload)
	case types.MsgTyping:
		if msg.From != name {
			fmt.Printf("\r\033[K[%s] %s is typing...\n", ts, msg.From)
		}
	case types.MsgPresence:
		status := "online"
		if msg.Payload == "offline" {
			status = "offline"
		}
		fmt.Printf("\r\033[K[%s] %s is %s\n", ts, msg.From, status)
	case types.MsgRoomListResp, types.MsgUserListResp:
		fmt.Printf("\r\033[K%s\n", msg.Payload)
	case types.MsgOK:
		fmt.Printf("\r\033[K[%s] %s\n", ts, msg.Payload)
	case types.MsgError:
		fmt.Printf("\r\033[K[%s] error: %s\n", ts, msg.Payload)
	default:
		fmt.Printf("\r\033[K[%s] %s\n", ts, msg.Payload)
	}
}
