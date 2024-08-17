package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
)

/*

TODO:
- [] Need to think about creating a fake profile, taking in a name to then start the flow of joining a lobby
- [] Fix sending connection on closed connection, check for EOF
*/

func main() {

	conn, err := net.Dial("tcp", ":8000")
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	input := make(chan string)

	kill := make(chan os.Signal, 1)
	signal.Notify(kill, os.Interrupt)

	go readKeyboardInput(input)

	go func() {
		tmp := make([]byte, 1024)
		for {
			_, err := conn.Read(tmp)
			if err != nil {
				if err == io.EOF {
					fmt.Printf("Lobby has been closed, quitting session")
					//TODO this is pretty ugly, probably a better way to do this
					kill <- os.Interrupt
				} else {
					fmt.Println("error reading incoming message: ", err)
				}
				return
			}

			buf := bytes.NewBuffer(tmp)

			dec := gob.NewDecoder(buf)

			msg := Message{}

			dec.Decode(&msg)
			fmt.Printf("\n%s:%s\n", msg.Sender, msg.Content)
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

func readKeyboardInput(input chan string) {

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input <- scanner.Text()
	}
}

type Message struct {
	Content string
	Sender  string
}
