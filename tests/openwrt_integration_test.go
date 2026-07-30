package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const initHarness = `
config_load() { return 0; }
config_get_bool() {
	case "$2.$3" in
		main.enabled) value="${CFG_ENABLED-1}" ;;
		network_device_connected.enabled) value="${CFG_DEVICE_CONNECTED_ENABLED-1}" ;;
		*) value="$4" ;;
	esac
	eval "$1=\$value"
}
config_get() {
	case "$2.$3" in
		main.router_id) value="${CFG_ROUTER_ID-router-a}" ;;
		main.interface) value="${CFG_INTERFACE-br-lan}" ;;
		main.log_level) value="${CFG_LOG_LEVEL-info}" ;;
		main.bus_capacity) value="${CFG_BUS_CAPACITY-128}" ;;
		mqtt.broker) value="${CFG_BROKER-127.0.0.1:1883}" ;;
		mqtt.client_id) value="${CFG_CLIENT_ID-client-a}" ;;
		mqtt.username) value="${CFG_USERNAME-user-a}" ;;
		mqtt.password) value="${CFG_PASSWORD-secret-value}" ;;
		mqtt.topic) value="${CFG_TOPIC-openwrt2mqtt}" ;;
		mqtt.qos) value="${CFG_QOS-1}" ;;
		mqtt.timeout) value="${CFG_TIMEOUT-10s}" ;;
		*) value="$4" ;;
	esac
	eval "$1=\$value"
}
uci() { printf '%s\n' fallback-router; }
logger() { printf 'LOGGER %s\n' "$*" >&2; }
procd_open_instance() { printf 'OPEN %s\n' "$*"; }
procd_close_instance() { printf 'CLOSE\n'; }
procd_set_param() { printf 'PARAM'; for value in "$@"; do printf ' <%s>' "$value"; done; printf '\n'; }
procd_add_reload_trigger() { printf 'TRIGGER %s\n' "$*"; }
stop() { printf 'STOP\n'; }
start() { printf 'START\n'; }
. "$INIT_SCRIPT"
"${HARNESS_ACTION:-start_service}"
`

func TestOpenWrtScriptsHaveValidShellSyntax(t *testing.T) {
	for _, path := range []string{initScriptPath(t), rpcScriptPath(t)} {
		command := exec.Command("sh", "-n", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("sh -n %s: %v\n%s", path, err, output)
		}
	}
}

func TestUCIConfigDefaultsDisabled(t *testing.T) {
	content, err := os.ReadFile(repoPath(t, "package", "openwrt2mqtt", "files", "etc", "config", "openwrt2mqtt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		"option enabled '0'",
		"option interface 'br-lan'",
		"option bus_capacity '128'",
		"config event 'network_device_connected'",
		"option enabled '1'",
		"option broker '127.0.0.1:1883'",
		"option topic 'openwrt2mqtt'",
		"option qos '0'",
		"option timeout '10s'",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("UCI config is missing %q", expected)
		}
	}
}

func TestInitScriptMapsUCIToEnvironment(t *testing.T) {
	output, err := runInitHarness(t, nil)
	if err != nil {
		t.Fatalf("start_service: %v\n%s", err, output)
	}
	for _, expected := range []string{
		"<OPENWRT2MQTT_ROUTER_ID=router-a>",
		"<OPENWRT2MQTT_INTERFACE=br-lan>",
		"<OPENWRT2MQTT_LOG_LEVEL=info>",
		"<OPENWRT2MQTT_BUS_CAPACITY=128>",
		"<OPENWRT2MQTT_EVENT_DEVICE_CONNECTED_ENABLED=1>",
		"<OPENWRT2MQTT_MQTT_BROKER=127.0.0.1:1883>",
		"<OPENWRT2MQTT_MQTT_CLIENT_ID=client-a>",
		"<OPENWRT2MQTT_MQTT_USERNAME=user-a>",
		"<OPENWRT2MQTT_MQTT_PASSWORD=secret-value>",
		"<OPENWRT2MQTT_MQTT_TOPIC=openwrt2mqtt>",
		"<OPENWRT2MQTT_MQTT_QOS=1>",
		"<OPENWRT2MQTT_MQTT_TIMEOUT=10s>",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("procd environment is missing %q\n%s", expected, output)
		}
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "PARAM <command>") && strings.Contains(line, "secret-value") {
			t.Fatalf("MQTT password leaked into command parameters: %s", line)
		}
	}
}

func TestInitScriptUsesSystemHostnameFallback(t *testing.T) {
	output, err := runInitHarness(t, map[string]string{
		"CFG_ROUTER_ID": "",
		"CFG_CLIENT_ID": "",
	})
	if err != nil {
		t.Fatalf("start_service: %v\n%s", err, output)
	}
	for _, expected := range []string{
		"<OPENWRT2MQTT_ROUTER_ID=fallback-router>",
		"<OPENWRT2MQTT_MQTT_CLIENT_ID=fallback-router>",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("fallback environment is missing %q\n%s", expected, output)
		}
	}
}

