package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/j-keck/arping"
	"github.com/vishvananda/netlink"
)

var (
	listenInterface = flag.String("interface", "", "Interface to monitor for neighbor updates")
)

type NeighborManager struct {
	mu                   sync.Mutex
	reachableNeighbors   []net.IP
	targetInterface      string
	targetInterfaceIndex int
}

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

func (nm *NeighborManager) AddNeighbor(ip net.IP, linkIndex int) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.NeighborExists(ip) {
		return
	}

	nm.reachableNeighbors = append(nm.reachableNeighbors, ip)
	if err := addRoute(ip, linkIndex); err != nil {
		log.Printf("Failed to add route for neighbor %s: %v", ip.String(), err)
	}
}

func (nm *NeighborManager) RemoveNeighbor(ip net.IP, linkIndex int) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	for i, n := range nm.reachableNeighbors {
		if n.Equal(ip) {
			nm.reachableNeighbors = append(nm.reachableNeighbors[:i], nm.reachableNeighbors[i+1:]...)
			if err := removeRoute(ip, linkIndex); err != nil {
				log.Printf("Failed to remove route for neighbor %s: %v", ip.String(), err)
			}
			return
		}
	}
}

func (nm *NeighborManager) NeighborExists(ip net.IP) bool {
	for _, n := range nm.reachableNeighbors {
		if n.Equal(ip) {
			return true
		}
	}
	return false
}

func (nm *NeighborManager) isNeighborExternallyLearned(flags int) bool {
	return flags&netlink.NTF_EXT_LEARNED != 0
}

func (nm *NeighborManager) initializeNeighborTable() error {
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
		if n.State == netlink.NUD_REACHABLE && !nm.isNeighborExternallyLearned(n.Flags) {
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

		if update.Neigh.State == netlink.NUD_REACHABLE && !nm.isNeighborExternallyLearned(update.Neigh.Flags) {
			nm.AddNeighbor(update.Neigh.IP, update.Neigh.LinkIndex)
		}

		if update.Neigh.State == netlink.NUD_FAILED || nm.isNeighborExternallyLearned(update.Neigh.Flags) {
			nm.RemoveNeighbor(update.Neigh.IP, update.Neigh.LinkIndex)
		}
	}
}

func routeExists(dst *net.IPNet, linkIndex int) (bool, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		LinkIndex: linkIndex,
		Dst:       dst,
	}, netlink.RT_FILTER_DST|netlink.RT_FILTER_OIF)
	if err != nil {
		return false, err
	}
	return len(routes) > 0, nil
}

func addRoute(ip net.IP, linkIndex int) error {
	mask := net.CIDRMask(32, 32)
	if ip.To4() == nil {
		mask = net.CIDRMask(128, 128)
	}

	routeDst := &net.IPNet{IP: ip, Mask: mask}

	exists, err := routeExists(routeDst, linkIndex)
	if err != nil {
		return fmt.Errorf("failed to check if route exists for %s: %w", ip.String(), err)
	}
	if exists {
		return nil
	}

	route := &netlink.Route{
		LinkIndex: linkIndex,
		Scope:     netlink.SCOPE_LINK,
		Dst:       &net.IPNet{IP: ip, Mask: mask},
	}

	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("failed to add route for %s: %w", ip.String(), err)
	}

	fmt.Printf("Added route for %s\n", ip.String())
	return nil
}

func removeRoute(ip net.IP, linkIndex int) error {
	mask := net.CIDRMask(32, 32)
	if ip.To4() == nil {
		mask = net.CIDRMask(128, 128)
	}

	routeDst := &net.IPNet{IP: ip, Mask: mask}

	exists, err := routeExists(routeDst, linkIndex)
	if err != nil {
		return fmt.Errorf("failed to check if route exists for %s: %w", ip.String(), err)
	}
	if !exists {
		return nil
	}

	fmt.Printf("Removing route for neighbor %+v\n", ip)

	route := &netlink.Route{
		LinkIndex: linkIndex,
		Scope:     netlink.SCOPE_LINK,
		Dst:       routeDst,
	}

	if err := netlink.RouteDel(route); err != nil {
		return fmt.Errorf("failed to remove route for %s: %w", ip.String(), err)
	}

	fmt.Printf("Removed route for %s\n", ip.String())
	return nil
}

func SendARPRequests(nm *NeighborManager) {
	for {
		nm.mu.Lock()
		for _, ip := range nm.reachableNeighbors {
			arping.Ping(ip)
		}
		nm.mu.Unlock()

		<-time.After(30 * time.Second)
	}
}

func main() {
	flag.Parse()

	fmt.Println("Initializing neighbor table and monitoring updates...")

	nm, err := NewNeighborManager(*listenInterface)
	if err != nil {
		log.Fatalf("Failed to initialize neighbor manager: %v", err)
	}

	if err := nm.initializeNeighborTable(); err != nil {
		log.Fatalf("Failed to initialize neighbor table: %v", err)
	}

	go SendARPRequests(nm)

	nm.MonitorNeighbors()
}
