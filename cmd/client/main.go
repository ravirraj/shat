package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	// "github.com/ravirraj/shat/internal/client"
)

func main() {

	args := os.Args
	if len(args) < 2 {
		fmt.Println("usage ./client (address of the server) ")
		return
	}

	conn, err := net.Dial("tcp", args[1])
	if err != nil {
		fmt.Println("err", err)
		return
	}

	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Ypur Name : ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	_, err = conn.Write([]byte(name))
	if err != nil {
		fmt.Println("Not able to send name to the server ", err)
		return
	}

	go func() {
		for {
			// msg, err := reader.ReadString('\n')
			// reader = bufio.NewReader(os.Stdin)
			// fmt.Printf("%v : ", name)

			msg, err := reader.ReadString('\n')

			if err != nil {
				return
			}

			// for {
			_, err = conn.Write([]byte(msg))
			if err != nil {
				os.Exit(0)
				return
			}
			// }
		}
	}()

	go func() {
		msg := make([]byte, 1024)

		for {
			n, err := conn.Read(msg)

			if err != nil {
				fmt.Println("not able to read message from server")
				os.Exit(0)
				return
			}

			incoming := string(msg[:n])
			if strings.HasPrefix(incoming, name+":") {
				incoming = strings.Replace(incoming, name+":", "You:", 1)
			}
			fmt.Print("\r")     // move cursor to start
			fmt.Print("\033[K") // clear line
			// fmt.Print(incoming) // print message
			// fmt.Print("You: ")  // reprint prompt
			fmt.Print(incoming)
		}

	}()

	select {}

}
