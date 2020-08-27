package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

type arguments struct {
	targetServer *string
	targetPort   *int

	enableSsl     *bool
	localBind     *string
	localBindPort *int
	remotePath    *string

	fakeHost *string
}

func main() {
	args := arguments{}

	args.localBind = flag.String("listen", "localhost", "Local address to bind on")
	args.localBindPort = flag.Int("listenPort", 42070, "Local port to bind on")

	args.targetServer = flag.String("target", "localhost", "Remote server")
	args.targetPort = flag.Int("port", 42069, "Remote server port")

	args.remotePath = flag.String("path", "/aohttp", "Remote path")
	args.enableSsl = flag.Bool("ssl", false, "Enables ssl")
	args.fakeHost = flag.String("host", "localhost", "Fake the host header")
	flag.Parse()

	fmt.Println("Waiting for connections...")

	listener, err := net.ListenTCP("tcp",
		&net.TCPAddr{
			IP:   net.ParseIP(*args.localBind),
			Port: *args.localBindPort,
		})

	if err != nil {
		fail("Failed to bind on %s: %s", *args.localBind, err)
	}
	var conn *net.TCPConn
	conn, err = listener.AcceptTCP()
	if err != nil {
		fail("Failed to accept on %s", *args.localBind, err)
	}
	defer conn.Close()
	remoteServer, err := net.ResolveIPAddr("ip", *args.targetServer)
	if err != nil {
		fail("Unable to resolve %s", *args.targetServer, err)
	}

	fmt.Printf("Connecting to %s...\n", remoteServer)
	var remoteConn *net.TCPConn

	remoteConn, err = net.DialTCP(
		"tcp",
		nil,
		&net.TCPAddr{
			IP:   remoteServer.IP,
			Port: *args.targetPort,
		},
	)

	if err != nil {
		fail("Failed to dial %s", remoteServer, err)
	}
	defer remoteConn.Close()
	if *args.enableSsl {
		sslClient := tls.Client(remoteConn, &tls.Config{
			ServerName: *args.targetServer,
		})
		sslClient.Handshake()
	}
	var host string
	if *args.fakeHost != "" {
		host = *args.fakeHost
	} else {
		host = *args.targetServer
	}

	const format = "GET %s HTTP/1.1\r\nUser-Agent: AoHTTP\r\nConnection: Upgrade\r\nUpgrade: aohttp\r\nHost: %s\r\n\r\n"
	_, err = io.WriteString(remoteConn, fmt.Sprintf(format, *args.remotePath, host))
	if err != nil {
		fail("Failed to send request to %s", err)
	}

	data := make([]byte, 1024)
	_ = remoteConn.SetDeadline(time.Now().Add(3 * time.Second))
	_ = remoteConn.SetReadBuffer(1024)
	_, err = remoteConn.Read(data)
	if err != nil {
		fail("Failed to read from %s", remoteServer, err)
	}

	stringData := string(data)
	if !strings.HasPrefix(stringData, "HTTP/1.1 101 Switching Protocols") {
		fail("Failed to switch protocols after http request", stringData)
	}
	_ = remoteConn.SetDeadline(time.Time{})

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

// Fails with specified message, give error as last argument, rest are formats
//noinspection GoDuplicate
func fail(message string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, message+": %s\n", args...)
	os.Exit(1)
}
