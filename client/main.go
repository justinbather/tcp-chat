package main

import (
	"bufio"
	"fmt"
	"io"
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

run:
	for {
		select {
		case <-kill:
			fmt.Println("Recieved SIGINT, quitting")
			close(input)
			panic("killing")

		case text, ok := <-input:
			fmt.Println("got message:", text)
			if !ok {
				fmt.Println("error sending message: ok", ok)
				break run
			}
			written, err := io.WriteString(conn, text+"\n")
			if err != nil {
				fmt.Println("error sending message: ", err)
				break run
			}

			fmt.Println("idk why its here, written -> ", written)
		}

	}
}
