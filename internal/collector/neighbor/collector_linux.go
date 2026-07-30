//go:build linux

package neighbor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/collector"
)

const (
	netlinkRoute      = 0
	rtmNewNeighbor    = 28
	rtmDelNeighbor    = 29
	rtmGroupNeighbor  = 4
	nudReachable      = 0x02
	nudFailed         = 0x20
	ndaDestination    = 1
	ndaLinkAddress    = 2
	netlinkBufferSize = 64 * 1024
)

// Collector observes IPv4 ARP neighbor state changes on one interface.
type Collector struct {
	interfaceName string
	routerID      string
}

func NewCollector(interfaceName, routerID string) *Collector {
	return &Collector{interfaceName: interfaceName, routerID: routerID}
}

func (c *Collector) Name() string { return "neighbor" }

func (c *Collector) Start(ctx context.Context, emitter collector.Emitter) error {
	if c.interfaceName == "" {
		return errors.New("neighbor interface must not be empty")
	}
	if c.routerID == "" {
		return errors.New("neighbor router ID must not be empty")
	}
	if emitter == nil {
		return errors.New("neighbor emitter must not be nil")
	}
	device, err := net.InterfaceByName(c.interfaceName)
	if err != nil {
		return fmt.Errorf("find neighbor interface %q: %w", c.interfaceName, err)
	}

	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, netlinkRoute)
	if err != nil {
		return fmt.Errorf("open neighbor netlink socket: %w", err)
	}
	defer syscall.Close(fd)
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK, Groups: rtmGroupNeighbor}); err != nil {
		return fmt.Errorf("bind neighbor netlink socket: %w", err)
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Sec: 1}); err != nil {
		return fmt.Errorf("set neighbor receive timeout: %w", err)
	}

	buffer := make([]byte, netlinkBufferSize)
	for {
		length, _, readErr := syscall.Recvfrom(fd, buffer, 0)
		if readErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(readErr, syscall.EINTR) || errors.Is(readErr, syscall.EAGAIN) || errors.Is(readErr, syscall.EWOULDBLOCK) {
				continue
			}
			return fmt.Errorf("read neighbor netlink socket: %w", readErr)
		}
		messages, parseErr := syscall.ParseNetlinkMessage(buffer[:length])
		if parseErr != nil {
			continue
		}
		for _, message := range messages {
			observed := parseMessage(c.routerID, c.interfaceName, device.Index, message, time.Now())
			if observed == nil {
				continue
			}
			if err := emitter.Emit(ctx, *observed); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("emit neighbor event: %w", err)
			}
		}
	}
}

func nativeUint16(value []byte) uint16 {
	return binary.NativeEndian.Uint16(value)
}
