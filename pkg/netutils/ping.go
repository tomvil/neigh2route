package netutils

import (
	"fmt"
	"time"

	"github.com/go-ping/ping"
)

func Ping(ip string) error {
	pinger, err := ping.NewPinger(ip)
	if err != nil {
		return fmt.Errorf("failed to create pinger: %w", err)
	}

	pinger.Count = 3
	pinger.Timeout = time.Second * 5
	pinger.Interval = time.Second * 1
	pinger.SetPrivileged(true)

	err = pinger.Run()
	if err != nil {
		return fmt.Errorf("failed to run pinger: %w", err)
	}

	return nil
}
