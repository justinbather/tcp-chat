package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
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

	lobby, listen := initServer()
	defer listen.Close()

	kill := make(chan os.Signal, 1)
	signal.Notify(kill, os.Interrupt)

	go captureSigInt(kill)

	run(listen, lobby)
}

func initServer() (*Lobby, net.Listener) {
	lobby := NewLobby()
	lobby.Clients = []*Client{}
	listen, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println("Error initializing server: ", err)
		return nil, nil
	}

	fmt.Println("Listening on TCP:8000")

	return lobby, listen
}

func run(listen net.Listener, lobby *Lobby) {

	totalClients := 0
	for {

		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("error accepting connection: ", err)
			return
		}

		if totalClients >= MAX_CLIENTS {
			fmt.Println("Cant handle any more clients")
			return
		}

		fmt.Println("Accepted new connection")

		client := processNewClient(lobby, conn, totalClients)

		go handleConn(client, lobby)
	}
}

func processNewClient(l *Lobby, c net.Conn, totalClients int) *Client {
	client := NewClient(c, totalClients)
	fmt.Println("new client :", client)

	l.Clients = append(l.Clients, client)
	totalClients++

	return client
}

func captureSigInt(kill chan os.Signal) {
	<-kill
	fmt.Println("shutting down")
	os.Exit(1)
}

func handleConn(c *Client, l *Lobby) {

	defer func() {
		fmt.Printf("\nHit defer func in handle conn, closing chans and conn\n")
		c.Conn.Close()
		fmt.Printf("closed connection for %s", c.Name)
	}()

	// need to handle disconnect as well
	go writeOutput(c)
	go readInput(c, l)

	for msg := range c.Incoming {
		fmt.Printf("User %s sent: %v", c.Name, msg)
		l.broadcast(msg)
	}
}

func readInput(c *Client, l *Lobby) {

	tmp := make([]byte, MAX_MESSAGE_SIZE)
	for {
		_, err := c.Conn.Read(tmp)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Client ended connection, recieved EOF. Gracefully closing connection")
			} else {
				fmt.Println("error reading input: ", err)
			}
			l.removeClient(c)
			return
		}

		buf := bytes.NewBuffer(tmp)

		dec := gob.NewDecoder(buf)

		msg := Message{}

		dec.Decode(&msg)

		msg.Sender = c.Name

		c.Incoming <- msg
	}
	//need to visit closing these channels and who should do it
}

func removeClient(l *Lobby, c *Client) {
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

func (l *Lobby) removeClient(c *Client) {

	for idx, client := range l.Clients {
		if client.Name == c.Name {
			l.Clients = append(l.Clients[:idx], l.Clients[idx+1:]...)
		}
	}

	close(c.Incoming)
	close(c.Outgoing)
	fmt.Printf("Removed %s from the lobby", c.Name)
	return
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
