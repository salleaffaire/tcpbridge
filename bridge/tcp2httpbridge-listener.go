package bridge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type TCP2HTTPBridgeListener struct {
	TCPPort     int
	HTTPPortIn  int
	HTTPPortOut int

	httpServer *http.Server
	conn       *net.Conn

	wg sync.WaitGroup
}

func NewTCP2HTTPBridgeListener(tcpPort, httpPortIn, httpPortOut int) *TCP2HTTPBridgeListener {
	bridge := &TCP2HTTPBridgeListener{
		TCPPort:     tcpPort,
		HTTPPortIn:  httpPortIn,
		HTTPPortOut: httpPortOut,
	}

	// HTTP Server for incoming data
	bridge.httpServer = &http.Server{Addr: fmt.Sprintf(":%d", bridge.HTTPPortIn)}

	// Start the HTTP server on port httpPortIn
	bridge.wg.Add(1)
	go bridge.startHTTPServer(&bridge.wg)

	// Channel to signal shutdown
	shutdown := make(chan struct{})
	var shutdownOnce sync.Once

	// Start the TCP listener on portX
	bridge.wg.Add(1)
	go bridge.startTCPListener(shutdown, &bridge.wg)

	// Wait for interrupt signal to gracefully shut down
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan // Block until a signal is received

	log.Println("Shutdown signal received. Initiating graceful shutdown...")

	// Signal shutdown to TCP listener
	shutdownOnce.Do(func() { close(shutdown) })

	// Shutdown the HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bridge.httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	return bridge
}

func (b *TCP2HTTPBridgeListener) Wait() {
	b.wg.Wait()
	fmt.Println("All goroutines finished. Exiting.")
	b.httpServer.Close()
}

func (b *TCP2HTTPBridgeListener) startHTTPServer(wg *sync.WaitGroup) {
	defer wg.Done()

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}
		// Simply echo the received data
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()
		log.Printf("Received HTTP data: %s", string(body))
		fmt.Fprintf(w, "Data received: %s", string(body))

		// Send the data to the TCP server
		b.sendTCPData(body)
	})

	log.Printf("HTTP server running on port %d", b.HTTPPortIn)
	if err := b.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func (b *TCP2HTTPBridgeListener) sendTCPData(data []byte) {
	// Send data on b.conn
	if b.conn == nil {
		log.Println("No TCP connection available")
		return
	}
	_, err := (*b.conn).Write(data)
	if err != nil {
		log.Printf("Failed to send data to TCP server: %v", err)
	}
}

func (b *TCP2HTTPBridgeListener) startTCPListener(shutdown <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", b.TCPPort))
	if err != nil {
		log.Fatalf("Failed to start TCP listener on port %d: %v", b.TCPPort, err)
	}
	defer listener.Close()
	log.Printf("TCP listener running on port %d", b.TCPPort)

	for {
		connChan := make(chan net.Conn)
		errChan := make(chan error)

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				errChan <- err
				return
			}
			connChan <- conn
		}()

		select {
		case <-shutdown:
			log.Println("Shutting down TCP listener.")
			return
		case err := <-errChan:
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				log.Printf("Temporary error accepting connection: %v", err)
				continue
			}
			log.Printf("Failed to accept connection: %v", err)
		case conn := <-connChan:
			log.Printf("Accepted connection from %s", conn.RemoteAddr())
			b.conn = &conn
			go b.handleTCPConnection()
		}
	}
}

func (b *TCP2HTTPBridgeListener) handleTCPConnection() {
	defer (*b.conn).Close()

	for {
		buffer := make([]byte, 1024)
		n, err := (*b.conn).Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from TCP connection: %v", err)
			} else {
				log.Printf("EOF: %v", err)
			}
			break
		}
		if n > 0 {
			data := buffer[:n]
			log.Printf("Received TCP data: %s", string(data))

			// Send the data as an HTTP POST request to portY
			b.sendHTTPData(data)
		}
	}
	log.Printf("Connection from %s closed", (*b.conn).RemoteAddr())
}

func (b *TCP2HTTPBridgeListener) sendHTTPData(data []byte) {
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/data", b.HTTPPortOut), "application/octet-stream",
		bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to send HTTP request: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("HTTP response status: %s", resp.Status)
}
