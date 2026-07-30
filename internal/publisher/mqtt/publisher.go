package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

const (
	defaultTopicPrefix = "openwrt2mqtt"
	defaultTimeout     = 10 * time.Second
)

// ErrOperationTimeout indicates that an MQTT operation exceeded its configured timeout.
var ErrOperationTimeout = errors.New("MQTT operation timed out")

// Config contains MQTT transport settings.
type Config struct {
	Broker      string
	ClientID    string
	Username    string
	Password    string
	TopicPrefix string
	QoS         byte
	Timeout     time.Duration
}

// Publisher serializes normalized events and publishes them through Paho.
type Publisher struct {
	client      paho.Client
	topicPrefix string
	qos         byte
	timeout     time.Duration
	closeOnce   sync.Once
}

func New(ctx context.Context, config Config) (*Publisher, error) {
	client, err := connect(ctx, config)
	if err != nil {
		return nil, err
	}
	return newWithClient(client, config), nil
}

// TestConnection connects to the broker and immediately disconnects.
func TestConnection(ctx context.Context, config Config) (time.Duration, error) {
	started := time.Now()
	client, err := connect(ctx, config)
	if err != nil {
		return 0, err
	}
	latency := time.Since(started)
	client.Disconnect(250)
	return latency, nil
}

func connect(ctx context.Context, config Config) (paho.Client, error) {
	if config.Broker == "" {
		return nil, errors.New("MQTT broker must not be empty")
	}
	if config.ClientID == "" {
		return nil, errors.New("MQTT client ID must not be empty")
	}
	if config.QoS > 2 {
		return nil, fmt.Errorf("MQTT QoS must be between 0 and 2")
	}
	if config.TopicPrefix == "" {
		config.TopicPrefix = defaultTopicPrefix
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}

	options := paho.NewClientOptions().
		AddBroker(config.Broker).
		SetClientID(config.ClientID).
		SetUsername(config.Username).
		SetPassword(config.Password).
		SetCleanSession(true).
		SetOrderMatters(false).
		SetConnectTimeout(config.Timeout).
		SetWriteTimeout(config.Timeout)
	client := paho.NewClient(options)
	if err := waitToken(ctx, client.Connect(), config.Timeout); err != nil {
		return nil, fmt.Errorf("connect MQTT broker: %w", err)
	}

	return client, nil
}

func newWithClient(client paho.Client, config Config) *Publisher {
	if config.TopicPrefix == "" {
		config.TopicPrefix = defaultTopicPrefix
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	return &Publisher{
		client:      client,
		topicPrefix: strings.Trim(config.TopicPrefix, "/"),
		qos:         config.QoS,
		timeout:     config.Timeout,
	}
}

func (p *Publisher) Publish(ctx context.Context, message event.Event) error {
	if p.client == nil {
		return errors.New("MQTT client must not be nil")
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	topic := eventTopic(p.topicPrefix, message)
	return waitToken(ctx, p.client.Publish(topic, p.qos, false, payload), p.timeout)
}

func (p *Publisher) Close() error {
	p.closeOnce.Do(func() {
		if p.client != nil && p.client.IsConnectionOpen() {
			p.client.Disconnect(1000)
		}
	})
	return nil
}

func eventTopic(prefix string, message event.Event) string {
	eventType := strings.ReplaceAll(message.Type, ".", "/")
	return strings.Join([]string{
		strings.Trim(prefix, "/"),
		message.RouterID,
		message.Category,
		eventType,
	}, "/")
}

func waitToken(ctx context.Context, token paho.Token, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("%w after %s", ErrOperationTimeout, timeout)
	}
}
