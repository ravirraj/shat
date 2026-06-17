package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ravirraj/shat/internal/protocol"
	"github.com/ravirraj/shat/internal/types"
)

var (
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	messagesStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("62"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)

	dmStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("208"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	okStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("46"))

	typingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)
)

type Model struct {
	fc         *protocol.FrameConn
	messages   []string
	rooms      []string
	users      []string
	input      string
	username   string
	currentRoom string
	width      int
	height     int
	err        string
}

func New(fc *protocol.FrameConn, username string) Model {
	return Model{
		fc:       fc,
		username: username,
		messages: []string{},
		rooms:    []string{},
		users:    []string{},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.input != "" {
				m.handleInput()
				m.input = ""
			}
			return m, nil
		case tea.KeyBackspace, tea.KeyCtrlH:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil
		case tea.KeySpace:
			m.input += " "
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case ServerMsg:
		m.handleServerMessage(msg.Msg)
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	sidebarWidth := m.width / 4
	mainWidth := m.width - sidebarWidth - 4
	mainHeight := m.height - 4

	roomList := m.renderRoomList()
	userList := m.renderUserList()
	sidebar := sidebarStyle.
		Width(sidebarWidth - 4).
		Height(mainHeight).
		Render(roomList + "\n\n" + userList)

	msgContent := m.renderMessages()
	messages := messagesStyle.
		Width(mainWidth - 2).
		Height(mainHeight).
		Render(msgContent)

	top := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, messages)

	inputBar := inputStyle.
		Width(m.width - 4).
		Render(fmt.Sprintf("[%s] > %s", m.username, m.input))

	return top + "\n" + inputBar
}

