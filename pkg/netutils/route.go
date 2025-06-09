package netutils

import (
	"fmt"
	"log"
	"net"

	"github.com/vishvananda/netlink"
)

func routeExists(dst *net.IPNet, linkIndex int) (bool, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		LinkIndex: linkIndex,
		Dst:       dst,
	}, netlink.RT_FILTER_DST|netlink.RT_FILTER_OIF)
	if err != nil {
		return false, fmt.Errorf("failed to list routes for dst %s on link %d: %w", dst.String(), linkIndex, err)
	}
	return len(routes) > 0, nil
}

func AddRoute(ip net.IP, linkIndex int) error {
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

	log.Printf("Added route for %s on link index %d", ip.String(), linkIndex)
	return nil
}

func RemoveRoute(ip net.IP, linkIndex int) error {
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

	route := &netlink.Route{
		LinkIndex: linkIndex,
		Scope:     netlink.SCOPE_LINK,
		Dst:       routeDst,
	}

	if err := netlink.RouteDel(route); err != nil {
		return fmt.Errorf("failed to remove route for %s: %w", ip.String(), err)
	}

	log.Printf("Removed route for %s on link index %d", ip.String(), linkIndex)
	return nil
}
