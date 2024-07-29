package main

import (
	"fmt"
	"io"
	"net"
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

	for {

		conn, err := listen.Accept()
		if err != nil {
			return
		}

		if totalClients+1 > MAX_CLIENTS {

			fmt.Println("Cant handle any more clients")
			return
		}

		client := NewClient(conn, totalClients)

		clients = append(clients, client)
		totalClients++

		handleConn(client)
	}

}

func handleConn(client *Client) {

	//need to wire up recieving message and putting it into other clients outgoing chan
	go writeOutput(client.Conn, client.Outgoing)
	go readInput(client.Conn, client.Incoming)
	fmt.Println("Go routines started for user", client.Name)
}

func readInput(c net.Conn, in chan<- string) {
	for {
		data, err := io.ReadAll(c)
		if err != nil {
			fmt.Println("error reading message", err)
			//error handling here
			break
		}
		msg := string(data)
		fmt.Printf("got message- %s\n", msg)
		in <- msg
	}
	//need to visit close these channels and who should do it
}

func writeOutput(c net.Conn, out <-chan string) {

	for {
		for msg := range out {
			_, err := io.WriteString(c, msg)
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
