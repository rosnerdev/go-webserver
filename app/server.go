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
	"io"
)

type Headers struct {
	Host string
	UserAgent string
	Accept string
	ContentLength string
	ContentType string
}

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
    requestLine, err := reader.ReadString('\n')
    if err != nil {
        log.Printf("Error reading connection: %v", err)
        return
    }

    path := getPath(requestLine)
    headers, err := getHeaders(reader)
    if err != nil {
        log.Printf("Error reading headers: %v", err)
        return
    }

    statusCode := getStatusCode(path)
    restResponse := getRestResponse(path, headers)

    response := fmt.Sprintf("HTTP/1.1 %s\r\n%s", statusCode, restResponse)
    if _, err := conn.Write([]byte(response)); err != nil {
        log.Printf("Error writing to connection: %v", err)
    }
}

func getPath(requestLine string) string {
    parts := strings.Split(strings.TrimSpace(requestLine), " ")
    if len(parts) < 2 {
        return ""
    }
    return parts[1]
}

func getHeaders(reader *bufio.Reader) (Headers, error) {
    headers := Headers{}
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            return headers, fmt.Errorf("error reading header line: %w", err)
        }

        // Remove trailing \r\n
        line = strings.TrimSpace(line)

        // Empty line means end of headers
        if line == "" {
            break
        }

        // Split the header line
        parts := strings.SplitN(line, ":", 2)
        if len(parts) == 2 {
            headerName := strings.TrimSpace(parts[0])
            headerValue := strings.TrimSpace(parts[1])

            switch strings.ToLower(headerName) {
            case "host":
                headers.Host = headerValue
            case "user-agent":
                headers.UserAgent = headerValue
            case "accept":
                headers.Accept = headerValue
            case "content-length":
                headers.ContentLength = headerValue
            case "content-type":
                headers.ContentType = headerValue
            }
        }
    }

    return headers, nil
}

func getStatusCode(path string) string {
    endpoints := []string{"/", "/echo", "/user-agent"}

    // Check if the path exactly matches any of the endpoints
    for _, endpoint := range endpoints {
        if path == endpoint || strings.HasPrefix(path, endpoint+"/") {
            return "200 OK"
        }
    }

    // If no match is found
    return "404 Not Found"
}

func getRestResponse(path string, headers Headers) string {
    switch {
    case path == "/":
        return "\r\n"
    case strings.HasPrefix(path, "/echo"):
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
    case path == "/user-agent":
        return fmt.Sprintf("Content-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(headers.UserAgent), headers.UserAgent)
    }
    return "\r\n"
}

