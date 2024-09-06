package main

import (
	"bytes"
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
	defer conn.Close()

	var statCode []byte

	buff := make([]byte, 1024)
	_, err = conn.Read(buff)
	if err != nil {
		log.Println("Error reading connection: ", err.Error())
		os.Exit(1)
	}

	var pathName []byte
	splitBuff := bytes.Split(buff, []byte(" "))
	if len(splitBuff) > 1 {
		pathName = splitBuff[1]
	}

	if len(pathName) == 1 {
		statCode = []byte("200 OK")
	} else {
		statCode = []byte("404 Not Found")
	}

	var buf bytes.Buffer
	buf.WriteString("HTTP/1.1 ")
	buf.Write(statCode)
	buf.WriteString("\r\n\r\n")
	toWrite := buf.Bytes()
	_, err = conn.Write(toWrite)
	if err != nil {
		log.Println("Error writing to connection: ", err.Error())
		os.Exit(1)
	}
}
