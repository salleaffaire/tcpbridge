package main

import (
	"flag"

	bridge "github.com/salleaffaire/httpbridge/bridge"
)

func main() {
	// Define CLI flags for source and destination ports
	srcPort := flag.Int("srcp", 8080, "Source port to capture TCP traffic")
	dstPort := flag.Int("dstp", 9090, "Destination port to send HTTP traffic")

	// Parse CLI flags
	flag.Parse()

	// Create a new bridge instance
	bridge := bridge.NewBridge(*srcPort, *dstPort)

	// Wait on bridge
	bridge.Wait()
}
