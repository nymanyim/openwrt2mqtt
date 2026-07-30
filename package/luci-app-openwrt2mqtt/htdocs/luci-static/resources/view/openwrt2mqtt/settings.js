'use strict';
'require view';
'require form';
'require rpc';
'require uci';
'require ui';

var messageExamplePanel = null;
var messageExampleButton = null;
var messageExampleOutsideHandler = null;
var messageExampleListenerTimer = null;

var callStatus = rpc.declare({
	object: 'openwrt2mqtt',
	method: 'status',
	expect: { '': {} }
});

var callReload = rpc.declare({
	object: 'openwrt2mqtt',
	method: 'reload',
	expect: { '': {} }
});

var callTestMQTT = rpc.declare({
	object: 'openwrt2mqtt',
	method: 'test_mqtt',
	expect: { '': {} }
});

function bindOption(option, sectionName, optionName) {
	option.cfgvalue = function() {
		return uci.get('openwrt2mqtt', sectionName, optionName);
	};
	option.write = function(sectionId, value) {
		uci.set('openwrt2mqtt', sectionName, optionName, value);
	};
	option.remove = function() {
		uci.unset('openwrt2mqtt', sectionName, optionName);
	};
	return option;
}

function addStatus(section, optionName, title, value) {
	var option = section.option(form.DummyValue, optionName, title);
	option.cfgvalue = function() { return value; };
}

function closeMessageExample() {
	if (messageExampleListenerTimer !== null) {
		window.clearTimeout(messageExampleListenerTimer);
		messageExampleListenerTimer = null;
	}
	if (messageExampleOutsideHandler !== null) {
		document.removeEventListener('click', messageExampleOutsideHandler);
		messageExampleOutsideHandler = null;
	}
	if (messageExamplePanel !== null)
		messageExamplePanel.remove();
	messageExamplePanel = null;
	messageExampleButton = null;
}

function createMessageExample() {
	var message = {
		schema_version: '1',
		event_id: '8f69af77935d0b4a1902c203179348fa',
		router_id: 'OpenWrt',
		category: 'network',
		type: 'device.connected',
		source: 'dhcp/br-lan',
		timestamp: '2026-07-30T05:18:21Z',
		data: {
			mac: 'AA:BB:CC:DD:EE:FF',
			transaction_id: '1234abcd',
			ip: '192.168.1.100',
			hostname: 'example-device',
			server_ip: '192.168.1.1'
		}
	};

	return E('div', {
		'class': 'cbi-section',
		'data-openwrt2mqtt-message-example': 'true'
	}, [
		E('h4', _('Device connection message example')),
		E('h5', _('MQTT topic example')),
		E('pre', 'openwrt2mqtt/OpenWrt/network/device/connected'),
		E('p', _('Topic structure: topic prefix/router ID/event category/event type. Actual values depend on the device configuration.')),
		E('h5', _('Message example')),
		E('pre', JSON.stringify(message, null, 2)),
		E('h5', _('Field description')),
		E('dl', {}, [
			E('dt', E('code', 'schema_version')),
			E('dd', _('Message schema version, currently 1.')),
			E('dt', E('code', 'event_id')),
			E('dd', _('Unique event identifier.')),
			E('dt', E('code', 'router_id')),
			E('dd', _('Router identifier that produced the event.')),
			E('dt', E('code', 'category')),
			E('dd', _('Event category. Device connection events use network.')),
			E('dt', E('code', 'type')),
			E('dd', _('Event type. Device connection events use device.connected.')),
			E('dt', E('code', 'source')),
			E('dd', _('Event source in the form dhcp/capture interface.')),
			E('dt', E('code', 'timestamp')),
			E('dd', _('UTC time when the event was generated.')),
			E('dt', E('code', 'data')),
			E('dd', _('Event-specific data.')),
			E('dt', E('code', 'mac')),
			E('dd', _('Device MAC address.')),
			E('dt', E('code', 'transaction_id')),
			E('dd', _('DHCP transaction identifier.')),
			E('dt', E('code', 'ip')),
			E('dd', _('IP address assigned to the device.')),
			E('dt', E('code', 'hostname')),
			E('dd', _('Hostname supplied by the device through DHCP.')),
			E('dt', E('code', 'server_ip')),
			E('dd', _('DHCP server address.'))
		]),
		E('p', _('mac and transaction_id are always present. ip, hostname, and server_ip are optional and may be absent from actual messages.'))
	]);
}

function toggleMessageExample(event) {
	var button = event.currentTarget;
	if (messageExamplePanel !== null) {
		closeMessageExample();
		return;
	}

	var row = button.closest ? button.closest('.cbi-value') : button.parentNode;
	if (row === null || row.parentNode === null)
		return;

	messageExampleButton = button;
	messageExamplePanel = createMessageExample();
	row.parentNode.insertBefore(messageExamplePanel, row.nextSibling);

	messageExampleOutsideHandler = function(clickEvent) {
		if (messageExamplePanel === null)
			return;
		if (messageExamplePanel.contains(clickEvent.target) || messageExampleButton.contains(clickEvent.target))
			return;
		closeMessageExample();
	};
	messageExampleListenerTimer = window.setTimeout(function() {
		messageExampleListenerTimer = null;
		document.addEventListener('click', messageExampleOutsideHandler);
	}, 0);
}

