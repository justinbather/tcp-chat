package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
)

const (
	MAX_CLIENTS      int = 4
	MAX_MESSAGE_SIZE     = 1024
	HELP_MSG             = "\n/help: lists all available commands.\n/create [lobby name]: creates a lobby name with the given name.\n/join [lobby name]: joins an exisiting lobby with the given name, if it exists.\n"
)

var CURR_ID = 1

/*TODO:
* clean this tf up
* - Not A Priority maybe we can poll the server from the client in the background to detect a closingwith some sort of retry policy
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

	fmt.Println("Listening on TCP:8001")

	return lobby, listen
}

func run(listen net.Listener, l *Lobby) {
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("error accepting connection: ", err)
			return
		}

		if l.TotalClients >= MAX_CLIENTS {
			fmt.Println("Cant handle any more clients")
			return
		}

		fmt.Println("Accepted new connection")

		client := processNewClient(l, conn)

		go handleConn(client, l)
	}
}

func processNewClient(l *Lobby, c net.Conn) *Client {
	//This total clients param is temporary until we have clients specifing the name
	client := NewClient(c, l.TotalClients)
	fmt.Println("new client :", client)

	l.Clients = append(l.Clients, client)
	l.TotalClients++

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
	// mock repeated messages to clients
	//go func() {
	//for {
	//	time.Sleep(time.Second * 4)
	//	l.Broadcast(ChatMsg{Content: "Hello from server", Sender: "Server", Id: CURR_ID})
	//	CURR_ID++
	//}

	//}()

	for msg := range c.Incoming {
		fmt.Printf("User %s sent: %s", msg.Sender, msg.Content)
		if len(msg.Content) == 0 {
			fmt.Println("Stopped empty message from being sent by ", msg.Sender)
		} else {
			l.Broadcast(msg)
		}
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
			l.CancelClientConn(c)
			return
		}

		buf := bytes.NewBuffer(tmp)
		dec := gob.NewDecoder(buf)
		msg := ChatMsg{}
		dec.Decode(&msg)
		//msg.Sender = c.Name

		msg.Id = CURR_ID
		CURR_ID++

		if isCommand(msg) {
			fmt.Println("Received command: ", msg.Content)
			processCommand(msg.Content, c)
		} else {
			c.Incoming <- msg
		}
	}
}

func isCommand(m ChatMsg) bool {
	if len(m.Content) == 0 {
		return false
	}

	if m.Content[0] == '/' {
		return true
	}

	return false
}

func processCommand(cmd string, c *Client) {
	// parse command and do command
	bulkCmd := strings.Split(cmd, "/")
	switch bulkCmd[1] {
	case "new":
		fmt.Println("Creating new lobby")
		break

	case "help":
		c.SendServerMessage(HELP_MSG)
	default:
		c.SendServerMessage("Not a valid command. try /help for a list of commands")
		break

	}
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
}

type Lobby struct {
	Clients      []*Client
	TotalClients int
}

func NewLobby() *Lobby {
	return &Lobby{Clients: nil, TotalClients: 0}
}

func (l *Lobby) Broadcast(msg ChatMsg) {
	for _, c := range l.Clients {
		c.Outgoing <- msg
	}
}

func (l *Lobby) CancelClientConn(c *Client) {
	c.closeChans()
	l.removeClient(c)
	fmt.Printf("Removed %s from the lobby", c.Name)
}

func (l *Lobby) removeClient(c *Client) {
	for idx, client := range l.Clients {
		if client.Name == c.Name {
			l.Clients = append(l.Clients[:idx], l.Clients[idx+1:]...)
		}
	}
}

type Client struct {
	Name     string
	Conn     net.Conn
	Incoming chan ChatMsg
	Outgoing chan ChatMsg
}

func NewClient(conn net.Conn, i int) *Client {
	names := []string{"justin", "riley", "cooper", "anonymous"}
	return &Client{
		Name:     names[i],
		Conn:     conn,
		Incoming: make(chan ChatMsg),
		Outgoing: make(chan ChatMsg),
	}
}

func (c *Client) closeChans() {
	close(c.Incoming)
	close(c.Outgoing)
}

func (c *Client) SendServerMessage(s string) {
	msg := ChatMsg{Content: s, Sender: "Server", Id: CURR_ID}
	c.Outgoing <- msg
}

type ChatMsg struct {
	Content string
	Sender  string
	Id      int
}
