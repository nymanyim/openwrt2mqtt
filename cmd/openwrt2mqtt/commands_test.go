package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	mqttPublisher "github.com/nymanyim/openwrt2mqtt/internal/publisher/mqtt"
)

func setMQTTEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
	t.Setenv("OPENWRT2MQTT_MQTT_BROKER", "tcp://127.0.0.1:1883")
	t.Setenv("OPENWRT2MQTT_MQTT_CLIENT_ID", "client-a")
	t.Setenv("OPENWRT2MQTT_MQTT_PASSWORD", "secret-value")
}

func TestHandleCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, exitCode := handleCommand(context.Background(), []string{"version"}, &stdout, &stderr)
	if !handled || exitCode != 0 || stdout.String() != "dev\n" || stderr.Len() != 0 {
		t.Fatalf("version command: handled=%v exit=%d stdout=%q stderr=%q", handled, exitCode, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	handled, exitCode = handleCommand(context.Background(), []string{"unknown"}, &stdout, &stderr)
	if !handled || exitCode != 2 || stdout.Len() != 0 || stderr.String() != "unknown command\n" {
		t.Fatalf("unknown command: handled=%v exit=%d stdout=%q stderr=%q", handled, exitCode, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	handled, exitCode = handleCommand(context.Background(), nil, &stdout, &stderr)
	if handled || exitCode != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("no command: handled=%v exit=%d stdout=%q stderr=%q", handled, exitCode, stdout.String(), stderr.String())
	}
}

func TestRunMQTTTestSuccess(t *testing.T) {
	setMQTTEnvironment(t)
	original := testMQTTConnection
	t.Cleanup(func() { testMQTTConnection = original })
	testMQTTConnection = func(_ context.Context, config mqttPublisher.Config) (time.Duration, error) {
		if config.ClientID == "client-a" || !strings.HasPrefix(config.ClientID, "openwrt2mqtt-test-") {
			t.Fatalf("test Client ID = %q", config.ClientID)
		}
		return 35 * time.Millisecond, nil
	}

	var output bytes.Buffer
	if err := runMQTTTest(context.Background(), &output); err != nil {
		t.Fatalf("runMQTTTest() error = %v", err)
	}
	if output.String() != "{\"success\":true,\"latency_ms\":35}\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunMQTTTestClassifiesErrorsWithoutSecrets(t *testing.T) {
	setMQTTEnvironment(t)
	original := testMQTTConnection
	t.Cleanup(func() { testMQTTConnection = original })

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{name: "timeout", err: mqttPublisher.ErrOperationTimeout, expected: "timeout"},
		{name: "connection", err: errors.New("secret-value authentication failed"), expected: "connection_failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testMQTTConnection = func(context.Context, mqttPublisher.Config) (time.Duration, error) {
				return 0, test.err
			}
			var output bytes.Buffer
			if err := runMQTTTest(context.Background(), &output); err == nil {
				t.Fatal("runMQTTTest() expected an error")
			}
			if !strings.Contains(output.String(), `"error":"`+test.expected+`"`) {
				t.Fatalf("output = %q", output.String())
			}
			if strings.Contains(output.String(), "secret-value") {
				t.Fatalf("password leaked in output: %q", output.String())
			}
		})
	}
}

func TestRunMQTTTestRejectsInvalidConfiguration(t *testing.T) {
	var output bytes.Buffer
	if err := runMQTTTest(context.Background(), &output); err == nil {
		t.Fatal("runMQTTTest() expected an error")
	}
	if output.String() != "{\"success\":false,\"error\":\"configuration_invalid\"}\n" {
		t.Fatalf("output = %q", output.String())
	}
}
