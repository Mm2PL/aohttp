package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
)

type serverArguments struct {
	targetPort *int

	bindPort    *int
	bindAddress *string

	acceptPath   *string
	acceptMethod *string

	hidden        *bool
	upgradeTarget *string
}

func main() {
	args := serverArguments{}

	args.targetPort = flag.Int("port", 80, "Target port")

	args.bindAddress = flag.String("baddr", "localhost", "Bind address")
	args.bindPort = flag.Int("bport", 42069, "Bind port")

	args.acceptPath = flag.String("path", "/aohttp", "Only acceptable path CHANGE THIS!!!")
	args.acceptMethod = flag.String("method", "GET", "Only acceptable method")
	args.upgradeTarget = flag.String("upgrade", "aohttp", "Only acceptable upgrade target")

	args.hidden = flag.Bool("hidden", false, "Make the server attempt hide itself")
	flag.Parse()

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP(*args.bindAddress),
		Port: *args.bindPort,
	})
	if err != nil {
		fail("Failed to bind on %s:%d", *args.bindAddress, *args.bindPort)
	}
	fmt.Printf("Now listening for new HTTP connections on %s:%d\n", *args.bindAddress, *args.bindPort)

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to accept: %s\n", err)
			continue
		}
		go handleConnection(conn, args)
	}

}

// Fails with specified message, give error as last argument, rest are formats
//noinspection GoDuplicate
func fail(message string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, message+": %s\n", args...)
	os.Exit(1)
}

func handleConnection(conn *net.TCPConn, args serverArguments) {
	defer conn.CloseRead()
	defer conn.CloseWrite()
	bufferedReader := bufio.NewReader(conn)
	request, err := http.ReadRequest(bufferedReader)
	if err != nil {
		fmt.Printf("Bad request from %s: %s.\n", conn.RemoteAddr().String(), err)
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}
	fmt.Printf("%s %s [%s]\n", request.Method, request.URL, request.Host)
	if !strings.EqualFold(request.URL.String(), *args.acceptPath) {
		conn.Write([]byte("HTTP/1.1 404 Not found\r\n\r\n"))
		return
	}
	if request.Header.Get("Connection") != "Upgrade" {
		if *args.hidden {
			conn.Write([]byte("HTTP/1.1 404 Not found\r\n\r\n"))
		} else {
			conn.Write([]byte("HTTP/1.1 426 Upgrade required\r\n\r\n"))
		}
		return
	}
	if request.Header.Get("Upgrade") != *args.upgradeTarget {
		if *args.hidden {
			conn.Write([]byte("HTTP/1.1 404 Not found\r\n\r\n"))
		} else {
			conn.Write([]byte("HTTP/1.1 426 Upgrade required\r\n\r\n"))
		}
		return
	}
	remoteConn, err := net.DialTCP("tcp", nil, &net.TCPAddr{
		Port: *args.targetPort,
		IP: net.ParseIP("localhost"),
	})
	if err != nil {
		conn.Write([]byte("HTTP/1.1 500 Internal server error\r\n\r\n"))
		return
	}
	defer remoteConn.Close()

	conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n"))


	go func() {
		_, err = remoteConn.ReadFrom(conn)
		if err != nil {
			fail("readfrom error", err)
		}
	}()
	_, err = conn.ReadFrom(remoteConn)
	if err != nil {
		fail("readfrom error", err)
	}
}
