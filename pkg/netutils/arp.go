package netutils

import (
	"log"
	"net"

	"github.com/j-keck/arping"
)

func SendARPRequest(ip net.IP) {
	if _, _, err := arping.Ping(ip); err != nil {
		log.Printf("Failed to send ARP request to %s: %v", ip.String(), err)
	}
}