func TestInitScriptAcceptsGoDurations(t *testing.T) {
	for _, duration := range []string{"0.5s", "1h30m", "250ms"} {
		output, err := runInitHarness(t, map[string]string{"CFG_TIMEOUT": duration})
		if err != nil {
			t.Fatalf("timeout %q: %v\n%s", duration, err, output)
		}
	}
}

func TestInitScriptDoesNotStartWhenDisabled(t *testing.T) {
	output, err := runInitHarness(t, map[string]string{
		"CFG_ENABLED": "0",
		"CFG_BROKER":  "",
	})
	if err != nil {
		t.Fatalf("disabled start_service: %v\n%s", err, output)
	}
	if strings.Contains(output, "PARAM <command>") {
		t.Fatalf("disabled service registered a command:\n%s", output)
	}
}

func TestInitScriptMQTTTestUsesEnvironmentWithoutPasswordArguments(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "openwrt2mqtt")
	binary := `#!/bin/sh
[ "$#" -eq 1 ] || exit 11
[ "$1" = "mqtt-test" ] || exit 12
[ "$OPENWRT2MQTT_MQTT_PASSWORD" = "secret-value" ] || exit 13
[ "$OPENWRT2MQTT_EVENT_DEVICE_CONNECTED_ENABLED" = "1" ] || exit 14
printf '%s\n' '{"success":true,"latency_ms":5}'
`
	if err := os.WriteFile(binaryPath, []byte(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	output, err := runInitHarness(t, map[string]string{
		"HARNESS_ACTION":      "mqtt_test",
		"OPENWRT2MQTT_BINARY": binaryPath,
	})
	if err != nil {
		t.Fatalf("mqtt_test: %v\n%s", err, output)
	}
	if strings.TrimSpace(output) != `{"success":true,"latency_ms":5}` {
		t.Fatalf("output = %q", output)
	}
}

func TestInitScriptUsesDefaultBrokerWhenEmpty(t *testing.T) {
	output, err := runInitHarness(t, map[string]string{
		"CFG_BROKER": "",
	})
	if err != nil {
		t.Fatalf("start_service: %v\n%s", err, output)
	}
	if !strings.Contains(output, "<OPENWRT2MQTT_MQTT_BROKER=127.0.0.1:1883>") {
		t.Fatalf("default MQTT broker was not registered:\n%s", output)
	}
}

func TestInitScriptReloadValidatesBeforeStopping(t *testing.T) {
	output, err := runInitHarness(t, map[string]string{
		"HARNESS_ACTION": "reload_service",
		"CFG_QOS":        "3",
	})
	if err == nil {
		t.Fatalf("reload_service accepted invalid configuration:\n%s", output)
	}
	if strings.Contains(output, "STOP") || strings.Contains(output, "START") {
		t.Fatalf("invalid reload changed service state:\n%s", output)
	}
}

func TestInitScriptRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "invalid QoS", env: map[string]string{"CFG_QOS": "3"}},
		{name: "invalid log level", env: map[string]string{"CFG_LOG_LEVEL": "trace"}},
		{name: "invalid bus capacity", env: map[string]string{"CFG_BUS_CAPACITY": "0"}},
		{name: "invalid timeout", env: map[string]string{"CFG_TIMEOUT": "0s"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := runInitHarness(t, test.env)
			if err == nil {
				t.Fatalf("start_service accepted invalid configuration:\n%s", output)
			}
			if strings.Contains(output, "PARAM <command>") {
				t.Fatalf("invalid configuration registered a command:\n%s", output)
			}
		})
	}
}

