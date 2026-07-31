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
	if !ok || entry.Title != "OpenWrt to MQTT" || entry.Action.Type != "view" || entry.Action.Path != "openwrt2mqtt/settings" {
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
		"_('Publish OpenWrt system events to MQTT.')",
		"form.NamedSection, 'main', 'openwrt2mqtt'",
		"s.tab('quick', _('Quick setup'))",
		"s.tab('events', _('Event settings'))",
		"s.tab('advanced', _('Advanced settings'))",
		"s.taboption('quick', form.Flag, 'enabled'",
		"bindOption(s.taboption('quick', form.Value, '_broker'",
		"bindOption(s.taboption('quick', form.Value, '_username'",
		"function ensureSection(sectionName, sectionType)",
		"uci.add('openwrt2mqtt', sectionType, sectionName)",
		"ensureSection(sectionName, sectionType)",
		"option.load = function() {",
		"o.default = '127.0.0.1:1883'",
		"o.placeholder = '127.0.0.1:1883'",
		"return uci.get('openwrt2mqtt', 'mqtt', 'broker') || '127.0.0.1:1883'",
		"bindOption(s.taboption('events', form.Flag, '_device_event_enabled'",
		"bindOption(s.taboption('events', form.Flag, '_device_disconnected_enabled'",
		"function enhanceSettingsForm(node)",
		"attachMessageExampleButton(node, '_device_event_enabled', 'device.connected')",
		"attachMessageExampleButton(node, '_device_disconnected_enabled', 'device.disconnected')",
		"var renderContents = m.renderContents.bind(m)",
		"m.renderContents = function() {",
		"return renderContents().then(enhanceSettingsForm)",
		`node.querySelector('[data-name="' + optionName + '"]')`,
		"row.querySelector(':scope > .cbi-value-title')",
		`row.querySelector(':scope > .cbi-value-field input[type="checkbox"]')`,
		"var titleContent = E('span', {",
		"'data-openwrt2mqtt-event-title': 'true'",
		"display:inline-flex;align-items:center;gap:.5em;min-height:30px",
		"title.removeAttribute('for')",
		"checkbox.closest('.cbi-checkbox')",
		"checkboxLabel.remove()",
		"titleContent.appendChild(checkboxControl)",
		"titleContent.appendChild(E('button', {",
		"'data-event-type': eventType",
		"data-openwrt2mqtt-message-example-button",
		"'aria-label': _('View message example')",
		"ui.showModal(_('Message example'), [",
		"document.getElementById('modal_overlay')",
		"clickEvent.target === overlay",
		"messageExampleButton.style.zIndex = '901'",
		"s.taboption('events', form.Value, 'interface'",
		"o.retain = true",
		"o.depends('_device_event_enabled', '1')",
		"bindSecondsOption(s.taboption('events', form.Value, '_offline_timeout', _('Offline time (seconds)'))",
		"'network_device_disconnected', 'offline_timeout', 'event'",
		"o.depends('_device_disconnected_enabled', '1')",
		"return match !== null ? match[1] : value",
		"uci.set('openwrt2mqtt', sectionName, optionName, value + 's')",
		"o.default = '5'",
		"o.datatype = 'and(uinteger,min(1))'",
		"configurePositiveIntegerInput(node, '_offline_timeout')",
		"input.setAttribute('inputmode', 'numeric')",
		"input.setAttribute('pattern', '[0-9]*')",
		"input.value.replace(/[^0-9]/g, '')",
		"}, true)",
		"s.taboption('advanced', form.ListValue, 'log_level'",
		"s.taboption('advanced', form.Value, 'bus_capacity'",
		"o.inputtitle = _('Reload service')",
		"o.inputtitle = _('Test connection')",
		"method: 'status'",
		"method: 'reload'",
		"method: 'test_mqtt'",
		"o.password = true",
		"o.load = function() { return ''; }",
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

	quickIndex := strings.Index(text, "s.tab('quick', _('Quick setup'))")
	eventsIndex := strings.Index(text, "s.tab('events', _('Event settings'))")
	advancedIndex := strings.Index(text, "s.tab('advanced', _('Advanced settings'))")
	if quickIndex < 0 || eventsIndex < quickIndex || advancedIndex < eventsIndex {
		t.Fatal("settings tabs must be ordered quick, events, advanced")
	}
	connectedIndex := strings.Index(text, "bindOption(s.taboption('events', form.Flag, '_device_event_enabled'")
	interfaceIndex := strings.Index(text, "s.taboption('events', form.Value, 'interface'")
	disconnectedIndex := strings.Index(text, "bindOption(s.taboption('events', form.Flag, '_device_disconnected_enabled'")
	offlineIndex := strings.Index(text, "bindSecondsOption(s.taboption('events', form.Value, '_offline_timeout'")
	if connectedIndex < 0 || interfaceIndex < connectedIndex || disconnectedIndex < interfaceIndex || offlineIndex < disconnectedIndex {
		t.Fatal("event options must be ordered connection, interface, disconnection, offline timeout")
	}

	interfaceBlock := text[interfaceIndex:disconnectedIndex]
	if strings.Count(interfaceBlock, "o.depends('_device_event_enabled', '1')") != 1 ||
		strings.Contains(interfaceBlock, "o.depends('_device_disconnected_enabled', '1')") ||
		!strings.Contains(interfaceBlock, "o.retain = true") {
		t.Fatal("capture interface must depend only on device connection and retain its saved value while hidden")
	}

	advancedOptionIndex := strings.Index(text, "s.taboption('advanced', form.ListValue, 'log_level'")
	if advancedOptionIndex < offlineIndex {
		t.Fatal("advanced options must follow event options")
	}
	offlineBlock := text[offlineIndex:advancedOptionIndex]
	if strings.Count(offlineBlock, "o.depends('_device_disconnected_enabled', '1')") != 1 ||
		strings.Contains(offlineBlock, "o.depends('_device_event_enabled', '1')") ||
		!strings.Contains(offlineBlock, "o.retain = true") {
		t.Fatal("offline time must depend only on device disconnection and retain its saved value while hidden")
	}

	for _, forbidden := range []string{
		".cfgvalue = function",
		"uci.get('openwrt2mqtt', 'mqtt', 'password')",
		"fs.exec(",
		"fs.read(",
		"/etc/init.d/openwrt2mqtt",
		"OPENWRT2MQTT_MQTT_PASSWORD",
		"_('Configure MQTT and enable the service. Advanced options already have safe defaults.')",
		"_('Event reporting')",
		"_('Test saved connection')",
		"_('Topic prefix')",
		"_('Router ID')",
		"_('Client ID')",
		"_('QoS')",
		"_('Connection timeout')",
		"_('Leave empty to keep the saved password.')",
		"_('Device connection message example')",
		"_('MQTT topic example')",
		"_('Topic structure: topic prefix/router ID/event category/event type. Actual values depend on the device configuration.')",
		"_('Field description')",
		"_('Message schema version, currently 1.')",
		"openwrt2mqtt/OpenWrt/network/device/connected",
		"E('dl'",
		"transaction_id: '1234abcd'",
		"server_ip: '192.168.1.1'",
		"source: 'dhcp/br-lan'",
		"o.default = '5s'",
		"Offline time must be a positive duration, for example 5s.",
		"bindOption(s.taboption('events', form.Value, '_offline_timeout', _('Offline time'))",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("settings page contains forbidden string %q", forbidden)
		}
	}

	for option, want := range map[string]int{
		"form.Button, '_reload'":    1,
		"form.Button, '_test_mqtt'": 1,
	} {
		if got := strings.Count(text, option); got != want {
			t.Fatalf("settings page contains %d occurrences of %q, want %d", got, option, want)
		}
	}

	statusIndex := strings.Index(text, "_('Status')")
	reloadIndex := strings.Index(text, "form.Button, '_reload'")
	settingsIndex := strings.Index(text, "_('Settings')")
	if statusIndex < 0 || reloadIndex < statusIndex || settingsIndex < reloadIndex {
		t.Fatal("reload button must be the last control in the Status section")
	}

	for _, expected := range []string{
		"data-openwrt2mqtt-message-example",
		"overlay.addEventListener('click', messageExampleOverlayHandler)",
		"overlay.removeEventListener('click', messageExampleOverlayHandler)",
		"messageExampleButton.setAttribute('aria-expanded', 'true')",
		"messageExampleButton.setAttribute('aria-expanded', 'false')",
		"JSON.stringify(createMessageExample(messageExampleButton.dataset.eventType), null, 2)",
		"schema_version: '1'",
		"event_id: '8f69af77935d0b4a1902c203179348fa'",
		"router_id: 'OpenWrt'",
		"category: 'network'",
		"type: eventType",
		"source: 'neighbor/br-lan'",
		"connection_type: 'network'",
		"interface: 'br-lan'",
		"mac: 'aa:bb:cc:dd:ee:ff'",
		"ip: '192.168.1.100'",
		"hostname: 'example-device'",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("message example is missing %q", expected)
		}
	}

	if node, err := exec.LookPath("node"); err == nil {
		command := exec.Command(node, "--check", path)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("node --check: %v\n%s", err, output)
		}
	}
}

