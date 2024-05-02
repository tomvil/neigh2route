package neighbor

import (
	"log"
	"net"
	"time"

	"github.com/tomvil/neigh2route/pkg/netutils"
	"github.com/vishvananda/netlink"
)

func NewNeighborManager(targetInterface string) (*NeighborManager, error) {
	nm := &NeighborManager{
		targetInterface: targetInterface,
	}

	if targetInterface != "" {
		iface, err := netlink.LinkByName(targetInterface)
		if err != nil {
			return nil, err
		}
		nm.targetInterfaceIndex = iface.Attrs().Index
	} else {
		nm.targetInterfaceIndex = -1
	}

	return nm, nil
}

func (n Neighbor) IsEmpty() bool {
	return n.ip == nil
}

func (n Neighbor) LinkIndexChanged(linkIndex int) bool {
	return n.linkIndex != linkIndex
}

func (nm *NeighborManager) AddNeighbor(ip net.IP, linkIndex int) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if neighbor := nm.getNeighbor(ip); !neighbor.IsEmpty() {
		if !neighbor.LinkIndexChanged(linkIndex) {
			return
		}

		log.Printf("Neighbor %s link index changed, re-adding route", ip.String())
		nm.RemoveNeighbor(neighbor.ip, neighbor.linkIndex)
	}

	nm.reachableNeighbors = append(nm.reachableNeighbors, Neighbor{
		ip:        ip,
		linkIndex: linkIndex,
	})

	if err := netutils.AddRoute(ip, linkIndex); err != nil {
		log.Printf("Failed to add route for neighbor %s: %v", ip.String(), err)
	}
}

func (nm *NeighborManager) RemoveNeighbor(ip net.IP, linkIndex int) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	for i, n := range nm.reachableNeighbors {
		if n.ip.Equal(ip) {
			nm.reachableNeighbors = append(nm.reachableNeighbors[:i], nm.reachableNeighbors[i+1:]...)
			if err := netutils.RemoveRoute(ip, linkIndex); err != nil {
				log.Printf("Failed to remove route for neighbor %s: %v", ip.String(), err)
			}
			return
		}
	}
}

func (nm *NeighborManager) getNeighbor(ip net.IP) Neighbor {
	for _, n := range nm.reachableNeighbors {
		if n.ip.Equal(ip) {
			return n
		}
	}
	return Neighbor{}
}

func (nm *NeighborManager) isNeighborExternallyLearned(flags int) bool {
	return flags&netlink.NTF_EXT_LEARNED != 0
}

func (nm *NeighborManager) InitializeNeighborTable() error {
	var neighbors []netlink.Neigh
	var err error

	if nm.targetInterfaceIndex >= 0 {
		neighbors, err = netlink.NeighList(nm.targetInterfaceIndex, netlink.FAMILY_ALL)
	} else {
		neighbors, err = netlink.NeighList(0, netlink.FAMILY_ALL)
	}

	if err != nil {
		return err
	}

	for _, n := range neighbors {
		if (n.State&(netlink.NUD_REACHABLE|netlink.NUD_STALE)) != 0 && !nm.isNeighborExternallyLearned(n.Flags) {
			nm.AddNeighbor(n.IP, n.LinkIndex)
		}
	}

	return nil
}

func (nm *NeighborManager) MonitorNeighbors() {
	updates := make(chan netlink.NeighUpdate)
	done := make(chan struct{})
	defer close(done)

	if err := netlink.NeighSubscribe(updates, done); err != nil {
		log.Fatalf("Failed to subscribe to neighbor updates: %v", err)
	}

	for update := range updates {
		if nm.targetInterfaceIndex > 0 && update.Neigh.LinkIndex != nm.targetInterfaceIndex {
			continue
		}

		if (update.Neigh.State&(netlink.NUD_REACHABLE|netlink.NUD_STALE)) != 0 && !nm.isNeighborExternallyLearned(update.Neigh.Flags) {
			nm.AddNeighbor(update.Neigh.IP, update.Neigh.LinkIndex)
		}

		if update.Neigh.State == netlink.NUD_FAILED || nm.isNeighborExternallyLearned(update.Neigh.Flags) {
			nm.RemoveNeighbor(update.Neigh.IP, update.Neigh.LinkIndex)
		}
	}
}

func (nm *NeighborManager) SendARPRequests() {
	for {
		nm.mu.Lock()
		for _, neighbor := range nm.reachableNeighbors {
			netutils.SendARPRequest(neighbor.ip)
		}
		nm.mu.Unlock()

		<-time.After(30 * time.Second)
	}
}

func (nm *NeighborManager) Cleanup() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	for _, n := range nm.reachableNeighbors {
		if err := netutils.RemoveRoute(n.ip, n.linkIndex); err != nil {
			log.Printf("Failed to remove route for neighbor %s: %v", n.ip.String(), err)
		}
	}
}
