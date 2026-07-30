package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/config"
	mqttPublisher "github.com/nymanyim/openwrt2mqtt/internal/publisher/mqtt"
)

var (
	testMQTTConnection = mqttPublisher.TestConnection
	readRandom         = rand.Read
)

func handleCommand(ctx context.Context, arguments []string, stdout, stderr io.Writer) (bool, int) {
	if len(arguments) == 0 {
		return false, 0
	}
	switch arguments[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return true, 0
	case "mqtt-test":
		if err := runMQTTTest(ctx, stdout); err != nil {
			return true, 1
		}
		return true, 0
	default:
		fmt.Fprintln(stderr, "unknown command")
		return true, 2
	}
}

type mqttTestResult struct {
	Success   bool   `json:"success"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runMQTTTest(ctx context.Context, output io.Writer) error {
	runtimeConfig, err := config.FromEnvironment()
	if err == nil && runtimeConfig.MQTTBroker == "" {
		err = errors.New("MQTT broker must not be empty")
	}
	if err != nil {
		if writeErr := writeMQTTTestResult(output, mqttTestResult{Error: "configuration_invalid"}); writeErr != nil {
			return writeErr
		}
		return err
	}

	latency, err := testMQTTConnection(ctx, mqttPublisher.Config{
		Broker:   runtimeConfig.MQTTBroker,
		ClientID: mqttTestClientID(),
		Username: runtimeConfig.MQTTUsername,
		Password: runtimeConfig.MQTTPassword,
		Timeout:  runtimeConfig.MQTTTimeout,
	})
	if err != nil {
		category := "connection_failed"
		if errors.Is(err, mqttPublisher.ErrOperationTimeout) || errors.Is(err, context.DeadlineExceeded) {
			category = "timeout"
		}
		if writeErr := writeMQTTTestResult(output, mqttTestResult{Error: category}); writeErr != nil {
			return writeErr
		}
		return err
	}

	return writeMQTTTestResult(output, mqttTestResult{
		Success:   true,
		LatencyMS: max(latency.Milliseconds(), 0),
	})
}

func mqttTestClientID() string {
	random := make([]byte, 6)
	if _, err := readRandom(random); err == nil {
		return "openwrt2mqtt-test-" + hex.EncodeToString(random)
	}
	return fmt.Sprintf("openwrt2mqtt-test-%x", time.Now().UnixNano())
}

func writeMQTTTestResult(output io.Writer, result mqttTestResult) error {
	return json.NewEncoder(output).Encode(result)
}
