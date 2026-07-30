package hostapd

import (
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	message := parseLine("router-a", "daemon.notice hostapd: phy1-ap0: AP-STA-CONNECTED AA:BB:CC:DD:EE:FF auth_alg=open", now, func(string) map[string]any {
		return map[string]any{"ip": "192.168.1.10", "hostname": "phone"}
	})
	if message == nil {
		t.Fatal("connected line was ignored")
	}
	if message.Type != "device.connected" || message.Source != "hostapd/phy1-ap0" {
		t.Fatalf("unexpected event: %#v", message)
	}
	if message.Data["mac"] != "aa:bb:cc:dd:ee:ff" || message.Data["hostname"] != "phone" {
		t.Fatalf("unexpected data: %#v", message.Data)
	}
}

func TestParseDisconnectedLine(t *testing.T) {
	message := parseLine("router-a", "hostapd: wlan0: AP-STA-DISCONNECTED 02:11:22:33:44:55", time.Now(), nil)
	if message == nil || message.Type != "device.disconnected" {
		t.Fatalf("unexpected event: %#v", message)
	}
}

func TestParseLineRejectsUnrelatedAndInvalidLines(t *testing.T) {
	for _, line := range []string{"hostapd: wlan0: AP-ENABLED", "hostapd: wlan0: AP-STA-CONNECTED invalid"} {
		if parseLine("router-a", line, time.Now(), nil) != nil {
			t.Fatalf("accepted line %q", line)
		}
	}
}
