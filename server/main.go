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

/*
func init() {

		gob.Register(ChatMsg{})
		gob.Register(TcpCmd{})
		gob.Register(BaseMsg{})
	}
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

type BaseMsg struct {
	Type string
}

type TcpCmd struct {
	BaseMsg
	Cmd string
}

type CmdResp struct {
	Username string
}

func sendAuthCmd(conn net.Conn) {
	fmt.Println("Sending Auth command")
	enc := gob.NewEncoder(conn)
	err := enc.Encode(TcpCmd{BaseMsg: BaseMsg{Type: "CmdMsg"}, Cmd: "NEED_AUTH"})
	if err != nil {
		//err handling here
		fmt.Println("Error sending message: ", err)
	}
	fmt.Println("Sent client command to supply auth")

}

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
		msg := CmdResp{}
		dec.Decode(&msg)
		fmt.Println(msg)

		if msg.Username == "" {
			return "", errors.New("Cant authenticate response")
		}
		return msg.Username, nil
	}

}

func performHandshake(c *Client) error {

	fmt.Println("Starting auth handshake")
	sendAuthCmd(c.Conn)
	authData, err := waitForAuth(c.Conn)
	if err != nil {
		return err
	}

	c.Name = authData

	return nil
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

func handleConn(c *Client, l *Lobby) {

	defer func() {
		fmt.Printf("\nHit defer func in handle conn, closing chans and conn\n")
		c.Conn.Close()
		fmt.Printf("closed connection for %s", c.Name)
	}()

	// this is BUGGIN, fs up when we try to send a server message
	err := performHandshake(c)
	if err != nil {
		c.SendServerMessage("Authentication Error")
		c.closeChans()
		return
	}

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

func NewClient(conn net.Conn) *Client {
	return &Client{
		Conn:     conn,
		Incoming: make(chan ChatMsg),
		Outgoing: make(chan ChatMsg),
	}
}

func (c *Client) closeChans() {
	close(c.Incoming)
	close(c.Outgoing)
}

func (c *Client) writeChatMsg(msg ChatMsg) error {
	enc := gob.NewEncoder(c.Conn)
	err := enc.Encode(msg)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) SendServerMessage(s string) error {
	fmt.Printf("Sending server message: %s", s)
	err := c.writeChatMsg(ChatMsg{BaseMsg: BaseMsg{Type: "ChatMsg"}, Sender: "Server", Content: s, Id: 100})

	if err != nil {
		return err
	}

	return nil
}

type ChatMsg struct {
	BaseMsg
	Content string
	Sender  string
	Id      int
}
