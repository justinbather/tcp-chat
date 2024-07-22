package main

import (
	"fmt"
	"net"
)

func main() {

	conn, err := net.Dial("tcp", ":8000")
	if err != nil {
		panic(err)
	}

	msg := []byte("Hello")
	conn.Write(msg)

	for {
		rec := make([]byte, 1024)
		conn.Read(rec)

		fmt.Printf("recieved -> %s", string(rec))
	}

}
