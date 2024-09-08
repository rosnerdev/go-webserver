package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type Headers struct {
	Host           string
	UserAgent      string
	Accept         string
	ContentLength  string
	ContentType    string
	AcceptEncoding string
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
	requestLine, _, err := reader.ReadLine()
	if err != nil {
		log.Printf("Error reading connection: %v", err)
		return
	}

	method := getMethod(string(requestLine))
	path := getPath(string(requestLine))
	headers, err := getHeaders(reader)
	if err != nil {
		log.Printf("Error reading headers: %v", err)
		return
	}

	var body []byte
	if method == "POST" {
		contentLength, _ := strconv.Atoi(headers.ContentLength)
		body = make([]byte, contentLength)
		_, err = io.ReadFull(reader, body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			return
		}
	}

	var statusCode, restResponse string
	switch method {
	case "POST":
		statusCode, restResponse = postResponse(path, body)
	case "GET":
		statusCode, restResponse = getResponse(path, "", headers)
	default:
		statusCode, restResponse = "405 Method Not Allowed", "\r\n"
	}

	response := fmt.Sprintf("HTTP/1.1 %s\r\n%s", statusCode, restResponse)
	_, err = conn.Write([]byte(response))
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func getPath(requestLine string) string {
	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func getMethod(requestLine string) string {
	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
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
			case "accept-encoding":
				headers.AcceptEncoding = headerValue
			}
		}
	}

	return headers, nil
}

func postResponse(path string, body []byte) (string, string) {
	switch {
	case strings.HasPrefix(path, "/files"):
		return handleFiles(path, "POST", body)
	}
	return "404 Not Found", "\r\n"
}

func getResponse(path, lastLine string, headers Headers) (string, string) {
	switch {
	case path == "/":
		return "200 OK", "\r\n"
	case strings.HasPrefix(path, "/echo"):
		return handleEcho(path, headers)
	case path == "/user-agent":
		return handleUserAgent(headers)
	case strings.HasPrefix(path, "/files"):
		return handleFiles(path, "GET", []byte(lastLine))
	}
	return "404 Not Found", "\r\n"
}

func handleEcho(path string, headers Headers) (string, string) {
	re := regexp.MustCompile(`^/echo(/(?P<toEcho>.*))?$`)
	match := re.FindStringSubmatch(path)
	if match != nil {
		toEchoIndex := re.SubexpIndex("toEcho")
		toEcho := match[toEchoIndex]
		if toEcho == "" {
			return "200 OK", "Content-Type: text/plain\r\nContent-Length: 0\r\n\r\n"
		}

		if headers.AcceptEncoding == "gzip" {
			return "200 OK", fmt.Sprintf("Content-Type: text/plain\r\nContent-Encoding: gzip\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho)
		}

		return "200 OK", fmt.Sprintf("Content-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(toEcho), toEcho)
	}
	return "404 Not Found", "\r\n"
}

func handleUserAgent(headers Headers) (string, string) {
	return "200 OK", fmt.Sprintf("Content-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(headers.UserAgent), headers.UserAgent)
}

func handleFiles(path, method string, body []byte) (string, string) {
	re := regexp.MustCompile(`^/files(/(?P<fileName>.*))?$`)
	match := re.FindStringSubmatch(path)
	if match != nil {
		fileNameIndex := re.SubexpIndex("fileName")
		fileName := match[fileNameIndex]
		switch method {
		case "GET":
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

			return "200 OK", fmt.Sprintf("Content-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", len(fileContent), fileContent)
		case "POST":
			if fileName == "" {
				return "400 Bad Request", "\r\n"
			}
			filePath := "/tmp/data/codecrafters.io/http-server-tester/" + fileName
			if err := os.WriteFile(filePath, body, 0644); err == nil {
				return "201 Created", "\r\n"
			} else {
				log.Printf("Error writing file: %v", err)
				return "500 Internal Server Error", "\r\n"
			}
		}
	}
	return "404 Not Found", "\r\n"
}
