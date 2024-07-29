package main

import (
	"net"
)

func main() {

	conn, err := net.Dial("tcp", ":8000")
	if err != nil {
		panic(err)
	}

	msg := []byte("Hello")
	conn.Write(msg)
}
