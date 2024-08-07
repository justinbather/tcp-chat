package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
)

const MAX_CLIENTS int = 4

// if we have a lobby which keeps an array of clients how should we keep the main thread open so we can accept new connections

// maybe we should create a thread for each client, and a thread for the lobby, so that if we get a message from a client on one thread
// we can send the message into the lobby channel, which then can put it into each of the connected clients channels too?

//or maybe just have the lobby loop over each client, writing to their conns

func main() {

	totalClients := 0

	clients := []*Client{}
	listen, err := net.Listen("tcp", ":8000")
	if err != nil {
		return
	}
	fmt.Println("Listening on TCP:8000")
	defer listen.Close()

	kill := make(chan os.Signal, 1)
	signal.Notify(kill, os.Interrupt)

	go func() {
		<-kill
		fmt.Println("shutting down")
		for _, c := range clients {
			c.Conn.Close()
		}
		listen.Close()
		os.Exit(1)
	}()

	for {

		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("error accepting connection: ", err)
			return
		}

		fmt.Println("Accepted new connection")

		if totalClients >= MAX_CLIENTS {
			fmt.Println("Cant handle any more clients")
			return
		}

		client := NewClient(conn, totalClients)
		fmt.Println("new client :", client)

		clients = append(clients, client)
		totalClients++

		go handleConn(client)
	}
}

func handleConn(client *Client) {

	defer func() {
		close(client.Incoming)
		close(client.Outgoing)
		client.Conn.Close()
		fmt.Printf("closed connection for %s", client.Name)
	}()
	//need to wire up recieving message and putting it into other clients outgoing chan

	// need to handle disconnect as well
	go writeOutput(client)
	go readInput(client)

	for msg := range client.Incoming {
		fmt.Printf("User %s sent: %s", client.Name, msg)
	}
}

func readInput(c *Client) {

	reader := bufio.NewReader(c.Conn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("error reading input: ", err)
			return
		}
		c.Incoming <- msg
	}
	//need to visit closing these channels and who should do it
}

func writeOutput(c *Client) {

	for {
		for msg := range c.Outgoing {
			_, err := io.WriteString(c.Conn, msg)
			if err != nil {
				//err handling here
				break
			}
			fmt.Println("wrote message to user")
		}
	}
	//need to visit close these channels and who should do it
}

type Lobby struct {
	Clients []*Client
}

func NewLobby() Lobby {
	return Lobby{Clients: nil}
}

type Client struct {
	Name     string
	Conn     net.Conn
	Incoming chan string
	Outgoing chan string
}

func NewClient(conn net.Conn, i int) *Client {
	names := []string{"justin", "riley", "cooper", "anonymous"}
	return &Client{
		Name:     names[i],
		Conn:     conn,
		Incoming: make(chan string),
		Outgoing: make(chan string),
	}
}
