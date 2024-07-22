package main

import (
	"fmt"
	"net"
)

// if we have a lobby which keeps an array of clients how should we keep the main thread open so we can accept new connections

// maybe we should create a thread for each client, and a thread for the lobby, so that if we get a message from a client on one thread
// we can send the message into the lobby channel, which then can put it into each of the connected clients channels too?

//or maybe just have the lobby loop over each client, writing to their conns

func main() {

	lobby := NewLobby()
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

		client := NewClient(conn)

		lobby.Clients = append(lobby.Clients, client)

		go handleConn(client)
	}

}

func handleConn(client *Client) {

	//this works we get a message. next is to setup the client to read from std in and send to this server upon \r
	// after this we should work on sending messages to other connections
	for {
		message := make([]byte, 2048)
		_, err := client.Conn.Read(message)
		if err != nil {
			return
		}

		fmt.Printf("Recieved message -> %s", string(message))

	}

}

type Lobby struct {
	Clients []*Client
}

func NewLobby() Lobby {
	return Lobby{Clients: nil}
}

type Client struct {
	Name string
	Conn net.Conn
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		Name: "justin", Conn: conn,
	}
}
