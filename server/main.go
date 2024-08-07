package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"os/signal"
)

const (
	MAX_CLIENTS      int = 4
	MAX_MESSAGE_SIZE     = 1024
)

// if we have a lobby which keeps an array of clients how should we keep the main thread open so we can accept new connections

// maybe we should create a thread for each client, and a thread for the lobby, so that if we get a message from a client on one thread
// we can send the message into the lobby channel, which then can put it into each of the connected clients channels too?

//or maybe just have the lobby loop over each client, writing to their conns

// TODO:
/*
* clean this tf up
* figure out gracefull closing of connections
* maybe we can poll the server from the client in the background to detect a closingwith some sort of retry policy

* work on broadcasting messages to all clients
 */
func main() {

	totalClients := 0

	lobby := NewLobby()
	lobby.Clients = []*Client{}
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
		for _, c := range lobby.Clients {
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

		lobby.Clients = append(lobby.Clients, client)
		totalClients++

		go handleConn(client, lobby)
	}
}

func handleConn(client *Client, l *Lobby) {

	fmt.Printf("Lobby: %+v", l)
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
		fmt.Printf("User %s sent: %v", client.Name, msg)
		l.broadcast(msg)
	}
}

func readInput(c *Client) {

	tmp := make([]byte, MAX_MESSAGE_SIZE)
	for {
		_, err := c.Conn.Read(tmp)
		if err != nil {
			fmt.Println("error reading input: ", err)
			return
		}
		fmt.Println(tmp)

		buf := bytes.NewBuffer(tmp)

		dec := gob.NewDecoder(buf)

		msg := Message{}

		dec.Decode(&msg)

		msg.Sender = c.Name

		c.Incoming <- msg
	}
	//need to visit closing these channels and who should do it
}

func writeOutput(c *Client) {

	for {
		for msg := range c.Outgoing {
			enc := gob.NewEncoder(c.Conn)
			err := enc.Encode(msg)
			if err != nil {
				//err handling here
				fmt.Println("Error sending message: ", err)
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

func (l *Lobby) broadcast(msg Message) {
	for _, c := range l.Clients {
		if c.Name != msg.Sender {
			c.Outgoing <- msg
		}
	}
}

func NewLobby() *Lobby {
	return &Lobby{Clients: nil}
}

type Client struct {
	Name     string
	Conn     net.Conn
	Incoming chan Message
	Outgoing chan Message
}

func NewClient(conn net.Conn, i int) *Client {
	names := []string{"justin", "riley", "cooper", "anonymous"}
	return &Client{
		Name:     names[i],
		Conn:     conn,
		Incoming: make(chan Message),
		Outgoing: make(chan Message),
	}
}

type Message struct {
	Content string
	Sender  string
}
