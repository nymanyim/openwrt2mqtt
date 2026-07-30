package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultInterface   = "br-lan"
	defaultMQTTBroker  = "127.0.0.1:1883"
	defaultTopicPrefix = "openwrt2mqtt"
	defaultMQTTTimeout = 10 * time.Second
	defaultBusCapacity = 128
)

// Runtime contains the Linux runtime configuration.
type Runtime struct {
	RouterID               string
	Interface              string
	BusCapacity            int
	DeviceConnectedEnabled bool
	MQTTBroker             string
	MQTTClientID           string
	MQTTUsername           string
	MQTTPassword           string
	MQTTTopic              string
	MQTTQoS                byte
	MQTTTimeout            time.Duration
}

func FromEnvironment() (Runtime, error) {
	config := Runtime{
		RouterID:               os.Getenv("OPENWRT2MQTT_ROUTER_ID"),
		Interface:              valueOrDefault("OPENWRT2MQTT_INTERFACE", defaultInterface),
		MQTTBroker:             valueOrDefault("OPENWRT2MQTT_MQTT_BROKER", defaultMQTTBroker),
		MQTTClientID:           os.Getenv("OPENWRT2MQTT_MQTT_CLIENT_ID"),
		MQTTUsername:           os.Getenv("OPENWRT2MQTT_MQTT_USERNAME"),
		MQTTPassword:           os.Getenv("OPENWRT2MQTT_MQTT_PASSWORD"),
		MQTTTopic:              valueOrDefault("OPENWRT2MQTT_MQTT_TOPIC", defaultTopicPrefix),
		MQTTTimeout:            defaultMQTTTimeout,
		BusCapacity:            defaultBusCapacity,
		DeviceConnectedEnabled: true,
	}
	if config.MQTTClientID == "" {
		config.MQTTClientID = config.RouterID
	}

	if value := os.Getenv("OPENWRT2MQTT_EVENT_DEVICE_CONNECTED_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return Runtime{}, fmt.Errorf("OPENWRT2MQTT_EVENT_DEVICE_CONNECTED_ENABLED must be a boolean")
		}
		config.DeviceConnectedEnabled = enabled
	}
	if value := os.Getenv("OPENWRT2MQTT_MQTT_QOS"); value != "" {
		qos, err := strconv.ParseUint(value, 10, 8)
		if err != nil || qos > 2 {
			return Runtime{}, fmt.Errorf("OPENWRT2MQTT_MQTT_QOS must be 0, 1, or 2")
		}
		config.MQTTQoS = byte(qos)
	}
	if value := os.Getenv("OPENWRT2MQTT_MQTT_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			return Runtime{}, fmt.Errorf("OPENWRT2MQTT_MQTT_TIMEOUT must be a positive duration")
		}
		config.MQTTTimeout = timeout
	}
	if value := os.Getenv("OPENWRT2MQTT_BUS_CAPACITY"); value != "" {
		capacity, err := strconv.Atoi(value)
		if err != nil || capacity <= 0 {
			return Runtime{}, fmt.Errorf("OPENWRT2MQTT_BUS_CAPACITY must be a positive integer")
		}
		config.BusCapacity = capacity
	}

	if config.RouterID == "" {
		return Runtime{}, errors.New("OPENWRT2MQTT_ROUTER_ID must not be empty")
	}
	if config.Interface == "" {
		return Runtime{}, errors.New("OPENWRT2MQTT_INTERFACE must not be empty")
	}
	if config.DeviceConnectedEnabled && config.MQTTBroker == "" {
		return Runtime{}, errors.New("OPENWRT2MQTT_MQTT_BROKER must not be empty when device connection events are enabled")
	}
	return config, nil
}

func valueOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
