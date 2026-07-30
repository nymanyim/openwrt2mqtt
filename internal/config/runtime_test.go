package config

import "testing"

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

func TestFromEnvironmentRequiresRouterAndBroker(t *testing.T) {
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("FromEnvironment() expected an error")
	}
}