func TestLuCIOfflineTimeoutCreatesMissingSection(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}

	path := repoPath(t, "package", "luci-app-openwrt2mqtt", "htdocs", "luci-static", "resources", "view", "openwrt2mqtt", "settings.js")
	script := `
const fs = require('fs');
const vm = require('vm');
const source = fs.readFileSync(process.argv[1], 'utf8');
const start = source.indexOf('function ensureSection');
const end = source.indexOf('function addStatus');
if (start < 0 || end <= start)
	throw new Error('unable to locate mapped option helpers');

const sections = {};
const additions = [];
const context = {
	uci: {
		get: function(config, section, option) {
			if (arguments.length === 2)
				return sections[section] || null;
			return sections[section] ? sections[section][option] : null;
		},
		add: function(config, type, section) {
			additions.push([config, type, section]);
			sections[section] = { '.type': type };
			return section;
		},
		set: function(config, section, option, value) {
			if (sections[section])
				sections[section][option] = value;
		},
		unset: function(config, section, option) {
			if (sections[section])
				delete sections[section][option];
		}
	}
};
vm.runInNewContext(source.slice(start, end) + '\nthis.bindSecondsOption = bindSecondsOption;', context);

const option = {};
context.bindSecondsOption(option, 'network_device_disconnected', 'offline_timeout', 'event');
option.write('main', '17');
if (additions.length !== 1 || additions[0].join('/') !== 'openwrt2mqtt/event/network_device_disconnected')
	throw new Error('missing named event section was not created correctly: ' + JSON.stringify(additions));
if (sections.network_device_disconnected.offline_timeout !== '17s')
	throw new Error('offline timeout was not written after creating the section');

option.write('main', '30');
if (additions.length !== 1)
	throw new Error('existing section was created more than once');
if (sections.network_device_disconnected.offline_timeout !== '30s')
	throw new Error('existing offline timeout was not updated');
`
	command := exec.Command(node, "-e", script, path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("offline timeout helper: %v\n%s", err, output)
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
		"msgid \"OpenWrt to MQTT\"",
		"msgstr \"OpenWrt to MQTT\"",
		"msgid \"Publish OpenWrt system events to MQTT.\"",
		"msgstr \"将OpenWrt系统事件发布到MQTT\"",
		"msgid \"Quick setup\"",
		"msgstr \"常用设置\"",
		"msgid \"Event settings\"",
		"msgstr \"事件设置\"",
		"msgid \"Advanced settings\"",
		"msgstr \"高级设置\"",
		"msgid \"Test connection\"",
		"msgstr \"测试连接\"",
		"msgid \"Device connection\"",
		"msgstr \"设备接入\"",
		"msgid \"Offline time (seconds)\"",
		"msgstr \"离线时间（秒）\"",
		"msgid \"Offline time must be a positive integer number of seconds.\"",
		"msgstr \"离线时间必须为大于零的整数秒。\"",
		"msgid \"View message example\"",
		"msgstr \"查看消息示例\"",
		"msgid \"Message example\"",
		"msgstr \"消息示例\"",
		"msgid \"MQTT server\"",
		"msgstr \"MQTT 服务器\"",
		"msgid \"Enter a host and port. tcp:// is added automatically when no protocol is specified.\"",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Simplified Chinese translation is missing %q", expected)
		}
	}

	for _, forbidden := range []string{
		"Configure MQTT and enable the service. Advanced options already have safe defaults.",
		"配置 MQTT 服务器并启用服务。高级选项已提供安全的默认值。",
		"Leave empty to keep the saved password.",
		"Device connection message example",
		"MQTT topic example",
		"Field description",
		"Message schema version, currently 1.",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Simplified Chinese translation contains obsolete string %q", forbidden)
		}
	}
}

