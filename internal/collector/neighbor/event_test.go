//go:build linux

package neighbor

import (
	"encoding/binary"
	"syscall"
	"testing"
	"time"
)

func TestParseReachableNeighbor(t *testing.T) {
	message := fixtureNeighborMessage(rtmNewNeighbor, nudReachable, 7)
	event := parseMessage("router-a", "br-lan", 7, message, time.Now())
	if event == nil || event.Type != "device.connected" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.Data["mac"] != "02:11:22:33:44:55" || event.Data["ip"] != "192.168.1.50" {
		t.Fatalf("unexpected data: %#v", event.Data)
	}
}

func TestParseFailedNeighbor(t *testing.T) {
	event := parseMessage("router-a", "br-lan", 7, fixtureNeighborMessage(rtmNewNeighbor, nudFailed, 7), time.Now())
	if event == nil || event.Type != "device.disconnected" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func TestParseNeighborIgnoresOtherInterfaceAndStaleState(t *testing.T) {
	if parseMessage("router-a", "br-lan", 8, fixtureNeighborMessage(rtmNewNeighbor, nudReachable, 7), time.Now()) != nil {
		t.Fatal("accepted another interface")
	}
	if parseMessage("router-a", "br-lan", 7, fixtureNeighborMessage(rtmNewNeighbor, 0x04, 7), time.Now()) != nil {
		t.Fatal("accepted stale state")
	}
}

func fixtureNeighborMessage(messageType, state uint16, interfaceIndex int32) syscall.NetlinkMessage {
	header := make([]byte, neighborHeaderSize)
	header[0] = syscall.AF_INET
	binary.NativeEndian.PutUint32(header[4:8], uint32(interfaceIndex))
	binary.NativeEndian.PutUint16(header[8:10], state)
	attributes := append(routeAttribute(ndaDestination, []byte{192, 168, 1, 50}), routeAttribute(ndaLinkAddress, []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55})...)
	data := append(header, attributes...)
	return syscall.NetlinkMessage{Header: syscall.NlMsghdr{Len: uint32(syscall.NLMSG_HDRLEN + len(data)), Type: messageType}, Data: data}
}

func routeAttribute(attributeType uint16, value []byte) []byte {
	length := syscall.SizeofRtAttr + len(value)
	aligned := (length + 3) &^ 3
	attribute := make([]byte, aligned)
	binary.NativeEndian.PutUint16(attribute[0:2], uint16(length))
	binary.NativeEndian.PutUint16(attribute[2:4], attributeType)
	copy(attribute[syscall.SizeofRtAttr:], value)
	return attribute
}
