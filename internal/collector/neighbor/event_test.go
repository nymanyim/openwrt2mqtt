//go:build linux

package neighbor

import (
	"encoding/binary"
	"syscall"
	"testing"
)

func TestParseActiveNeighbor(t *testing.T) {
	for _, state := range []uint16{nudReachable, nudStale, nudDelay, nudProbe, nudPermanent} {
		observed := parseNeighbor(7, fixtureNeighborMessage(rtmNewNeighbor, state, 7))
		if observed == nil || !observed.active {
			t.Fatalf("state %#x was not active: %#v", state, observed)
		}
		if observed.mac.String() != "02:11:22:33:44:55" || observed.ip.String() != "192.168.1.50" {
			t.Fatalf("unexpected observation: %#v", observed)
		}
	}
}
func TestParseInactiveNeighbor(t *testing.T) {
	for _, message := range []syscall.NetlinkMessage{fixtureNeighborMessage(rtmNewNeighbor, nudIncomplete, 7), fixtureNeighborMessage(rtmNewNeighbor, nudFailed, 7), fixtureNeighborMessage(rtmDelNeighbor, 0, 7)} {
		observed := parseNeighbor(7, message)
		if observed == nil || observed.active {
			t.Fatalf("unexpected observation: %#v", observed)
		}
	}
}
func TestParseNeighborIgnoresOtherInterface(t *testing.T) {
	if parseNeighbor(8, fixtureNeighborMessage(rtmNewNeighbor, nudReachable, 7)) != nil {
		t.Fatal("accepted another interface")
	}
}
func fixtureNeighborMessage(messageType, state uint16, interfaceIndex int32) syscall.NetlinkMessage {
	header := make([]byte, neighborHeaderSize)
	header[0] = syscall.AF_INET
	binary.NativeEndian.PutUint32(header[4:8], uint32(interfaceIndex))
	binary.NativeEndian.PutUint16(header[8:10], state)
	attributes := append(routeAttribute(ndaDestination, []byte{192, 168, 1, 50}), routeAttribute(ndaLinkAddress, []byte{2, 17, 34, 51, 68, 85})...)
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
