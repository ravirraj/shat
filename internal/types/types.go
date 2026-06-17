package types

import "time"

type MsgType string

const (
	MsgAuth          MsgType = "AUTH"
	MsgAuthOK        MsgType = "AUTH_OK"
	MsgAuthError     MsgType = "AUTH_ERROR"
	MsgRegister      MsgType = "REGISTER"
	MsgRegisterOK    MsgType = "REGISTER_OK"
	MsgRegisterError MsgType = "REGISTER_ERROR"
	MsgChat          MsgType = "CHAT"
	MsgDM            MsgType = "DM"
	MsgRoomCreate    MsgType = "ROOM_CREATE"
	MsgRoomJoin      MsgType = "ROOM_JOIN"
	MsgRoomLeave     MsgType = "ROOM_LEAVE"
	MsgRoomList      MsgType = "ROOM_LIST"
	MsgRoomListResp  MsgType = "ROOM_LIST_RESP"
	MsgUserList      MsgType = "USER_LIST"
	MsgUserListResp  MsgType = "USER_LIST_RESP"
	MsgTyping        MsgType = "TYPING"
	MsgRead          MsgType = "READ"
	MsgKeyExchange   MsgType = "KEY_EXCHANGE"
	MsgKeyResponse   MsgType = "KEY_RESPONSE"
	MsgRotateKey     MsgType = "ROTATE_KEY"
	MsgPublicKey     MsgType = "PUBLIC_KEY"
	MsgEncrypted     MsgType = "ENCRYPTED"
	MsgError         MsgType = "ERROR"
	MsgOK            MsgType = "OK"
	MsgSystem        MsgType = "SYSTEM"
	MsgPresence      MsgType = "PRESENCE"
)

type Message struct {
	Type     MsgType   `json:"type"`
	ID       string    `json:"id,omitempty"`
	From     string    `json:"from,omitempty"`
	To       string    `json:"to,omitempty"`
	Room     string    `json:"room,omitempty"`
	Payload  string    `json:"payload,omitempty"`
	Password string    `json:"password,omitempty"`
	Data     []byte    `json:"data,omitempty"`
	Ts       time.Time `json:"ts"`
}

type User struct {
	Username string
	Password string
}

type Room struct {
	Name    string
	Clients map[string]bool
}

type ClientState struct {
	Username    string
	CurrentRoom string
	PublicKey   []byte
	AESKey      []byte
}

type RoomMessage struct {
	Room    string
	Message *Message
}
