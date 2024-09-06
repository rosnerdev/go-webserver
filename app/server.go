package main

import (
	"log"
	"net"
	"os"
)

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		log.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	conn, err := l.Accept()
	if err != nil {
		log.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}

	toWrite := []byte("HTTP/1.1 200 OK\r\n\r\n")
	_, err = conn.Write(toWrite)
	if err != nil {
		log.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}
	defer conn.Close()
}
