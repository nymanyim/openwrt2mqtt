//go:build linux

package dhcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/collector"
)

const (
	ethernetProtocolAll = 0x0003
	captureBufferSize   = 2048

	bpfLoadHalfAbsolute = 0x28
	bpfLoadByteAbsolute = 0x30
	bpfLoadXMSH         = 0xb1
	bpfLoadHalfIndirect = 0x48
	bpfJumpEqual        = 0x15
	bpfJumpSet          = 0x45
	bpfReturn           = 0x06
)

// PacketCollector captures DHCP traffic from one Linux interface.
type PacketCollector struct {
	interfaceName string
	tracker       *Tracker
}

func NewCollector(interfaceName, routerID string) *PacketCollector {
	return &PacketCollector{
		interfaceName: interfaceName,
		tracker:       NewTracker(routerID, "dhcp/"+interfaceName),
	}
}

func (c *PacketCollector) Name() string {
	return "dhcp"
}

func (c *PacketCollector) Start(ctx context.Context, emitter collector.Emitter) error {
	if c.interfaceName == "" {
		return errors.New("DHCP interface must not be empty")
	}
	if emitter == nil {
		return errors.New("DHCP emitter must not be nil")
	}

	device, err := net.InterfaceByName(c.interfaceName)
	if err != nil {
		return fmt.Errorf("find DHCP interface %q: %w", c.interfaceName, err)
	}
	fd, err := syscall.LsfSocket(device.Index, ethernetProtocolAll)
	if err != nil {
		return fmt.Errorf("open AF_PACKET socket on %q: %w", c.interfaceName, err)
	}
	if err := syscall.AttachLsf(fd, dhcpFilter()); err != nil {
		syscall.Close(fd)
		return fmt.Errorf("attach DHCP socket filter: %w", err)
	}

	defer syscall.Close(fd)
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Sec: 1}); err != nil {
		return fmt.Errorf("set DHCP receive timeout: %w", err)
	}

	frame := make([]byte, captureBufferSize)
	for {
		length, readErr := syscall.Read(fd, frame)
		if readErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(readErr, syscall.EINTR) || errors.Is(readErr, syscall.EAGAIN) || errors.Is(readErr, syscall.EWOULDBLOCK) {
				continue
			}
			return fmt.Errorf("read DHCP packet: %w", readErr)
		}

		message, parseErr := ParseFrame(frame[:length])
		if parseErr != nil {
			continue
		}
		connected := c.tracker.Observe(message, time.Now())
		if connected == nil {
			continue
		}
		if err := emitter.Emit(ctx, *connected); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("emit DHCP event: %w", err)
		}
	}
}

// dhcpFilter accepts unfragmented IPv4 UDP packets using DHCP ports.
func dhcpFilter() []syscall.SockFilter {
	return []syscall.SockFilter{
		{Code: bpfLoadHalfAbsolute, K: 12},
		{Code: bpfJumpEqual, Jf: 14, K: 0x0800},
		{Code: bpfLoadByteAbsolute, K: 23},
		{Code: bpfJumpEqual, Jf: 12, K: 17},
		{Code: bpfLoadHalfAbsolute, K: 20},
		{Code: bpfJumpSet, Jt: 10, K: 0x3fff},
		{Code: bpfLoadXMSH, K: 14},
		{Code: bpfLoadHalfIndirect, K: 14},
		{Code: bpfJumpEqual, Jt: 2, K: 67},
		{Code: bpfJumpEqual, Jt: 3, K: 68},
		{Code: bpfReturn, K: 0},
		{Code: bpfLoadHalfIndirect, K: 16},
		{Code: bpfJumpEqual, Jt: 2, Jf: 3, K: 68},
		{Code: bpfLoadHalfIndirect, K: 16},
		{Code: bpfJumpEqual, Jf: 1, K: 67},
		{Code: bpfReturn, K: 0xffff},
		{Code: bpfReturn, K: 0},
	}
}
