package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"io"
	"os"
)

type Headers struct {
	Host           string
	UserAgent      string
	Accept         string
	ContentLength  string
	ContentType    string
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

	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

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

	statusCode, restResponse := getResponse(path, headers)

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

		line = strings.TrimSpace(line)

		if line == "" {
			break
		}

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

func getResponse(path string, headers Headers) (string, string) {
	switch {
	case path == "/":
		return "200 OK", "\r\n"
	case strings.HasPrefix(path, "/echo"):
		return handleEcho(path)
	case path == "/user-agent":
		return handleUserAgent(headers)
	case strings.HasPrefix(path, "/files"):
		return handleFiles(path)
	}
	return "404 Not Found", "\r\n"
}

func handleEcho(path string) (string, string) {
	re := regexp.MustCompile(`^/echo(/(?P<toEcho>.*))?$`)
	match := re.FindStringSubmatch(path)
	if match != nil {
		toEchoIndex := re.SubexpIndex("toEcho")
		toEcho := match[toEchoIndex]
		if toEcho == "" {
			return "200 OK", "Content-Type: text/plain\r\nContent-Length: 0\r\n\r\n"
		}
		return "200 OK", fmt.Sprintf("Content-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho)
	}
	return "404 Not Found", "\r\n"
}

func handleUserAgent(headers Headers) (string, string) {
	return "200 OK", fmt.Sprintf("Content-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(headers.UserAgent), headers.UserAgent)
}

func handleFiles(path string) (string, string) {
	re := regexp.MustCompile(`^/files(/(?P<fileName>.*))?$`)
	match := re.FindStringSubmatch(path)
	if match != nil {
		fileNameIndex := re.SubexpIndex("fileName")
		fileName := match[fileNameIndex]
		if fileName == "" {
			return "200 OK", "Content-Type: text/plain\r\nContent-Length: 0\r\n\r\n"
		}

		file, err := os.Open("/tmp/data/codecrafters.io/http-server-tester/" + fileName)
		if err != nil {
			log.Println(err)
			return "404 Not Found", "\r\n"
		}
		defer file.Close()

		var fileContent string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fileContent += scanner.Text()
		}

		return fmt.Sprintf("200 OK"), fmt.Sprintf("Content-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", len(fileContent), fileContent)
	}
	return "404 Not Found", "\r\n"
}
