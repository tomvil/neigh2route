package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tomvil/neigh2route/internal/neighbor"
)

var (
	listenInterface = flag.String("interface", "", "Interface to monitor for neighbor updates")
)

func main() {
	flag.Parse()

	nm, err := neighbor.NewNeighborManager(*listenInterface)
	if err != nil {
		log.Fatalf("Failed to initialize neighbor manager: %v", err)
	}

	if err := nm.InitializeNeighborTable(); err != nil {
		log.Fatalf("Failed to initialize neighbor table: %v", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		log.Printf("Received signal: %s. Cleaning up and exiting...", sig)
		nm.Cleanup()
		os.Exit(0)
	}()

	go nm.SendPings()

	nm.MonitorNeighbors()
}
