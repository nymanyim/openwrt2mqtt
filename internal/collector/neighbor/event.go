package neighbor

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

const neighborHeaderSize = 12

func parseMessage(routerID, interfaceName string, interfaceIndex int, message syscall.NetlinkMessage, now time.Time) *event.Event {
	if message.Header.Type != rtmNewNeighbor && message.Header.Type != rtmDelNeighbor {
		return nil
	}
	if len(message.Data) < neighborHeaderSize || message.Data[0] != syscall.AF_INET {
		return nil
	}
	if int(int32(binary.NativeEndian.Uint32(message.Data[4:8]))) != interfaceIndex {
		return nil
	}

	state := nativeUint16(message.Data[8:10])
	eventType := ""
	switch {
	case message.Header.Type == rtmDelNeighbor || state&nudFailed != 0:
		eventType = "device.disconnected"
	case state&nudReachable != 0:
		eventType = "device.connected"
	default:
		return nil
	}

	data := map[string]any{"connection_type": "network", "interface": interfaceName}
	for offset := neighborHeaderSize; offset+syscall.SizeofRtAttr <= len(message.Data); {
		length := int(binary.NativeEndian.Uint16(message.Data[offset : offset+2]))
		attributeType := binary.NativeEndian.Uint16(message.Data[offset+2 : offset+4])
		if length < syscall.SizeofRtAttr || offset+length > len(message.Data) {
			return nil
		}
		value := message.Data[offset+syscall.SizeofRtAttr : offset+length]
		switch attributeType {
		case ndaDestination:
			if len(value) == net.IPv4len {
				data["ip"] = net.IP(value).String()
			}
		case ndaLinkAddress:
			if len(value) == 6 {
				data["mac"] = strings.ToLower(net.HardwareAddr(value).String())
			}
		}
		offset += (length + 3) &^ 3
	}
	mac, _ := data["mac"].(string)
	if mac == "" {
		return nil
	}
	timestamp := now.UTC()
	return &event.Event{
		SchemaVersion: "1", ID: neighborEventID(routerID, eventType, mac, timestamp), RouterID: routerID,
		Category: "network", Type: eventType, Source: "neighbor/" + interfaceName, Timestamp: timestamp, Data: data,
	}
}

func neighborEventID(routerID, eventType, mac string, timestamp time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d", routerID, eventType, mac, timestamp.UnixNano())))
	return hex.EncodeToString(sum[:16])
}
