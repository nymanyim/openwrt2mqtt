//go:build linux

package neighbor

import (
	"encoding/binary"
	"errors"
	"net"
	"syscall"
	"time"
)

const (
	etherTypeARP  = 0x0806
	etherTypeIPv4 = 0x0800
	arpPacketSize = 42
)

func probeARP(device *net.Interface, sourceIP, targetIP net.IP, targetMAC net.HardwareAddr, timeout time.Duration) bool {
	return probeARPAttempts(device, sourceIP, targetIP, targetMAC, time.Now().Add(timeout), 1, 0)
}

func probeARPAttempts(device *net.Interface, sourceIP, targetIP net.IP, targetMAC net.HardwareAddr, deadline time.Time, attempts int, interval time.Duration) bool {
	if attempts < 1 || !time.Now().Before(deadline) {
		return false
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(etherTypeARP)))
	if err != nil {
		return false
	}
	defer syscall.Close(fd)
	if syscall.Bind(fd, &syscall.SockaddrLinklayer{Protocol: htons(etherTypeARP), Ifindex: device.Index}) != nil {
		return false
	}
	packet := make([]byte, arpPacketSize)
	copy(packet[0:6], targetMAC)
	copy(packet[6:12], device.HardwareAddr)
	binary.BigEndian.PutUint16(packet[12:14], etherTypeARP)
	binary.BigEndian.PutUint16(packet[14:16], 1)
	binary.BigEndian.PutUint16(packet[16:18], etherTypeIPv4)
	packet[18], packet[19] = 6, 4
	binary.BigEndian.PutUint16(packet[20:22], 1)
	copy(packet[22:28], device.HardwareAddr)
	copy(packet[28:32], sourceIP.To4())
	copy(packet[32:38], targetMAC)
	copy(packet[38:42], targetIP.To4())
	var destination [8]uint8
	copy(destination[:], targetMAC)
	address := &syscall.SockaddrLinklayer{Protocol: htons(etherTypeARP), Ifindex: device.Index, Halen: 6, Addr: destination}
	buffer := make([]byte, 256)
	nextSend := time.Now()
	sent := 0
	for {
		now := time.Now()
		if sent < attempts && !now.Before(nextSend) {
			if syscall.Sendto(fd, packet, 0, address) != nil {
				return false
			}
			sent++
			nextSend = nextSend.Add(interval)
			continue
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		wait := remaining
		if sent < attempts {
			untilNextSend := time.Until(nextSend)
			if untilNextSend <= 0 {
				continue
			}
			if untilNextSend < wait {
				wait = untilNextSend
			}
		}
		if syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, durationTimeval(wait)) != nil {
			return false
		}
		n, _, receiveErr := syscall.Recvfrom(fd, buffer, 0)
		if receiveErr != nil {
			if errors.Is(receiveErr, syscall.EINTR) || errors.Is(receiveErr, syscall.EAGAIN) || errors.Is(receiveErr, syscall.EWOULDBLOCK) {
				continue
			}
			return false
		}
		if n < arpPacketSize || binary.BigEndian.Uint16(buffer[20:22]) != 2 {
			continue
		}
		if net.IP(buffer[28:32]).Equal(targetIP) && net.HardwareAddr(buffer[22:28]).String() == targetMAC.String() {
			return true
		}
	}
}

func durationTimeval(value time.Duration) *syscall.Timeval {
	if value < time.Microsecond {
		value = time.Microsecond
	}
	return &syscall.Timeval{Sec: int64(value / time.Second), Usec: int64((value % time.Second) / time.Microsecond)}
}
func interfaceIPv4(device *net.Interface) (net.IP, error) {
	addresses, err := device.Addrs()
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		var ip net.IP
		switch value := address.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip4 := ip.To4(); ip4 != nil {
			return ip4, nil
		}
	}
	return nil, errors.New("interface has no IPv4 address")
}
func htons(value uint16) uint16 { return value<<8 | value>>8 }
