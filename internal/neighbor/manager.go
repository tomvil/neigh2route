package neighbor

import (
	"log"
	"net"
	"sync"
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
	var shouldRemoveNeighbor bool

	nm.mu.Lock()
	neighbor := nm.getNeighbor(ip)
	if !neighbor.IsEmpty() {
		if !neighbor.LinkIndexChanged(linkIndex) {
			nm.mu.Unlock()
			return
		}

		log.Printf("Neighbor %s link index changed, re-adding neighbor", ip.String())
		shouldRemoveNeighbor = true
	}

	nm.reachableNeighbors = append(nm.reachableNeighbors, Neighbor{
		ip:        ip,
		linkIndex: linkIndex,
	})
	nm.mu.Unlock()

	if shouldRemoveNeighbor {
		nm.RemoveNeighbor(neighbor.ip, neighbor.linkIndex)
	}

	if err := netutils.AddRoute(ip, linkIndex); err != nil {
		log.Printf("Failed to add route for neighbor %s: %v", ip.String(), err)
		return
	}

	log.Printf("Added neighbor %s", ip.String())
}

func (nm *NeighborManager) RemoveNeighbor(ip net.IP, linkIndex int) {
	var shouldRemoveRoute bool

	nm.mu.Lock()
	for i, n := range nm.reachableNeighbors {
		if n.ip.Equal(ip) && n.linkIndex == linkIndex {
			nm.reachableNeighbors = append(nm.reachableNeighbors[:i], nm.reachableNeighbors[i+1:]...)
			log.Printf("Removed neighbor %s", ip.String())
			shouldRemoveRoute = true
			break
		}
	}
	nm.mu.Unlock()

	if shouldRemoveRoute {
		if err := netutils.RemoveRoute(ip, linkIndex); err != nil {
			log.Printf("Failed to remove route for neighbor %s: %v", ip.String(), err)
			return
		}
		log.Printf("Removed route for neighbor %s", ip.String())
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
	interfaceIndex := 0
	if nm.targetInterfaceIndex >= 0 {
		interfaceIndex = nm.targetInterfaceIndex
	}

	neighbors, err := netlink.NeighList(interfaceIndex, netlink.FAMILY_ALL)
	if err != nil {
		return err
	}

	log.Printf("Initializing neighbor table with %d neighbors", len(neighbors))

	for _, n := range neighbors {
		if n.IP.IsLinkLocalUnicast() {
			log.Printf("Skipping link-local neighbor with IP=%s, LinkIndex=%d", n.IP, n.LinkIndex)
			continue
		}

		if (n.State&(netlink.NUD_REACHABLE|netlink.NUD_STALE)) != 0 && !nm.isNeighborExternallyLearned(n.Flags) {
			log.Printf("Adding neighbor with IP=%s, LinkIndex=%d", n.IP, n.LinkIndex)
			nm.AddNeighbor(n.IP, n.LinkIndex)
		}
	}

	log.Printf("Neighbor table initialized finished")

	return nil
}

func (nm *NeighborManager) MonitorNeighbors() {
	updates := make(chan netlink.NeighUpdate)
	done := make(chan struct{})
	defer close(done)

	if err := netlink.NeighSubscribe(updates, done); err != nil {
		log.Fatalf("Failed to subscribe to neighbor updates: %v (interface: %s, index: %d)",
			err, nm.targetInterface, nm.targetInterfaceIndex)
	}

	for update := range updates {
		if nm.targetInterfaceIndex > 0 && update.Neigh.LinkIndex != nm.targetInterfaceIndex {
			continue
		}

		if update.Neigh.IP.IsLinkLocalUnicast() {
			continue
		}

		log.Printf("Received neighbor update: IP=%s, State=%s, Flags=%s, LinkIndex=%d",
			update.Neigh.IP, neighborStateToString(update.Neigh.State), neighborFlagsToString(update.Neigh.Flags), update.Neigh.LinkIndex)

		if (update.Neigh.State&(netlink.NUD_REACHABLE|netlink.NUD_STALE)) != 0 && !nm.isNeighborExternallyLearned(update.Neigh.Flags) {
			log.Printf("Adding neighbor with IP=%s, LinkIndex=%d", update.Neigh.IP, update.Neigh.LinkIndex)
			nm.AddNeighbor(update.Neigh.IP, update.Neigh.LinkIndex)
		}

		if update.Neigh.State == netlink.NUD_FAILED || nm.isNeighborExternallyLearned(update.Neigh.Flags) {
			log.Printf("Removing neighbor with IP=%s, LinkIndex=%d", update.Neigh.IP, update.Neigh.LinkIndex)
			nm.RemoveNeighbor(update.Neigh.IP, update.Neigh.LinkIndex)
		}
	}
}

func (nm *NeighborManager) SendPings() {
	for {
		var wg sync.WaitGroup

		nm.mu.Lock()
		neighbors := make([]Neighbor, len(nm.reachableNeighbors))
		copy(neighbors, nm.reachableNeighbors)
		nm.mu.Unlock()

		for _, n := range neighbors {
			wg.Add(1)
			go func(n Neighbor) {
				defer wg.Done()
				if err := netutils.Ping(n.ip.String()); err != nil {
					log.Printf("Failed to ping neighbor %s: %v", n.ip.String(), err)
				}
			}(n)
		}
		wg.Wait()

		<-time.After(30 * time.Second)
	}
}

func (nm *NeighborManager) PersistentRoutes() {
	for {
		nm.mu.Lock()
		neighbors := make([]Neighbor, len(nm.reachableNeighbors))
		copy(neighbors, nm.reachableNeighbors)
		nm.mu.Unlock()

		log.Printf("Adding persistent routes for %d neighbors", len(neighbors))
		for _, n := range neighbors {
			if err := netutils.AddRoute(n.ip, n.linkIndex); err != nil {
				log.Printf("Failed to add route for neighbor %s: %v", n.ip.String(), err)
			}
		}

		<-time.After(30 * time.Second)
	}
}

func (nm *NeighborManager) Cleanup() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	for _, n := range nm.reachableNeighbors {
		if err := netutils.RemoveRoute(n.ip, n.linkIndex); err != nil {
			log.Printf("Failed to remove route for neighbor %s: %v", n.ip.String(), err)
			continue
		}
		log.Printf("Removed route for neighbor %s", n.ip.String())
	}
}
