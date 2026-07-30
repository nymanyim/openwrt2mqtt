package mqtt

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

type completedToken struct {
	done chan struct{}
	err  error
}

func newCompletedToken(err error) *completedToken {
	done := make(chan struct{})
	close(done)
	return &completedToken{done: done, err: err}
}
func (t *completedToken) Wait() bool                     { return true }
func (t *completedToken) WaitTimeout(time.Duration) bool { return true }
func (t *completedToken) Done() <-chan struct{}          { return t.done }
func (t *completedToken) Error() error                   { return t.err }

type fakeClient struct {
	topic   string
	qos     byte
	payload []byte
}

func (c *fakeClient) IsConnected() bool      { return true }
func (c *fakeClient) IsConnectionOpen() bool { return true }
func (c *fakeClient) Connect() paho.Token    { return newCompletedToken(nil) }
func (c *fakeClient) Disconnect(uint)        {}
func (c *fakeClient) Publish(topic string, qos byte, _ bool, payload interface{}) paho.Token {
	c.topic = topic
	c.qos = qos
	c.payload = append([]byte(nil), payload.([]byte)...)
	return newCompletedToken(nil)
}
func (c *fakeClient) Subscribe(string, byte, paho.MessageHandler) paho.Token {
	return newCompletedToken(nil)
}
func (c *fakeClient) SubscribeMultiple(map[string]byte, paho.MessageHandler) paho.Token {
	return newCompletedToken(nil)
}
func (c *fakeClient) Unsubscribe(...string) paho.Token     { return newCompletedToken(nil) }
func (c *fakeClient) AddRoute(string, paho.MessageHandler) {}
func (c *fakeClient) OptionsReader() paho.ClientOptionsReader {
	options := paho.NewClientOptions()
	options.Servers = []*url.URL{}
	return paho.NewOptionsReader(options)
}

func TestPublishUsesNormalizedTopicAndJSON(t *testing.T) {
	client := &fakeClient{}
	output := newWithClient(client, Config{TopicPrefix: "routers", QoS: 1, Timeout: time.Second})
	message := event.Event{
		SchemaVersion: "1",
		ID:            "event-1",
		RouterID:      "router-a",
		Category:      "network",
		Type:          "device.connected",
		Source:        "dhcp/br-lan",
		Timestamp:     time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
		Data:          map[string]any{"mac": "02:11:22:33:44:55"},
	}

	if err := output.Publish(context.Background(), message); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if client.topic != "routers/router-a/network/device/connected" {
		t.Fatalf("topic = %q", client.topic)
	}
	if client.qos != 1 {
		t.Fatalf("QoS = %d", client.qos)
	}
	var decoded event.Event
	if err := json.Unmarshal(client.payload, &decoded); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if decoded.ID != message.ID || decoded.Data["mac"] != message.Data["mac"] {
		t.Fatalf("decoded event = %#v", decoded)
	}
}
