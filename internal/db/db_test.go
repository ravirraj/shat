package db

import (
	"os"
	"testing"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return db
}

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := db.CreateUser("alice", "hash123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if !db.UserExists("alice") {
		t.Fatal("UserExists returned false for created user")
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("alice", "hash123")
	err := db.CreateUser("alice", "hash456")
	if err == nil {
		t.Fatal("expected error for duplicate user")
	}
}

func TestGetUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("alice", "hash123")

	id, hash, err := db.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
	if hash != "hash123" {
		t.Errorf("expected hash 'hash123', got %q", hash)
	}
}

func TestGetUserNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, _, err := db.GetUser("nobody")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestCreateRoom(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := db.CreateRoom("general")
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}

	if !db.RoomExists("general") {
		t.Fatal("RoomExists returned false for created room")
	}
}

func TestCreateRoomDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateRoom("general")
	err := db.CreateRoom("general")
	if err == nil {
		t.Fatal("expected error for duplicate room")
	}
}

func TestJoinLeaveRoom(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("alice", "hash123")
	db.CreateRoom("general")

	err := db.JoinRoom("alice", "general")
	if err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}

	err = db.LeaveRoom("alice", "general")
	if err != nil {
		t.Fatalf("LeaveRoom failed: %v", err)
	}
}

func TestStoreAndGetMessages(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("alice", "hash123")
	db.CreateRoom("general")

	err := db.StoreMessage("general", "alice", "hello world", "chat")
	if err != nil {
		t.Fatalf("StoreMessage failed: %v", err)
	}

	messages, err := db.GetRoomMessages("general", 10)
	if err != nil {
		t.Fatalf("GetRoomMessages failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", messages[0].Content)
	}
	if messages[0].From != "alice" {
		t.Errorf("expected from 'alice', got %q", messages[0].From)
	}
	if messages[0].Type != "chat" {
		t.Errorf("expected type 'chat', got %q", messages[0].Type)
	}
}

func TestGetRoomMessagesLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("alice", "hash123")
	db.CreateRoom("general")

	for i := 0; i < 10; i++ {
		db.StoreMessage("general", "alice", "msg", "chat")
	}

	messages, err := db.GetRoomMessages("general", 5)
	if err != nil {
		t.Fatalf("GetRoomMessages failed: %v", err)
	}

	if len(messages) != 5 {
		t.Errorf("expected 5 messages, got %d", len(messages))
	}
}

func TestOfflineQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("alice", "hash123")
	db.CreateUser("bob", "hash456")

	err := db.QueueOfflineMessage("bob", "hello", "dm", "alice")
	if err != nil {
		t.Fatalf("QueueOfflineMessage failed: %v", err)
	}

	messages, err := db.GetAndClearOfflineMessages("bob")
	if err != nil {
		t.Fatalf("GetAndClearOfflineMessages failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 offline message, got %d", len(messages))
	}

	if messages[0].Content != "hello" {
		t.Errorf("expected content 'hello', got %q", messages[0].Content)
	}
	if messages[0].From != "alice" {
		t.Errorf("expected from 'alice', got %q", messages[0].From)
	}

	messages, _ = db.GetAndClearOfflineMessages("bob")
	if len(messages) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(messages))
	}
}

func TestListRooms(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateRoom("general")
	db.CreateRoom("random")
	db.CreateRoom("dev")

	rooms, err := db.ListRooms()
	if err != nil {
		t.Fatalf("ListRooms failed: %v", err)
	}

	if len(rooms) != 3 {
		t.Errorf("expected 3 rooms, got %d", len(rooms))
	}
}

func TestUserExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if db.UserExists("alice") {
		t.Fatal("UserExists returned true for non-existent user")
	}

	db.CreateUser("alice", "hash123")

	if !db.UserExists("alice") {
		t.Fatal("UserExists returned false for existing user")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
