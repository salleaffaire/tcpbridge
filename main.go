package main

import (
	"flag"

	bridge "github.com/salleaffaire/httpbridge/bridge"
)

func main() {
	// Define CLI flags for source and destination ports
	tcpPort := flag.Int("tcpp", 8080, "TCP traffic in/out")
	httpPortIn := flag.Int("httppin", 9090, "Port to receive HTTP traffic")
	httpPortOut := flag.Int("httppout", 9091, "Port to send HTTP traffic")

	// Parse CLI flags
	flag.Parse()

	// Create a new bridge instance
	TCP2HTTPBridge := bridge.NewTCP2HTTPBridge(*tcpPort, *httpPortIn, *httpPortOut)
	// Wait on bridge
	TCP2HTTPBridge.Wait()

}
