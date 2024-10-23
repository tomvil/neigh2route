package netutils

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

// TestAddRouteIntegration adds a real route and checks if it is added
func TestAddRouteIntegration(t *testing.T) {
	ip := net.ParseIP("192.168.100.100")
	linkIndex := 1

	err := AddRoute(ip, linkIndex)
	if err != nil {
		t.Fatalf("failed to add route: %v", err)
	}

	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		LinkIndex: linkIndex,
		Dst:       &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)},
	}, netlink.RT_FILTER_DST|netlink.RT_FILTER_OIF)
	if err != nil {
		t.Fatalf("failed to list routes: %v", err)
	}

	if len(routes) == 0 {
		t.Fatalf("expected route to exist but none found")
	}
}

// TestRemoveRouteIntegration removes a real route and checks if it is removed
func TestRemoveRouteIntegration(t *testing.T) {
	ip := net.ParseIP("192.168.100.100")
	linkIndex := 1

	err := RemoveRoute(ip, linkIndex)
	if err != nil {
		t.Fatalf("failed to remove route: %v", err)
	}

	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		LinkIndex: linkIndex,
		Dst:       &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)},
	}, netlink.RT_FILTER_DST|netlink.RT_FILTER_OIF)
	if err != nil {
		t.Fatalf("failed to list routes: %v", err)
	}

	if len(routes) > 0 {
		t.Fatalf("expected route to be removed but found")
	}
}