func TestRPCDProtocol(t *testing.T) {
	functionsPath := filepath.Join(t.TempDir(), "functions.sh")
	functions := `
config_load() { return 0; }
config_get_bool() {
	case "$2.$3" in
		main.enabled) value="${RPC_ENABLED:-1}" ;;
		network_device_connected.enabled) value="${RPC_DEVICE_CONNECTED_ENABLED:-1}" ;;
		*) value="$4" ;;
	esac
	eval "$1=\$value"
}
`
	if err := os.WriteFile(functionsPath, []byte(functions), 0o600); err != nil {
		t.Fatal(err)
	}
	initPath := filepath.Join(t.TempDir(), "openwrt2mqtt-init")
	init := `#!/bin/sh
case "$1" in
	running) [ "${RPC_RUNNING:-0}" = "1" ] ;;
	configured) [ "${RPC_CONFIGURED:-0}" = "1" ] ;;
	validate) [ "${RPC_VALIDATE_OK:-0}" = "1" ] ;;
	reload) [ "${RPC_RELOAD_OK:-0}" = "1" ] ;;
	mqtt_test) printf '%s\n' "${RPC_MQTT_RESULT:-{\"success\":true,\"latency_ms\":5}}" ;;
esac
`
	if err := os.WriteFile(initPath, []byte(init), 0o700); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(t.TempDir(), "openwrt2mqtt")
	binary := `#!/bin/sh
[ "$1" = "version" ] && printf '%s\n' "${RPC_VERSION:-1.2.3}"
`
	if err := os.WriteFile(binaryPath, []byte(binary), 0o700); err != nil {
		t.Fatal(err)
	}

	environment := map[string]string{
		"OPENWRT2MQTT_FUNCTIONS_SH": functionsPath,
		"OPENWRT2MQTT_INIT_SCRIPT":  initPath,
		"OPENWRT2MQTT_BINARY":       binaryPath,
	}
	list := runRPCD(t, environment, "list")
	var signature map[string]map[string]string
	if err := json.Unmarshal([]byte(list), &signature); err != nil {
		t.Fatalf("invalid list response: %v\n%s", err, list)
	}
	if _, ok := signature["status"]; !ok {
		t.Fatal("list response is missing status")
	}
	if _, ok := signature["reload"]; !ok {
		t.Fatal("list response is missing reload")
	}
	if _, ok := signature["test_mqtt"]; !ok {
		t.Fatal("list response is missing test_mqtt")
	}

	environment["RPC_ENABLED"] = "1"
	environment["RPC_RUNNING"] = "1"
	environment["RPC_CONFIGURED"] = "1"
	environment["RPC_DEVICE_CONNECTED_ENABLED"] = "1"
	status := runRPCD(t, environment, "call", "status")
	var state struct {
		Running                bool   `json:"running"`
		Enabled                bool   `json:"enabled"`
		Configured             bool   `json:"configured"`
		DeviceConnectedEnabled bool   `json:"device_connected_enabled"`
		Version                string `json:"version"`
	}
	if err := json.Unmarshal([]byte(status), &state); err != nil {
		t.Fatalf("invalid status response: %v\n%s", err, status)
	}
	if !state.Running || !state.Enabled || !state.Configured || !state.DeviceConnectedEnabled || state.Version != "1.2.3" {
		t.Fatalf("unexpected status: %#v", state)
	}
	for _, secret := range []string{"password", "username", "broker"} {
		if strings.Contains(status, secret) {
			t.Fatalf("status leaks %q: %s", secret, status)
		}
	}

	environment["RPC_VALIDATE_OK"] = "1"
	environment["RPC_RELOAD_OK"] = "1"
	reload := runRPCD(t, environment, "call", "reload")
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal([]byte(reload), &result); err != nil || !result.Success {
		t.Fatalf("unexpected reload response: %v %q", err, reload)
	}

	mqttResult := runRPCD(t, environment, "call", "test_mqtt")
	var mqttState struct {
		Success   bool  `json:"success"`
		LatencyMS int64 `json:"latency_ms"`
	}
	if err := json.Unmarshal([]byte(mqttResult), &mqttState); err != nil || !mqttState.Success || mqttState.LatencyMS != 5 {
		t.Fatalf("unexpected MQTT test response: %v %q", err, mqttResult)
	}

	environment["RPC_RELOAD_OK"] = "0"
	command := rpcCommand(t, environment, "call", "reload")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("reload failure returned success: %s", output)
	}

	environment["RPC_VALIDATE_OK"] = "0"
	environment["RPC_RELOAD_OK"] = "1"
	command = rpcCommand(t, environment, "call", "reload")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("invalid configuration returned reload success: %s", output)
	}
}

func runInitHarness(t *testing.T, environment map[string]string) (string, error) {
	t.Helper()
	command := exec.Command("sh", "-c", initHarness)
	command.Env = append(os.Environ(), "INIT_SCRIPT="+initScriptPath(t))
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}
	output, err := command.CombinedOutput()
	return string(output), err
}

func runRPCD(t *testing.T, environment map[string]string, arguments ...string) string {
	t.Helper()
	command := rpcCommand(t, environment, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("rpcd %v: %v\n%s", arguments, err, output)
	}
	return strings.TrimSpace(string(output))
}

func rpcCommand(t *testing.T, environment map[string]string, arguments ...string) *exec.Cmd {
	t.Helper()
	commandArguments := append([]string{rpcScriptPath(t)}, arguments...)
	command := exec.Command("sh", commandArguments...)
	command.Env = os.Environ()
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}
	return command
}

func initScriptPath(t *testing.T) string {
	t.Helper()
	return repoPath(t, "package", "openwrt2mqtt", "files", "etc", "init.d", "openwrt2mqtt")
}

func rpcScriptPath(t *testing.T) string {
	t.Helper()
	return repoPath(t, "package", "openwrt2mqtt", "files", "usr", "libexec", "rpcd", "openwrt2mqtt")
}

func repoPath(t *testing.T, elements ...string) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repository path")
	}
	root := filepath.Dir(filepath.Dir(source))
	return filepath.Join(append([]string{root}, elements...)...)
}
