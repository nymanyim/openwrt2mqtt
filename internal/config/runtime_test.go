package config

import (
	"strings"
	"testing"
	"time"
)

func TestFromEnvironment(t *testing.T) {
	t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
	t.Setenv("OPENWRT2MQTT_MQTT_BROKER", "tcp://127.0.0.1:1883")
	t.Setenv("OPENWRT2MQTT_MQTT_QOS", "1")

	config, err := FromEnvironment()
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}
	if config.Interface != "br-lan" || config.MQTTBroker != "tcp://127.0.0.1:1883" || config.MQTTClientID != "router-a" || config.MQTTQoS != 1 || !config.DeviceConnectedEnabled {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestFromEnvironmentParsesDeviceConnectedFlag(t *testing.T) {
	t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
	t.Setenv("OPENWRT2MQTT_MQTT_BROKER", "tcp://127.0.0.1:1883")
	t.Setenv("OPENWRT2MQTT_EVENT_DEVICE_CONNECTED_ENABLED", "false")

	config, err := FromEnvironment()
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}
	if config.DeviceConnectedEnabled {
		t.Fatal("DeviceConnectedEnabled = true")
	}
}

func TestFromEnvironmentUsesDefaultBroker(t *testing.T) {
	t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")

	config, err := FromEnvironment()
	if err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}
	if config.MQTTBroker != defaultMQTTBroker {
		t.Fatalf("MQTTBroker = %q", config.MQTTBroker)
	}
}

func TestFromEnvironmentAllowsMissingBrokerWhenDeviceEventIsDisabled(t *testing.T) {
	t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
	t.Setenv("OPENWRT2MQTT_EVENT_DEVICE_CONNECTED_ENABLED", "false")

	if _, err := FromEnvironment(); err != nil {
		t.Fatalf("FromEnvironment() error = %v", err)
	}
}

func TestFromEnvironmentRejectsInvalidDeviceConnectedFlag(t *testing.T) {
	t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
	t.Setenv("OPENWRT2MQTT_MQTT_BROKER", "tcp://127.0.0.1:1883")
	t.Setenv("OPENWRT2MQTT_EVENT_DEVICE_CONNECTED_ENABLED", "invalid")

	if _, err := FromEnvironment(); err == nil {
		t.Fatal("FromEnvironment() expected an error")
	}
}

func TestFromEnvironmentAcceptsMinimumOfflineTimeout(t *testing.T) {
	for _, value := range []string{"3s", "3000ms", "0.05m"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
			t.Setenv("OPENWRT2MQTT_OFFLINE_TIMEOUT", value)

			config, err := FromEnvironment()
			if err != nil {
				t.Fatalf("FromEnvironment() error = %v", err)
			}
			if config.OfflineTimeout != 3*time.Second {
				t.Fatalf("OfflineTimeout = %s, want 3s", config.OfflineTimeout)
			}
		})
	}
}

func TestFromEnvironmentRejectsOfflineTimeoutBelowMinimum(t *testing.T) {
	for _, value := range []string{"1s", "2s", "2999ms"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
			t.Setenv("OPENWRT2MQTT_OFFLINE_TIMEOUT", value)

			_, err := FromEnvironment()
			if err == nil {
				t.Fatal("FromEnvironment() expected an error")
			}
			if !strings.Contains(err.Error(), "must be at least 3s") {
				t.Fatalf("error = %q", err)
			}
		})
	}
}

func TestFromEnvironmentRejectsInvalidOfflineTimeout(t *testing.T) {
	t.Setenv("OPENWRT2MQTT_ROUTER_ID", "router-a")
	t.Setenv("OPENWRT2MQTT_OFFLINE_TIMEOUT", "invalid")

	_, err := FromEnvironment()
	if err == nil {
		t.Fatal("FromEnvironment() expected an error")
	}
	if !strings.Contains(err.Error(), "must be a valid duration") {
		t.Fatalf("error = %q", err)
	}
}

func TestFromEnvironmentRequiresRouterAndBroker(t *testing.T) {
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("FromEnvironment() expected an error")
	}
}
