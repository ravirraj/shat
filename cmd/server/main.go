package main

import (
	"fmt"
	"os"

	"github.com/ravirraj/shat/internal/db"
	"github.com/ravirraj/shat/internal/hub"
	"github.com/ravirraj/shat/internal/server"
)

func main() {
	database, err := db.New("shat.db")
	if err != nil {
		fmt.Println("database error:", err)
		os.Exit(1)
	}
	defer database.Close()

	h := hub.NewHub()
	go h.Run()

	s := server.NewServer(":8000", h, database)
	s.Start()
}