func TestBuildWorkflowIncludesSimplifiedChinesePackage(t *testing.T) {
	path := repoPath(t, ".github", "workflows", "build-openwrt.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		"PACKAGE_luci-i18n-openwrt2mqtt-zh-cn",
		"luci-i18n-openwrt2mqtt-zh-cn-*.apk",
		"-eq 3",
		"cd artifacts",
		"xargs -0 sha256sum",
		"type: boolean",
		"inputs.release",
		"Release 只能从 main 分支发布。",
		"permissions:\n      contents: write",
		"uses: actions/download-artifact@v4",
		"uses: softprops/action-gh-release@v2",
		"tag_name: v${{ needs.test.outputs.package_version }}",
		"generate_release_notes: true",
		"release-assets/SHA256SUMS",
		"Release 标签 ${tag} 已存在，请先更新 PKG_VERSION。",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("build workflow is missing %q", expected)
		}
	}
}

func TestOpenWrtPackageMakefiles(t *testing.T) {
	checks := map[string][]string{
		repoPath(t, "package", "openwrt2mqtt", "Makefile"): {
			"PKG_VERSION:=1.0.0",
			"PKG_RELEASE:=2",
			"GO_PKG_BUILD_PKG:=$(GO_PKG)/cmd/openwrt2mqtt",
			"GO_PKG_LDFLAGS_X:=main.version=$(PKG_VERSION)",
			"DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle +procd +rpcd +uci",
			"/etc/config/openwrt2mqtt",
			"$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/openwrt2mqtt $(1)/usr/sbin/openwrt2mqtt",
			"$(INSTALL_BIN) ./files/usr/libexec/openwrt2mqtt/migrate-config $(1)/usr/libexec/openwrt2mqtt/migrate-config",
			"/usr/libexec/openwrt2mqtt/migrate-config || exit 1",
		},
		repoPath(t, "package", "luci-app-openwrt2mqtt", "Makefile"): {
			"PKG_VERSION:=1.0.0",
			"PKG_RELEASE:=2",
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
