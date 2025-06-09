package neighbor

import (
	"net"
	"sync"
)

type NeighborManager struct {
	mu                   sync.Mutex
	reachableNeighbors   []Neighbor
	targetInterface      string
	targetInterfaceIndex int
	isShuttingDown       bool
}

type Neighbor struct {
	ip        net.IP
	linkIndex int
}
