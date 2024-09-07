package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"regexp"
	"runtime"
	"bytes"
)

const (
	port         = ":4221"
	maxReadBytes = 1024
)

func main() {
	maxWorkers := runtime.GOMAXPROCS(0)
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to bind to port 4221: %v", err)
	}
	defer listener.Close()

	log.Printf("Listening on %s", port)

	// Create a buffered channel to limit the number of concurrent goroutines
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Acquire a slot from the semaphore
		semaphore <- struct{}{}
		wg.Add(1)

		go func(c net.Conn) {
			defer func() {
				wg.Done()
				<-semaphore // Release the slot back to the semaphore
			}()
			handleConnection(c)
		}(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	requestLine, isPrefix, err := reader.ReadLine()
	if err != nil {
		log.Printf("Error reading connection: %v", err)
		return
	}

	if isPrefix {
		log.Printf("Request line too long")
		return
	}

	path := getPath(requestLine)
	statusCode := getStatusCode(path)
	restResponse := getRestResponse(path)

	response := fmt.Sprintf("HTTP/1.1 %s\r\n%s", statusCode, restResponse)
	if _, err := conn.Write([]byte(response)); err != nil {
		log.Printf("Error writing to connection: %v", err)
	}
}

func getPath(requestLine []byte) string {
	parts := bytes.Split(requestLine, []byte(" "))
	if len(parts) < 2 {
		return ""
	}
	return string(parts[1])
}

func getStatusCode(path string) string {
	if path == "/" || strings.HasPrefix(path, "/echo") {
		return "200 OK"
	}
	return "404 Not Found"
}

func getRestResponse(path string) string {
    re := regexp.MustCompile(`^/echo(/(?P<toEcho>.*))?$`)
    match := re.FindStringSubmatch(path)

    if match != nil {
        toEchoIndex := re.SubexpIndex("toEcho")
        toEcho := match[toEchoIndex]

        if toEcho == "" {
            return "Content-Type: text/plain\r\nContent-Length: 0\r\n\r\n"
        }
        return fmt.Sprintf("Content-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho)
    }

    return "\r\n"
}
