package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/ravirraj/shat/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	db *db.DB
}

func New(database *db.DB) *Auth {
	return &Auth{db: database}
}

func (a *Auth) Register(username, password string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	if len(password) < 4 {
		return fmt.Errorf("password must be at least 4 characters")
	}

	if a.db.UserExists(username) {
		return fmt.Errorf("username %q already taken", username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return a.db.CreateUser(username, string(hash))
}

func (a *Auth) Login(username, password string) (string, error) {
	if username == "" || password == "" {
		return "", fmt.Errorf("username and password required")
	}

	_, hash, err := a.db.GetUser(username)
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
