//go:build linux

package neighbor

import (
	"context"
	"errors"
	"net"
	"syscall"
)

const (
	etherTypeIPv6       = 0x86dd
	ethernetHeaderSize  = 14
	trafficCaptureSize  = 64
	trafficReadTimeoutS = 1
)

func observeTraffic(ctx context.Context, device *net.Interface, observed chan<- net.HardwareAddr) error {
	fd, err := syscall.LsfSocket(device.Index, 0x0003)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	if err := syscall.AttachLsf(fd, trafficFilter()); err != nil {
		return err
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Sec: trafficReadTimeoutS}); err != nil {
		return err
	}
	frame := make([]byte, trafficCaptureSize)
	for {
		n, readErr := syscall.Read(fd, frame)
		if readErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(readErr, syscall.EINTR) || errors.Is(readErr, syscall.EAGAIN) || errors.Is(readErr, syscall.EWOULDBLOCK) {
				continue
			}
			return readErr
		}
		mac := sourceMAC(frame[:n], device.HardwareAddr)
		if mac == nil {
			continue
		}
		select {
		case observed <- mac:
		default:
		}
	}
}

func sourceMAC(frame []byte, local net.HardwareAddr) net.HardwareAddr {
	if len(frame) < ethernetHeaderSize {
		return nil
	}
	etherType := uint16(frame[12])<<8 | uint16(frame[13])
	if etherType != etherTypeARP && etherType != etherTypeIPv4 && etherType != etherTypeIPv6 {
		return nil
	}
	mac := net.HardwareAddr(frame[6:12])
	if !isUnicastMAC(mac) || mac.String() == local.String() {
		return nil
	}
	return append(net.HardwareAddr(nil), mac...)
}

func isUnicastMAC(mac net.HardwareAddr) bool {
	return len(mac) == 6 && mac[0]&1 == 0 && (mac[0]|mac[1]|mac[2]|mac[3]|mac[4]|mac[5]) != 0
}

func trafficFilter() []syscall.SockFilter {
	const (
		bpfLoadHalfAbsolute = 0x28
		bpfJumpEqual        = 0x15
		bpfReturn           = 0x06
	)
	return []syscall.SockFilter{
		{Code: bpfLoadHalfAbsolute, K: 12},
		{Code: bpfJumpEqual, Jt: 3, K: etherTypeARP},
		{Code: bpfJumpEqual, Jt: 2, K: etherTypeIPv4},
		{Code: bpfJumpEqual, Jt: 1, K: etherTypeIPv6},
		{Code: bpfReturn, K: 0},
		{Code: bpfReturn, K: trafficCaptureSize},
	}
}
