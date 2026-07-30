package dhcp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

var fixtureMAC = net.HardwareAddr{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}

func TestParseFrame(t *testing.T) {
	frame := fixtureFrame(MessageRequest, 0x10203040, net.IPv4(192, 168, 1, 50), nil, "test-device")

	message, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("ParseFrame() error = %v", err)
	}
	if message.Type != MessageRequest || message.Transaction != 0x10203040 {
		t.Fatalf("unexpected message: %#v", message)
	}
	if message.ClientMAC != "02:11:22:33:44:55" {
		t.Fatalf("ClientMAC = %q", message.ClientMAC)
	}
	if !message.RequestedIP.Equal(net.IPv4(192, 168, 1, 50)) {
		t.Fatalf("RequestedIP = %v", message.RequestedIP)
	}
	if message.Hostname != "test-device" {
		t.Fatalf("Hostname = %q", message.Hostname)
	}
}

func TestTrackerEmitsOnlyForInitialAcquisition(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	tracker := NewTracker("router-a", "dhcp/br-lan")
	xid := uint32(0x10203040)

	discover, _ := ParseFrame(fixtureFrame(MessageDiscover, xid, nil, nil, "test-device"))
	request, _ := ParseFrame(fixtureFrame(MessageRequest, xid, net.IPv4(192, 168, 1, 50), nil, "test-device"))
	ack, _ := ParseFrame(fixtureFrame(MessageACK, xid, nil, net.IPv4(192, 168, 1, 50), ""))

	if tracker.Observe(discover, now) != nil || tracker.Observe(request, now.Add(time.Second)) != nil {
		t.Fatal("event emitted before ACK")
	}
	connected := tracker.Observe(ack, now.Add(2*time.Second))
	if connected == nil {
		t.Fatal("ACK did not emit connected event")
	}
	if connected.Type != "device.connected" || connected.Category != "network" {
		t.Fatalf("unexpected event: %#v", connected)
	}
	if connected.Data["ip"] != "192.168.1.50" || connected.Data["hostname"] != "test-device" {
		t.Fatalf("unexpected event data: %#v", connected.Data)
	}

	renewRequest, _ := ParseFrame(fixtureFrame(MessageRequest, xid+1, nil, nil, "test-device"))
	renewACK, _ := ParseFrame(fixtureFrame(MessageACK, xid+1, nil, net.IPv4(192, 168, 1, 50), ""))
	if tracker.Observe(renewRequest, now.Add(time.Minute)) != nil {
		t.Fatal("renewal REQUEST emitted an event")
	}
	if tracker.Observe(renewACK, now.Add(time.Minute+time.Second)) != nil {
		t.Fatal("renewal ACK emitted an event")
	}
}

func TestTrackerRejectsMismatchedTransaction(t *testing.T) {
	tracker := NewTracker("router-a", "dhcp/br-lan")
	now := time.Now()
	discover, _ := ParseFrame(fixtureFrame(MessageDiscover, 1, nil, nil, ""))
	request, _ := ParseFrame(fixtureFrame(MessageRequest, 2, nil, nil, ""))
	ack, _ := ParseFrame(fixtureFrame(MessageACK, 2, nil, net.IPv4(192, 168, 1, 60), ""))

	tracker.Observe(discover, now)
	tracker.Observe(request, now.Add(time.Second))
	if tracker.Observe(ack, now.Add(2*time.Second)) != nil {
		t.Fatal("mismatched transaction emitted an event")
	}
}

func fixtureFrame(messageType byte, transaction uint32, requestedIP, yourIP net.IP, hostname string) []byte {
	bootp := make([]byte, 240)
	bootp[0] = 2
	if messageType == MessageDiscover || messageType == MessageRequest {
		bootp[0] = 1
	}
	bootp[1] = 1
	bootp[2] = 6
	binary.BigEndian.PutUint32(bootp[4:8], transaction)
	if ip := yourIP.To4(); ip != nil {
		copy(bootp[16:20], ip)
	}
	copy(bootp[28:34], fixtureMAC)
	copy(bootp[236:240], []byte{0x63, 0x82, 0x53, 0x63})

	options := []byte{53, 1, messageType}
	if ip := requestedIP.To4(); ip != nil {
		options = append(options, 50, 4)
		options = append(options, ip...)
	}
	if hostname != "" {
		options = append(options, 12, byte(len(hostname)))
		options = append(options, hostname...)
	}
	options = append(options, 54, 4, 192, 168, 1, 1, 255)
	payload := append(bootp, options...)

	udp := make([]byte, 8+len(payload))
	sourcePort, destinationPort := uint16(67), uint16(68)
	if messageType == MessageDiscover || messageType == MessageRequest {
		sourcePort, destinationPort = 68, 67
	}
	binary.BigEndian.PutUint16(udp[0:2], sourcePort)
	binary.BigEndian.PutUint16(udp[2:4], destinationPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	copy(udp[8:], payload)

	ip := make([]byte, 20+len(udp))
	ip[0] = 0x45
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)))
	ip[8] = 64
	ip[9] = 17
	copy(ip[20:], udp)

	frame := make([]byte, 14+len(ip))
	copy(frame[0:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	copy(frame[6:12], fixtureMAC)
	binary.BigEndian.PutUint16(frame[12:14], 0x0800)
	copy(frame[14:], ip)
	return frame
}
