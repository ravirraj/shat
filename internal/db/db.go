package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS rooms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS room_members (
		user_id INTEGER NOT NULL,
		room_id INTEGER NOT NULL,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, room_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (room_id) REFERENCES rooms(id)
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		msg_type TEXT NOT NULL DEFAULT 'chat',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (room_id) REFERENCES rooms(id),
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS offline_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		msg_type TEXT NOT NULL DEFAULT 'chat',
		sender_id INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);
	`

	_, err := d.conn.Exec(schema)
	return err
}

func (d *DB) CreateUser(username, passwordHash string) error {
	_, err := d.conn.Exec(
		"INSERT INTO users (username, password_hash) VALUES (?, ?)",
		username, passwordHash,
	)
	return err
}

func (d *DB) GetUser(username string) (id int, passwordHash string, err error) {
	err = d.conn.QueryRow(
		"SELECT id, password_hash FROM users WHERE username = ?", username,
	).Scan(&id, &passwordHash)
	return
}

func (d *DB) UserExists(username string) bool {
	var count int
	d.conn.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
	return count > 0
}

func (d *DB) CreateRoom(name string) error {
	_, err := d.conn.Exec("INSERT INTO rooms (name) VALUES (?)", name)
	return err
}

func (d *DB) RoomExists(name string) bool {
	var count int
	d.conn.QueryRow("SELECT COUNT(*) FROM rooms WHERE name = ?", name).Scan(&count)
	return count > 0
}

func (d *DB) JoinRoom(username, roomName string) error {
	var userID, roomID int
	err := d.conn.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		return err
	}
	err = d.conn.QueryRow("SELECT id FROM rooms WHERE name = ?", roomName).Scan(&roomID)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(
		"INSERT OR IGNORE INTO room_members (user_id, room_id) VALUES (?, ?)",
		userID, roomID,
	)
	return err
}

func (d *DB) LeaveRoom(username, roomName string) error {
	var userID, roomID int
	err := d.conn.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		return err
	}
	err = d.conn.QueryRow("SELECT id FROM rooms WHERE name = ?", roomName).Scan(&roomID)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(
		"DELETE FROM room_members WHERE user_id = ? AND room_id = ?",
		userID, roomID,
	)
	return err
}

func (d *DB) StoreMessage(roomName, username, content, msgType string) error {
	var userID, roomID int
	err := d.conn.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		return err
	}
	err = d.conn.QueryRow("SELECT id FROM rooms WHERE name = ?", roomName).Scan(&roomID)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(
		"INSERT INTO messages (room_id, user_id, content, msg_type) VALUES (?, ?, ?, ?)",
		roomID, userID, content, msgType,
	)
	return err
}

func (d *DB) GetRoomMessages(roomName string, limit int) ([]Message, error) {
	var roomID int
	err := d.conn.QueryRow("SELECT id FROM rooms WHERE name = ?", roomName).Scan(&roomID)
	if err != nil {
		return nil, err
	}

	rows, err := d.conn.Query(`
		SELECT m.content, m.msg_type, u.username, m.created_at
		FROM messages m
		JOIN users u ON m.user_id = u.id
		WHERE m.room_id = ?
		ORDER BY m.created_at DESC
		LIMIT ?
	`, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var createdAt string
		if err := rows.Scan(&msg.Content, &msg.Type, &msg.From, &createdAt); err != nil {
			return nil, err
		}
		msg.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		messages = append(messages, msg)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

type Message struct {
	Content   string
	Type      string
	From      string
	CreatedAt time.Time
}

func (d *DB) QueueOfflineMessage(username, content, msgType, sender string) error {
	var userID int
	err := d.conn.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		return err
	}
	var senderID sql.NullInt64
	if sender != "" {
		var sid int
		err = d.conn.QueryRow("SELECT id FROM users WHERE username = ?", sender).Scan(&sid)
		if err == nil {
			senderID = sql.NullInt64{Int64: int64(sid), Valid: true}
		}
	}
	_, err = d.conn.Exec(
		"INSERT INTO offline_queue (user_id, content, msg_type, sender_id) VALUES (?, ?, ?, ?)",
		userID, content, msgType, senderID,
	)
	return err
}

func (d *DB) GetAndClearOfflineMessages(username string) ([]OfflineMessage, error) {
	var userID int
	err := d.conn.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID)
	if err != nil {
		return nil, err
	}

	rows, err := d.conn.Query(`
		SELECT oq.content, oq.msg_type, COALESCE(u.username, 'system'), oq.created_at
		FROM offline_queue oq
		LEFT JOIN users u ON oq.sender_id = u.id
		WHERE oq.user_id = ?
		ORDER BY oq.created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []OfflineMessage
	for rows.Next() {
		var msg OfflineMessage
		var createdAt string
		if err := rows.Scan(&msg.Content, &msg.Type, &msg.From, &createdAt); err != nil {
			return nil, err
		}
		msg.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		messages = append(messages, msg)
	}

	_, err = d.conn.Exec("DELETE FROM offline_queue WHERE user_id = ?", userID)
	return messages, err
}

type OfflineMessage struct {
	Content   string
	Type      string
	From      string
	CreatedAt time.Time
}

func (d *DB) ListRooms() ([]string, error) {
	rows, err := d.conn.Query("SELECT name FROM rooms ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		rooms = append(rooms, name)
	}
	return rooms, nil
}
