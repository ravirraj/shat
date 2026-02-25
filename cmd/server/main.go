package main

import "net"

func main() {

	conn, err := net.Listen("tcp", ":8000")
	if err != nil {
		panic(err)
	}
	_ = conn
}
