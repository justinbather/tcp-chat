package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"os/signal"
)

func main() {

	conn, err := net.Dial("tcp", ":8000")
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	kill := make(chan os.Signal, 1)
	signal.Notify(kill, os.Interrupt)

	input := make(chan string)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input <- scanner.Text()
		}
	}()

	go func() {
		tmp := make([]byte, 1024)
		for {
			_, err := conn.Read(tmp)
			if err != nil {
				fmt.Println("error reading incoming message: ", err)
				return
			}

			buf := bytes.NewBuffer(tmp)

			dec := gob.NewDecoder(buf)

			msg := Message{}

			dec.Decode(&msg)
			fmt.Printf("recieved message: %s from %s\n", msg.Content, msg.Sender)
		}
	}()

run:
	for {
		select {
		case <-kill:
			fmt.Println("Recieved SIGINT, quitting")
			close(input)
			panic("killing")

		case text, ok := <-input:
			if !ok {
				fmt.Println("error sending message: ok", ok)
				break run
			}
			msg := Message{Content: text}
			enc := gob.NewEncoder(conn)
			err := enc.Encode(msg)
			if err != nil {
				fmt.Println("error sending message: ", err)
				break run
			}
		}

	}
}

type Message struct {
	Content string
	Sender  string
}
