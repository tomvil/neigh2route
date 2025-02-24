package neighbor

import (
	"strings"

	"github.com/vishvananda/netlink"
)

func neighborStateToString(state int) string {
	states := []string{}
	if state&netlink.NUD_INCOMPLETE != 0 {
		states = append(states, "INCOMPLETE")
	}
	if state&netlink.NUD_REACHABLE != 0 {
		states = append(states, "REACHABLE")
	}
	if state&netlink.NUD_STALE != 0 {
		states = append(states, "STALE")
	}
	if state&netlink.NUD_DELAY != 0 {
		states = append(states, "DELAY")
	}
	if state&netlink.NUD_PROBE != 0 {
		states = append(states, "PROBE")
	}
	if state&netlink.NUD_FAILED != 0 {
		states = append(states, "FAILED")
	}
	if len(states) == 0 {
		return "UNKNOWN"
	}
	return strings.Join(states, "|")
}

func neighborFlagsToString(flags int) string {
	flagNames := []string{}

	if flags&netlink.NTF_USE != 0 {
		flagNames = append(flagNames, "USE")
	}
	if flags&netlink.NTF_SELF != 0 {
		flagNames = append(flagNames, "SELF")
	}
	if flags&netlink.NTF_MASTER != 0 {
		flagNames = append(flagNames, "MASTER")
	}
	if flags&netlink.NTF_PROXY != 0 {
		flagNames = append(flagNames, "PROXY")
	}
	if flags&netlink.NTF_EXT_LEARNED != 0 {
		flagNames = append(flagNames, "EXT_LEARNED")
	}
	if flags&netlink.NTF_ROUTER != 0 {
		flagNames = append(flagNames, "ROUTER")
	}

	if len(flagNames) == 0 {
		return "NONE"
	}

	return strings.Join(flagNames, "|")
}
