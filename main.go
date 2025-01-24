package main

import (
	"flag"

	bridge "github.com/salleaffaire/httpbridge/bridge"
)

const (
	// Modes
	Listener = "listener"
	Caller   = "caller"
)

func main() {
	// Define CLI flags for source and destination ports
	tcpPort := flag.Int("tcpp", 8080, "TCP traffic in/out")
	httpPortIn := flag.Int("httppin", 9090, "Port to receive HTTP traffic")
	httpPortOut := flag.Int("httppout", 9091, "Port to send HTTP traffic")
	mode := flag.String("mode", Listener, "Mode to run the bridge in: listener or caller")

	// Parse CLI flags
	flag.Parse()

	if *mode == Listener {
		// Create a new bridge instance
		TCP2HTTPBridgeListener := bridge.NewTCP2HTTPBridgeListener(*tcpPort, *httpPortIn, *httpPortOut)
		// Wait on bridge
		TCP2HTTPBridgeListener.Wait()
		return
	} else if *mode == Caller {
		// Create a new bridge instance
		TCP2HTTPBridgeCaller := bridge.NewTCP2HTTPBridgeCaller(*tcpPort, *httpPortIn, *httpPortOut)
		// Wait on bridge
		TCP2HTTPBridgeCaller.Wait()
		return
	} else {
		panic("Invalid mode. Must be either listener or caller")
	}

}
