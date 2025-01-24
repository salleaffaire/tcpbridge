package bridge

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

type TCP2HTTPBridgeCaller struct {
	TCPPort     int
	HTTPPortIn  int
	HTTPPortOut int

	httpServer *http.Server
	conn       *net.Conn

	wg sync.WaitGroup
}

func NewTCP2HTTPBridgeCaller(tcpPort, httpPortIn, httpPortOut int) *TCP2HTTPBridgeCaller {
	bridge := &TCP2HTTPBridgeCaller{
		TCPPort:     tcpPort,
		HTTPPortIn:  httpPortIn,
		HTTPPortOut: httpPortOut,

		conn:       nil,
		httpServer: nil,
	}

	// HTTP Server for incoming data
	bridge.httpServer = &http.Server{Addr: fmt.Sprintf(":%d", bridge.HTTPPortIn)}

	// Start the HTTP server on port httpPortIn
	bridge.wg.Add(1)
	go bridge.startHTTPServer(&bridge.wg)

	return bridge
}

func (b *TCP2HTTPBridgeCaller) Wait() {
	b.wg.Wait()
	fmt.Println("All goroutines finished. Exiting.")
	b.httpServer.Close()
}

func (b *TCP2HTTPBridgeCaller) startHTTPServer(wg *sync.WaitGroup) {
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

		if b.conn == nil {
			// Establish a TCP connection to the server
			conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", b.TCPPort))
			if err != nil {
				log.Fatalf("Failed to establish TCP connection: %v", err)
			}
			b.conn = &conn

			go b.handleTCPConnection(conn)
		}

		// Send the data to the TCP server
		b.sendTCPData(body)
	})

	log.Printf("HTTP server running on port %d", b.HTTPPortIn)
	if err := b.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func (bridge *TCP2HTTPBridgeCaller) sendTCPData(data []byte) {
	// Send data on b.conn
	if bridge.conn == nil {
		log.Println("No TCP connection available")
		return
	}
	_, err := (*bridge.conn).Write(data)
	if err != nil {
		log.Printf("Failed to send data to TCP server: %v", err)
	}
}

func (b *TCP2HTTPBridgeCaller) handleTCPConnection(conn net.Conn) {
	defer conn.Close()

	for {
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
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
	log.Printf("Connection from %s closed", conn.RemoteAddr())
}

func (b *TCP2HTTPBridgeCaller) sendHTTPData(data []byte) {
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/data", b.HTTPPortOut), "application/octet-stream",
		bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to send HTTP request: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("HTTP response status: %s", resp.Status)
}
