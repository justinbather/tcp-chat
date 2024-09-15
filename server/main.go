package main

import (
	"bytes"
	"encoding/gob"
	"errors"
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

func init() {

	gob.Register(ChatMsg{})
	gob.Register(TcpCmd{})
	gob.Register(Message{})
}
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
		if client == nil {
			continue
		}

		go handleConn(client, l)
	}
}

// In the process of creating this master type, will need to be protected behind methods so we dont create
// a message with more than one inner messages
type Message struct {
	Type string
	TcpCmd
	ChatMsg
}

func NewTcpCmd(cmd string) Message {
	return Message{Type: "TcpCmd", TcpCmd: TcpCmd{Cmd: cmd}}
}

type TcpCmd struct {
	Cmd  string
	Data string
}

type ChatMsg struct {
	Content string
	Sender  string
	Id      int
}

func NewChatMsg(c string, s string, id int) Message {
	return Message{Type: "ChatMsg", ChatMsg: ChatMsg{Content: c, Sender: s, Id: id}}
}

type CmdResp struct {
	Username string
}

func sendAuthCmd(conn net.Conn) {
	fmt.Println("Sending Auth command")
	enc := gob.NewEncoder(conn)
	err := enc.Encode(NewTcpCmd("NEED_AUTH"))
	if err != nil {
		//err handling here
		fmt.Println("Error sending message: ", err)
	}
	fmt.Println("Sent client command to supply auth")

}

func sendAck(conn net.Conn) error {
	fmt.Println("Sending ACK")
	enc := gob.NewEncoder(conn)
	err := enc.Encode(NewTcpCmd("AUTH_ACK"))
	if err != nil {
		fmt.Println("Error sending ACK: ", err)
		return err
	}
	fmt.Println("Ack Sent, Client Authenticated")
	return nil
}

// TODO: This writing message is duplicated, refactor
func waitForAuth(conn net.Conn) (string, error) {
	fmt.Println("Waiting for Auth Response from Client")
	tmp := make([]byte, MAX_MESSAGE_SIZE)
	for {
		_, err := conn.Read(tmp)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Client ended connection, recieved EOF. Gracefully closing connection")
			} else {
				fmt.Println("error reading input: ", err)
			}
			return "", errors.New("Cant authenticate response")
		}

		buf := bytes.NewBuffer(tmp)
		dec := gob.NewDecoder(buf)
		msg := Message{}
		dec.Decode(&msg)
		fmt.Println(msg)

		//TODO: UGLY
		if msg.Type != "TcpCmd" || msg.TcpCmd.Cmd != "AUTH_RESP" || msg.TcpCmd.Data == "" {
			return "", errors.New("Cant authenticate response")
		}

		// Data is username string for now
		return msg.TcpCmd.Data, nil
	}

}

func processNewClient(l *Lobby, c net.Conn) *Client {
	//This total clients param is temporary until we have clients specifing the name
	client := NewClient(c)
	client.Name = "Justin"
	fmt.Println("Requesting client provides Auth")

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

func handshake(conn net.Conn) (string, error) {
	authData, err := waitForAuth(conn)
	if err != nil {
		fmt.Println("Error reading auth response")
		return "", err
	}
	err = sendAck(conn)
	if err != nil {
		fmt.Println("Error sending auth ack")
		return "", err
	}

	return authData, nil

}

func handleConn(c *Client, l *Lobby) {

	defer func() {
		fmt.Printf("\nHit defer func in handle conn, closing chans and conn\n")
		c.Conn.Close()
		fmt.Printf("closed connection for %s", c.Name)
	}()

	authData, err := handshake(c.Conn)
	if err != nil {
		fmt.Printf("Error completing tcp handshake. error: %s", err)
	}

	//TODO: Just sending username to test auth flow, this will change to getting user data from user and pass, returning all data to client
	c.Name = authData

	go writeOutput(c)
	go readInput(c, l)

	// OH, without the goroutines running we cant send anuthing silly
	// c.Outgoing <- ChatMsg{Sender: "Server", Content: "Hello", Id: 10}

	for msg := range c.Incoming {
		fmt.Printf("User %s sent: %s", msg.Sender, msg.Content)
		if len(msg.Content) > 0 {
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
		msg := Message{}
		dec.Decode(&msg)
		//msg.Sender = c.Name

		msg.Id = CURR_ID
		CURR_ID++

		if isCommand(msg) {
			fmt.Println("Received command: ", msg.ChatMsg.Content)
			processCommand(msg.ChatMsg.Content, c)
		} else {
			c.Incoming <- msg
		}
	}
}

func isCommand(m Message) bool {
	if len(m.ChatMsg.Content) == 0 {
		return false
	}

	if m.ChatMsg.Content[0] == '/' {
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

func (l *Lobby) Broadcast(msg Message) {
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
	Incoming chan Message
	Outgoing chan Message
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		Conn:     conn,
		Incoming: make(chan Message),
		Outgoing: make(chan Message),
	}
}

func (c *Client) closeChans() {
	close(c.Incoming)
	close(c.Outgoing)
}

func (c *Client) writeChatMsg(msg Message) error {
	enc := gob.NewEncoder(c.Conn)
	err := enc.Encode(msg)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) SendServerMessage(s string) error {
	fmt.Printf("Sending server message: %s", s)
	err := c.writeChatMsg(NewChatMsg(s, "Server", CURR_ID))

	if err != nil {
		return err
	}

	return nil
}
