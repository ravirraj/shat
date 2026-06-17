package auth

import (
	"testing"

	"github.com/ravirraj/shat/internal/db"
)

func setupTestAuth(t *testing.T) *Auth {
	t.Helper()
	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return New(database)
}

func TestRegister(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	err := a.Register("alice", "password123")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
}

func TestRegisterDuplicate(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	a.Register("alice", "password123")
	err := a.Register("alice", "password456")
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegisterEmptyUsername(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	err := a.Register("", "password123")
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestRegisterEmptyPassword(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	err := a.Register("alice", "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestRegisterShortPassword(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	err := a.Register("alice", "ab")
	if err == nil {
		t.Fatal("expected error for short password")
	}
}

func TestLogin(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	a.Register("alice", "password123")

	token, err := a.Login("alice", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	a.Register("alice", "password123")

	_, err := a.Login("alice", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLoginNonExistentUser(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	_, err := a.Login("nobody", "password123")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestLoginEmptyCredentials(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	_, err := a.Login("", "password123")
	if err == nil {
		t.Fatal("expected error for empty username")
	}

	_, err = a.Login("alice", "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestLoginTokenUniqueness(t *testing.T) {
	a := setupTestAuth(t)
	defer a.db.Close()

	a.Register("alice", "password123")

	token1, _ := a.Login("alice", "password123")
	token2, _ := a.Login("alice", "password123")

	if token1 == token2 {
		t.Fatal("expected unique tokens for different login sessions")
	}
}
