package dhcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
)

const (
	MessageDiscover byte = 1
	MessageOffer    byte = 2
	MessageRequest  byte = 3
	MessageDecline  byte = 4
	MessageACK      byte = 5
	MessageNAK      byte = 6
	MessageRelease  byte = 7
	MessageInform   byte = 8
)

var (
	errNotDHCP       = errors.New("not a DHCP packet")
	errMalformedDHCP = errors.New("malformed DHCP packet")
)

// Message contains the DHCP fields required for transaction correlation.
type Message struct {
	Type        byte
	Transaction uint32
	ClientMAC   string
	YourIP      net.IP
	RequestedIP net.IP
	ServerIP    net.IP
	Hostname    string
}

// ParseFrame parses an Ethernet frame containing an IPv4 UDP DHCP packet.
func ParseFrame(frame []byte) (Message, error) {
	payload, err := udpPayload(frame)
	if err != nil {
		return Message{}, err
	}
	return parsePayload(payload)
}

func udpPayload(frame []byte) ([]byte, error) {
	if len(frame) < 14 {
		return nil, errMalformedDHCP
	}

	offset := 14
	etherType := binary.BigEndian.Uint16(frame[12:14])
	for etherType == 0x8100 || etherType == 0x88a8 {
		if len(frame) < offset+4 {
			return nil, errMalformedDHCP
		}
		etherType = binary.BigEndian.Uint16(frame[offset+2 : offset+4])
		offset += 4
	}
	if etherType != 0x0800 || len(frame) < offset+20 {
		return nil, errNotDHCP
	}

	ip := frame[offset:]
	if ip[0]>>4 != 4 {
		return nil, errNotDHCP
	}
	ipHeaderLen := int(ip[0]&0x0f) * 4
	if ipHeaderLen < 20 || len(ip) < ipHeaderLen+8 {
		return nil, errMalformedDHCP
	}
	if ip[9] != 17 {
		return nil, errNotDHCP
	}
	if binary.BigEndian.Uint16(ip[6:8])&0x3fff != 0 {
		return nil, errNotDHCP
	}

	totalLen := int(binary.BigEndian.Uint16(ip[2:4]))
	if totalLen < ipHeaderLen+8 || totalLen > len(ip) {
		return nil, errMalformedDHCP
	}
	udp := ip[ipHeaderLen:totalLen]
	sourcePort := binary.BigEndian.Uint16(udp[0:2])
	destinationPort := binary.BigEndian.Uint16(udp[2:4])
	if !((sourcePort == 67 && destinationPort == 68) || (sourcePort == 68 && destinationPort == 67)) {
		return nil, errNotDHCP
	}
	udpLen := int(binary.BigEndian.Uint16(udp[4:6]))
	if udpLen < 8 || udpLen > len(udp) {
		return nil, errMalformedDHCP
	}
	return udp[8:udpLen], nil
}

func parsePayload(payload []byte) (Message, error) {
	if len(payload) < 240 || string(payload[236:240]) != "\x63\x82\x53\x63" {
		return Message{}, errMalformedDHCP
	}

	hardwareLen := int(payload[2])
	if payload[1] != 1 || hardwareLen != 6 || len(payload) < 28+hardwareLen {
		return Message{}, fmt.Errorf("%w: unsupported client hardware address", errMalformedDHCP)
	}

	message := Message{
		Transaction: binary.BigEndian.Uint32(payload[4:8]),
		ClientMAC:   strings.ToLower(net.HardwareAddr(payload[28 : 28+hardwareLen]).String()),
	}
	if !isZeroIPv4(payload[16:20]) {
		message.YourIP = append(net.IP(nil), payload[16:20]...)
	}

	for offset := 240; offset < len(payload); {
		code := payload[offset]
		offset++
		switch code {
		case 0:
			continue
		case 255:
			if message.Type == 0 {
				return Message{}, fmt.Errorf("%w: missing message type", errMalformedDHCP)
			}
			return message, nil
		}
		if offset >= len(payload) {
			return Message{}, errMalformedDHCP
		}
		length := int(payload[offset])
		offset++
		if offset+length > len(payload) {
			return Message{}, errMalformedDHCP
		}
		value := payload[offset : offset+length]
		offset += length

		switch code {
		case 12:
			message.Hostname = strings.TrimSpace(string(value))
		case 50:
			if len(value) == net.IPv4len {
				message.RequestedIP = append(net.IP(nil), value...)
			}
		case 53:
			if len(value) == 1 {
				message.Type = value[0]
			}
		case 54:
			if len(value) == net.IPv4len {
				message.ServerIP = append(net.IP(nil), value...)
			}
		}
	}

	return Message{}, fmt.Errorf("%w: missing end option", errMalformedDHCP)
}

func isZeroIPv4(ip []byte) bool {
	return len(ip) == net.IPv4len && ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] == 0
}
