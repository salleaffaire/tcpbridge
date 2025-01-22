package bridge

import (
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

type HTTP2TCPBridge struct {
	HTTPPort int
	TCPPort  int

	httpServer *http.Server

	tcpAddress  string
	tcpConn     net.Conn
	tcpConnLock sync.Mutex

	wg sync.WaitGroup
}

func NewHTTP2TCPBridge(httpPort, tcpPort int) *HTTP2TCPBridge {
	bridge := &HTTP2TCPBridge{
		HTTPPort:   httpPort,
		TCPPort:    tcpPort,
		tcpAddress: fmt.Sprintf("localhost:%d", tcpPort),
		httpServer: &http.Server{Addr: fmt.Sprintf(":%d", httpPort)},
	}

	// Establish a persistent TCP connection
	if err := bridge.connectToTCPServer(); err != nil {
		log.Fatalf("Failed to connect to TCP server: %v", err)
	}

	// Start the HTTP server to receive data
	bridge.wg.Add(1)
	go bridge.startHTTPServer(bridge.httpServer, &bridge.wg)

	// Wait for interrupt signal to gracefully shut down
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan // Block until a signal is received

	log.Println("Shutdown signal received. Initiating graceful shutdown...")

	// Shutdown the HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bridge.httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	// Close the persistent TCP connection
	bridge.closeTCPConnection()

	return bridge
}

func (b *HTTP2TCPBridge) Wait() {
	b.wg.Wait()
	fmt.Println("All goroutines finished. Exiting.")
	b.httpServer.Close()
}

func (b *HTTP2TCPBridge) startHTTPServer(server *http.Server, wg *sync.WaitGroup) {
	defer wg.Done()

	http.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		log.Printf("Received HTTP data: %s", string(body))

		// Forward the data to the TCP server
		err = b.sendToTCPServer(body)
		if err != nil {
			http.Error(w, "Failed to forward data to TCP server", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Data forwarded to TCP server")
	})

	log.Printf("HTTP server running on port %d", b.HTTPPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func (b *HTTP2TCPBridge) connectToTCPServer() error {
	b.tcpConnLock.Lock()
	defer b.tcpConnLock.Unlock()

	for {
		conn, err := net.Dial("tcp", b.tcpAddress)
		if err != nil {
			log.Printf("Failed to connect to TCP server at %s: %v. Retrying in 2 seconds...", b.tcpAddress, err)
			time.Sleep(2 * time.Second) // Wait before retrying
			continue
		}
		b.tcpConn = conn
		log.Printf("Successfully connected to TCP server at %s", b.tcpAddress)
		return nil
	}
}

func (b *HTTP2TCPBridge) closeTCPConnection() {
	b.tcpConnLock.Lock()
	defer b.tcpConnLock.Unlock()

	if b.tcpConn != nil {
		b.tcpConn.Close()
		log.Println("Closed connection to TCP server")
		b.tcpConn = nil
	}
}

func (b *HTTP2TCPBridge) sendToTCPServer(data []byte) error {
	b.tcpConnLock.Lock()
	defer b.tcpConnLock.Unlock()

	if b.tcpConn == nil {
		if err := b.connectToTCPServer(); err != nil {
			return fmt.Errorf("failed to reconnect to TCP server: %w", err)
		}
	}

	_, err := b.tcpConn.Write(data)
	if err != nil {
		log.Printf("Failed to write data to TCP server: %v", err)
		// Attempt to reconnect and retry sending data
		b.tcpConn.Close()
		b.tcpConn = nil

		if reconnectErr := b.connectToTCPServer(); reconnectErr != nil {
			return fmt.Errorf("failed to reconnect after write error: %w", reconnectErr)
		}

		_, retryErr := b.tcpConn.Write(data)
		if retryErr != nil {
			return fmt.Errorf("failed to send data after reconnecting: %w", retryErr)
		}
	}

	log.Printf("Data successfully forwarded to TCP server: %s", data)
	return nil
}
