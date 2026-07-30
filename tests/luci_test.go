package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLuCIMenuAndACL(t *testing.T) {
	menuPath := repoPath(t, "package", "luci-app-openwrt2mqtt", "root", "usr", "share", "luci", "menu.d", "luci-app-openwrt2mqtt.json")
	aclPath := repoPath(t, "package", "luci-app-openwrt2mqtt", "root", "usr", "share", "rpcd", "acl.d", "luci-app-openwrt2mqtt.json")

	var menu map[string]struct {
		Title  string `json:"title"`
		Action struct {
			Type string `json:"type"`
			Path string `json:"path"`
		} `json:"action"`
	}
	readJSON(t, menuPath, &menu)
	entry, ok := menu["admin/services/openwrt2mqtt"]
	if !ok || entry.Title != "OpenWrt Event Publisher" || entry.Action.Type != "view" || entry.Action.Path != "openwrt2mqtt/settings" {
		t.Fatalf("unexpected menu entry: %#v", entry)
	}

	var acl map[string]struct {
		Read struct {
			UCI  []string            `json:"uci"`
			UBus map[string][]string `json:"ubus"`
		} `json:"read"`
		Write struct {
			UCI  []string            `json:"uci"`
			UBus map[string][]string `json:"ubus"`
		} `json:"write"`
	}
	readJSON(t, aclPath, &acl)
	permissions, ok := acl["luci-app-openwrt2mqtt"]
	if !ok {
		t.Fatal("ACL entry is missing")
	}
	if strings.Join(permissions.Read.UCI, ",") != "openwrt2mqtt" || strings.Join(permissions.Write.UCI, ",") != "openwrt2mqtt" {
		t.Fatalf("unexpected UCI permissions: %#v", permissions)
	}
	if strings.Join(permissions.Read.UBus["openwrt2mqtt"], ",") != "status" {
		t.Fatalf("unexpected read ubus permissions: %#v", permissions.Read.UBus)
	}
	if strings.Join(permissions.Write.UBus["openwrt2mqtt"], ",") != "reload,test_mqtt" {
		t.Fatalf("unexpected write ubus permissions: %#v", permissions.Write.UBus)
	}

	content, err := os.ReadFile(aclPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"file"`, `"exec"`, `"network"`, `"firewall"`, `"dhcp"`} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("ACL contains forbidden permission %s", forbidden)
		}
	}
}

func TestLuCISettingsPage(t *testing.T) {
	path := repoPath(t, "package", "luci-app-openwrt2mqtt", "htdocs", "luci-static", "resources", "view", "openwrt2mqtt", "settings.js")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)

	for _, expected := range []string{
		"new form.Map('openwrt2mqtt'",
		"form.NamedSection, 'main', 'openwrt2mqtt'",
		"s.tab('quick', _('Quick setup'))",
		"s.tab('advanced', _('Advanced settings'))",
		"s.taboption('quick', form.Flag, 'enabled'",
		"bindOption(s.taboption('quick', form.Value, '_broker'",
		"bindOption(s.taboption('advanced', form.Flag, '_device_event_enabled'",
		"method: 'status'",
		"method: 'reload'",
		"method: 'test_mqtt'",
		"o.password = true",
		"o.cfgvalue = function() { return ''; }",
		"o.remove = function() {}",
		"if (value)",
		"uci.set('openwrt2mqtt', 'mqtt', 'password', value)",
		"configuration_invalid",
		"connection_failed",
		"timeout",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("settings page is missing %q", expected)
		}
	}
	for _, forbidden := range []string{
		"uci.get('openwrt2mqtt', 'mqtt', 'password')",
		"fs.exec(",
		"fs.read(",
		"/etc/init.d/openwrt2mqtt",
		"OPENWRT2MQTT_MQTT_PASSWORD",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("settings page contains forbidden string %q", forbidden)
		}
	}

	if node, err := exec.LookPath("node"); err == nil {
		command := exec.Command(node, "--check", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("node --check: %v\n%s", err, output)
		}
	}
}

func TestLuCISimplifiedChineseTranslation(t *testing.T) {
	path := repoPath(t, "package", "luci-app-openwrt2mqtt", "po", "zh_Hans", "openwrt2mqtt.po")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)

	for _, expected := range []string{
		"Language: zh_Hans",
		"msgid \"OpenWrt Event Publisher\"",
		"msgstr \"OpenWrt 事件推送\"",
		"msgid \"Quick setup\"",
		"msgstr \"常用设置\"",
		"msgid \"Advanced settings\"",
		"msgstr \"高级设置\"",
		"msgid \"MQTT server\"",
		"msgstr \"MQTT 服务器\"",
		"msgid \"Leave empty to keep the saved password.\"",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Simplified Chinese translation is missing %q", expected)
		}
	}
}

func TestStage5PackageMakefiles(t *testing.T) {
	checks := map[string][]string{
		repoPath(t, "package", "openwrt2mqtt", "Makefile"): {
			"PKG_VERSION:=1.0.0",
			"GO_PKG_BUILD_PKG:=$(GO_PKG)/cmd/openwrt2mqtt",
			"GO_PKG_LDFLAGS_X:=main.version=$(PKG_VERSION)",
			"DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle +procd +rpcd +uci",
			"/etc/config/openwrt2mqtt",
			"$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/openwrt2mqtt $(1)/usr/sbin/openwrt2mqtt",
		},
		repoPath(t, "package", "luci-app-openwrt2mqtt", "Makefile"): {
			"PKG_VERSION:=1.0.0",
			"LUCI_DEPENDS:=+luci-base +openwrt2mqtt",
			"include $(TOPDIR)/feeds/luci/luci.mk",
		},
	}

	for path, expectedStrings := range checks {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, expected := range expectedStrings {
			if !strings.Contains(text, expected) {
				t.Fatalf("%s is missing %q", path, expected)
			}
		}
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
