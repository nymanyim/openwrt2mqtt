package hostapd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

const (
	connectedMarker    = "AP-STA-CONNECTED "
	disconnectedMarker = "AP-STA-DISCONNECTED "
)

func parseLine(routerID, line string, now time.Time, resolve func(string) map[string]any) *event.Event {
	eventType, mac := eventFields(line)
	if eventType == "" {
		return nil
	}
	data := map[string]any{"mac": mac, "connection_type": "wifi"}
	if iface := interfaceName(line); iface != "" {
		data["interface"] = iface
	}
	if resolve != nil {
		for key, value := range resolve(mac) {
			data[key] = value
		}
	}
	timestamp := now.UTC()
	return &event.Event{
		SchemaVersion: "1",
		ID:            eventID(routerID, eventType, mac, timestamp),
		RouterID:      routerID,
		Category:      "network",
		Type:          eventType,
		Source:        "hostapd/" + interfaceName(line),
		Timestamp:     timestamp,
		Data:          data,
	}
}

func eventFields(line string) (string, string) {
	for marker, eventType := range map[string]string{
		connectedMarker: "device.connected", disconnectedMarker: "device.disconnected",
	} {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		field := strings.Fields(line[index+len(marker):])
		if len(field) == 0 {
			return "", ""
		}
		mac, err := net.ParseMAC(field[0])
		if err != nil || len(mac) != 6 {
			return "", ""
		}
		return eventType, strings.ToLower(mac.String())
	}
	return "", ""
}

func eventID(routerID, eventType, mac string, timestamp time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d", routerID, eventType, mac, timestamp.UnixNano())))
	return hex.EncodeToString(sum[:16])
}
