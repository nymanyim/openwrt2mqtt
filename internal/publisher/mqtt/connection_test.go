package mqtt

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestConnectionConnectsAndDisconnectsWithoutPublishing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	packets := make(chan byte, 2)
	errors := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			errors <- acceptErr
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		packetType, _, readErr := readMQTTPacket(reader)
		if readErr != nil {
			errors <- readErr
			return
		}
		packets <- packetType
		if _, writeErr := connection.Write([]byte{0x20, 0x02, 0x00, 0x00}); writeErr != nil {
			errors <- writeErr
			return
		}
		connection.SetReadDeadline(time.Now().Add(2 * time.Second))
		packetType, _, readErr = readMQTTPacket(reader)
		if readErr != nil {
			errors <- readErr
			return
		}
		packets <- packetType
	}()

	latency, err := TestConnection(context.Background(), Config{
		Broker:   listener.Addr().String(),
		ClientID: "connection-test",
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if latency < 0 {
		t.Fatalf("latency = %s", latency)
	}

	select {
	case err := <-errors:
		t.Fatal(err)
	case first := <-packets:
		if first>>4 != 1 {
			t.Fatalf("first packet type = %d", first>>4)
		}
	}
	select {
	case err := <-errors:
		t.Fatal(err)
	case second := <-packets:
		if second>>4 != 14 {
			t.Fatalf("second packet type = %d; expected DISCONNECT", second>>4)
		}
		if second>>4 == 3 {
			t.Fatal("connection test published an MQTT message")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for MQTT disconnect")
	}
}

func TestNormalizeBroker(t *testing.T) {
	tests := map[string]string{
		"":                          "tcp://127.0.0.1:1883",
		"127.0.0.1:1883":            "tcp://127.0.0.1:1883",
		" tcp://broker.local:1883 ": "tcp://broker.local:1883",
		"ssl://broker.local:8883":   "ssl://broker.local:8883",
	}
	for input, expected := range tests {
		if actual := normalizeBroker(input); actual != expected {
			t.Fatalf("normalizeBroker(%q) = %q; want %q", input, actual, expected)
		}
	}
}

func TestConnectionTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			time.Sleep(time.Second)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = TestConnection(ctx, Config{
		Broker:   "tcp://" + listener.Addr().String(),
		ClientID: "timeout-test",
		Timeout:  500 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("TestConnection() expected an error")
	}
}

func readMQTTPacket(reader *bufio.Reader) (byte, []byte, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	remaining := 0
	multiplier := 1
	for {
		value, readErr := reader.ReadByte()
		if readErr != nil {
			return 0, nil, readErr
		}
		remaining += int(value&0x7f) * multiplier
		if value&0x80 == 0 {
			break
		}
		multiplier *= 128
	}
	body := make([]byte, remaining)
	if _, err := io.ReadFull(reader, body); err != nil {
		return 0, nil, err
	}
	return first, body, nil
}