return view.extend({
	load: function() {
		return Promise.all([
			uci.load('openwrt2mqtt'),
			callStatus()
		]);
	},

	render: function(data) {
		var m, s, o, deviceConnectionEnabled;
		var status = data[1] || {};
		closeMessageExample();

		m = new form.Map('openwrt2mqtt', _('OpenWrt to MQTT'),
			_('Configure MQTT and enable the service. Advanced options already have safe defaults.'));

		s = m.section(form.NamedSection, 'main', 'openwrt2mqtt', _('Status'));
		s.anonymous = true;
		s.addremove = false;

		addStatus(s, '_service_status', _('Service'),
			status.running ? _('Running') : (status.enabled ? _('Not running') : _('Disabled')));
		addStatus(s, '_mqtt_status', _('MQTT configuration'),
			status.configured ? _('Configured') : _('Not configured'));
		addStatus(s, '_version', _('Version'), status.version || _('Unknown'));

		o = s.option(form.Button, '_reload', '');
		o.inputtitle = _('Reload service');
		o.inputstyle = 'apply';
		o.onclick = function() {
			return callReload().then(function(result) {
				if (!result.success)
					throw new Error(_('Service reload failed.'));
				ui.addNotification(null, E('p', _('Service reloaded successfully.')), 'info');
				window.location.reload();
			}).catch(function(error) {
				ui.addNotification(null, E('p', error.message || _('Service reload failed.')), 'error');
			});
		};

		s = m.section(form.NamedSection, 'main', 'openwrt2mqtt', _('Settings'));
		s.anonymous = true;
		s.addremove = false;
		s.tab('quick', _('Quick setup'));
		s.tab('events', _('Event settings'));
		s.tab('advanced', _('Advanced settings'));

		o = s.taboption('quick', form.Flag, 'enabled', _('Enable service'));
		o.default = o.disabled;
		o.rmempty = false;

		o = bindOption(s.taboption('quick', form.Value, '_broker', _('MQTT server'),
			_('Enter a host and port. tcp:// is added automatically when no protocol is specified.')),
			'mqtt', 'broker');
		o.default = '127.0.0.1:1883';
		o.placeholder = '127.0.0.1:1883';
		o.cfgvalue = function() {
			return uci.get('openwrt2mqtt', 'mqtt', 'broker') || '127.0.0.1:1883';
		};
		o.rmempty = false;

		o = bindOption(s.taboption('quick', form.Value, '_username', _('Username')), 'mqtt', 'username');
		o.optional = true;

		o = s.taboption('quick', form.Value, '_password', _('Password'),
			_('Leave empty to keep the saved password.'));
		o.password = true;
		o.optional = true;
		o.cfgvalue = function() { return ''; };
		o.write = function(sectionId, value) {
			if (value)
				uci.set('openwrt2mqtt', 'mqtt', 'password', value);
		};
		o.remove = function() {};

		o = s.taboption('quick', form.Button, '_test_mqtt', '');
		o.inputtitle = _('Test connection');
		o.inputstyle = 'action';
		o.onclick = function() {
			return callTestMQTT().then(function(result) {
				if (result.success) {
					ui.addNotification(null,
						E('p', _('MQTT connection succeeded in %d ms.').format(result.latency_ms || 0)), 'info');
					return;
				}
				var messages = {
					configuration_invalid: _('The saved MQTT configuration is invalid.'),
					timeout: _('The MQTT connection timed out.'),
					connection_failed: _('The MQTT connection failed.')
				};
				ui.addNotification(null, E('p', messages[result.error] || _('The MQTT connection failed.')), 'error');
			}).catch(function(error) {
				ui.addNotification(null, E('p', error.message || _('The MQTT connection test failed.')), 'error');
			});
		};

		deviceConnectionEnabled = bindOption(s.taboption('events', form.Flag, '_device_event_enabled',
			_('Device connection'),
			_('Publish an event after a device obtains a DHCP address. Normal renewals are ignored.')),
			'network_device_connected', 'enabled');
		deviceConnectionEnabled.default = deviceConnectionEnabled.enabled;
		deviceConnectionEnabled.rmempty = false;

		o = s.taboption('events', form.Button, '_device_message_example', '');
		o.inputtitle = _('View message example');
		o.inputstyle = 'action';
		o.onclick = toggleMessageExample;

		o = s.taboption('events', form.Value, 'interface', _('Capture interface'));
		o.default = 'br-lan';
		o.rmempty = false;
		o.depends('_device_event_enabled', '1');
		o.validate = function(sectionId, value) {
			return value ? true : _('The capture interface must not be empty.');
		};

		o = s.taboption('advanced', form.ListValue, 'log_level', _('Log level'));
		o.value('debug', _('Debug'));
		o.value('info', _('Info'));
		o.value('warn', _('Warning'));
		o.value('error', _('Error'));
		o.default = 'info';
		o.rmempty = false;

		o = s.taboption('advanced', form.Value, 'bus_capacity', _('Event queue capacity'));
		o.datatype = 'min(1)';
		o.default = '128';
		o.rmempty = false;

		return m.render();
	}
});