func (m *Model) renderRoomList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Rooms"))
	b.WriteString("\n")
	if len(m.rooms) == 0 {
		b.WriteString(userStyle.Render("  (none)"))
	} else {
		for _, r := range m.rooms {
			prefix := "  "
			if r == m.currentRoom {
				prefix = "▸ "
			}
			b.WriteString(userStyle.Render(prefix + r))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *Model) renderUserList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Online"))
	b.WriteString("\n")
	if len(m.users) == 0 {
		b.WriteString(userStyle.Render("  (none)"))
	} else {
		for _, u := range m.users {
			b.WriteString(userStyle.Render("  " + u))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *Model) renderMessages() string {
	start := 0
	if len(m.messages) > 100 {
		start = len(m.messages) - 100
	}
	msgs := m.messages[start:]

	var b strings.Builder
	for _, msg := range msgs {
		if strings.HasPrefix(msg, "[error]") {
			b.WriteString(errorStyle.Render(msg[7:]))
		} else if strings.HasPrefix(msg, "[ok]") {
			b.WriteString(okStyle.Render(msg[4:]))
		} else if strings.HasPrefix(msg, "[system]") {
			b.WriteString(systemStyle.Render(msg[8:]))
		} else if strings.HasPrefix(msg, "[dm]") {
			b.WriteString(dmStyle.Render(msg[4:]))
		} else if strings.HasPrefix(msg, "[typing]") {
			b.WriteString(typingStyle.Render(msg[8:]))
		} else {
			b.WriteString(msgStyle.Render(msg))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) handleInput() {
	input := m.input
	if strings.HasPrefix(input, "/") {
		m.handleCommand(input)
		return
	}

	if m.currentRoom == "" {
		m.addMessage("[error] Join a room first: /join <room>")
		return
	}

	err := m.fc.WriteMsg(&types.Message{
		Type:    types.MsgChat,
		Payload: input,
	})
	if err != nil {
		m.addMessage(fmt.Sprintf("[error] send failed: %v", err))
	}
}

func (m *Model) handleCommand(input string) {
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch cmd {
	case "/quit":
		tea.Quit()
	case "/help":
		m.addMessage("[system] Commands: /help, /quit, /create, /join, /leave, /rooms, /online, /dm, /keyex, /rotate, /typing")
	case "/create":
		m.fc.WriteMsg(&types.Message{Type: types.MsgRoomCreate, Payload: arg})
	case "/join":
		m.fc.WriteMsg(&types.Message{Type: types.MsgRoomJoin, Payload: arg})
	case "/leave":
		m.fc.WriteMsg(&types.Message{Type: types.MsgRoomLeave})
	case "/rooms":
		m.fc.WriteMsg(&types.Message{Type: types.MsgRoomList})
	case "/online":
		m.fc.WriteMsg(&types.Message{Type: types.MsgUserList})
	case "/dm":
		dmParts := strings.SplitN(arg, " ", 2)
		if len(dmParts) < 2 {
			m.addMessage("[error] usage: /dm <user> <message>")
			return
		}
		m.fc.WriteMsg(&types.Message{
			Type:    types.MsgDM,
			To:      dmParts[0],
			Payload: dmParts[1],
		})
	case "/typing":
		m.fc.WriteMsg(&types.Message{Type: types.MsgTyping})
	default:
		m.addMessage(fmt.Sprintf("[error] unknown command: %s", cmd))
	}
}

func (m *Model) handleServerMessage(msg *types.Message) {
	ts := msg.Ts.Format("15:04:05")

	switch msg.Type {
	case types.MsgChat:
		prefix := msg.From
		if msg.From == m.username {
			prefix = "You"
		}
		m.addMessage(fmt.Sprintf("[%s] %s: %s", ts, prefix, msg.Payload))
	case types.MsgDM:
		m.addMessage(fmt.Sprintf("[dm][%s] [DM from %s] %s", ts, msg.From, msg.Payload))
	case types.MsgSystem:
		m.addMessage(fmt.Sprintf("[system][%s] * %s", ts, msg.Payload))
	case types.MsgTyping:
		if msg.From != m.username {
			m.addMessage(fmt.Sprintf("[typing][%s] %s is typing...", ts, msg.From))
		}
	case types.MsgPresence:
		if msg.Payload == "online" {
			found := false
			for _, u := range m.users {
				if u == msg.From {
					found = true
					break
				}
			}
			if !found && msg.From != m.username {
				m.users = append(m.users, msg.From)
			}
		} else {
			for i, u := range m.users {
				if u == msg.From {
					m.users = append(m.users[:i], m.users[i+1:]...)
					break
				}
			}
		}
		m.addMessage(fmt.Sprintf("[system][%s] %s is %s", ts, msg.From, msg.Payload))
	case types.MsgRoomListResp:
		if msg.Payload == "no rooms" || msg.Payload == "" {
			m.rooms = nil
		} else {
			m.rooms = strings.Split(msg.Payload, "\n")
		}
	case types.MsgUserListResp:
		if msg.Payload == "no users" || msg.Payload == "" {
			m.users = nil
		} else {
			m.users = strings.Split(msg.Payload, "\n")
		}
	case types.MsgOK:
		m.addMessage(fmt.Sprintf("[ok][%s] %s", ts, msg.Payload))
		if strings.Contains(msg.Payload, "joined room") {
			parts := strings.Split(msg.Payload, "\"")
			if len(parts) >= 2 {
				m.currentRoom = parts[1]
				found := false
				for _, r := range m.rooms {
					if r == m.currentRoom {
						found = true
						break
					}
				}
				if !found {
					m.rooms = append(m.rooms, m.currentRoom)
				}
				m.fc.WriteMsg(&types.Message{Type: types.MsgUserList})
			}
		}
		if strings.Contains(msg.Payload, "created and joined room") {
			parts := strings.Split(msg.Payload, "\"")
			if len(parts) >= 2 {
				m.currentRoom = parts[1]
				found := false
				for _, r := range m.rooms {
					if r == m.currentRoom {
						found = true
						break
					}
				}
				if !found {
					m.rooms = append(m.rooms, m.currentRoom)
				}
			}
		}
	case types.MsgError:
		m.addMessage(fmt.Sprintf("[error][%s] %s", ts, msg.Payload))
	case types.MsgKeyExchange:
		m.addMessage(fmt.Sprintf("[system][%s] * key exchange received from %s", ts, msg.From))
	default:
		m.addMessage(fmt.Sprintf("[%s] %s", ts, msg.Payload))
	}
}

func (m *Model) addMessage(msg string) {
	m.messages = append(m.messages, msg)
	if len(m.messages) > 500 {
		m.messages = m.messages[len(m.messages)-500:]
	}
}

type ServerMsg struct {
	Msg *types.Message
}

func ListenForMessages(fc *protocol.FrameConn) tea.Cmd {
	return func() tea.Msg {
		msg, err := fc.ReadMsg()
		if err != nil {
			return tea.Quit()
		}
		return ServerMsg{Msg: msg}
	}
}
