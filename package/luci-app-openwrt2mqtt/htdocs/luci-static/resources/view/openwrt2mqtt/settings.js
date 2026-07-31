'use strict';
'require view';
'require form';
'require rpc';
'require uci';
'require ui';

var messageExampleOpen = false;
var messageExampleButton = null;
var messageExampleOverlayHandler = null;

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

function ensureSection(sectionName, sectionType) {
	if (uci.get('openwrt2mqtt', sectionName) == null)
		uci.add('openwrt2mqtt', sectionType, sectionName);
}

function bindOption(option, sectionName, optionName, sectionType) {
	option.load = function() {
		return uci.get('openwrt2mqtt', sectionName, optionName);
	};
	option.write = function(sectionId, value) {
		ensureSection(sectionName, sectionType);
		uci.set('openwrt2mqtt', sectionName, optionName, value);
	};
	option.remove = function() {
		uci.unset('openwrt2mqtt', sectionName, optionName);
	};
	return option;
}

function bindSecondsOption(option, sectionName, optionName, sectionType) {
	option.load = function() {
		var value = uci.get('openwrt2mqtt', sectionName, optionName);
		var match = typeof value === 'string' ? /^([1-9][0-9]*)s$/.exec(value) : null;
		return match !== null ? match[1] : value;
	};
	option.write = function(sectionId, value) {
		ensureSection(sectionName, sectionType);
		uci.set('openwrt2mqtt', sectionName, optionName, value + 's');
	};
	option.remove = function() {
		uci.unset('openwrt2mqtt', sectionName, optionName);
	};
	return option;
}

function addStatus(section, optionName, title, value) {
	var option = section.option(form.DummyValue, optionName, title);
	option.load = function() { return value; };
}

function createMessageExample(eventType) {
	return {
		schema_version: '1',
		event_id: '8f69af77935d0b4a1902c203179348fa',
		router_id: 'OpenWrt',
		category: 'network',
		type: eventType,
		source: 'neighbor/br-lan',
		timestamp: '2026-07-30T05:18:21Z',
		data: {
			connection_type: 'network',
			interface: 'br-lan',
			ip: '192.168.1.100',
			mac: 'aa:bb:cc:dd:ee:ff',
			hostname: 'example-device'
		}
	};
}

function closeMessageExample() {
	var overlay = document.getElementById('modal_overlay');
	if (overlay !== null && messageExampleOverlayHandler !== null)
		overlay.removeEventListener('click', messageExampleOverlayHandler);
	if (messageExampleButton !== null) {
		messageExampleButton.setAttribute('aria-expanded', 'false');
		messageExampleButton.style.position = '';
		messageExampleButton.style.zIndex = '';
	}

	messageExampleOverlayHandler = null;
	messageExampleButton = null;
	if (messageExampleOpen)
		ui.hideModal();
	messageExampleOpen = false;
}

function toggleMessageExample(event) {
	event.preventDefault();
	event.stopPropagation();

	if (messageExampleOpen) {
		closeMessageExample();
		return;
	}

	messageExampleButton = event.currentTarget;
	messageExampleButton.setAttribute('aria-expanded', 'true');
	messageExampleButton.style.position = 'relative';
	messageExampleButton.style.zIndex = '901';
	ui.showModal(_('Message example'), [
		E('pre', {
			'data-openwrt2mqtt-message-example': 'true'
		}, JSON.stringify(createMessageExample(messageExampleButton.dataset.eventType), null, 2))
	]);
	messageExampleOpen = true;

	var overlay = document.getElementById('modal_overlay');
	messageExampleOverlayHandler = function(clickEvent) {
		if (clickEvent.target === overlay)
			closeMessageExample();
	};
	if (overlay !== null)
		overlay.addEventListener('click', messageExampleOverlayHandler);
}

function attachMessageExampleButton(node, optionName, eventType) {
	var row = node.querySelector('[data-name="' + optionName + '"]');
	var title = row !== null ? row.querySelector(':scope > .cbi-value-title') : null;
	var checkbox = row !== null ? row.querySelector(':scope > .cbi-value-field input[type="checkbox"]') : null;
	if (title === null || row.querySelector('[data-openwrt2mqtt-message-example-button="true"]') !== null)
		return;

	var titleText = title.textContent.trim();
	var titleContent = E('span', {
		'data-openwrt2mqtt-event-title': 'true',
		'style': 'display:inline-flex;align-items:center;gap:.5em;min-height:30px;vertical-align:middle'
	});
	while (title.firstChild !== null)
		titleContent.appendChild(title.firstChild);

	title.removeAttribute('for');
	title.style.paddingTop = '0';
	title.appendChild(titleContent);

	if (checkbox !== null) {
		var checkboxControl = checkbox.closest('.cbi-checkbox');
		var checkboxLabel = checkboxControl !== null ? checkboxControl.querySelector(':scope > label[for]') : null;
		if (checkboxControl !== null) {
			if (checkboxLabel !== null)
				checkboxLabel.remove();
			checkbox.setAttribute('aria-label', titleText);
			checkboxControl.style.display = 'inline-flex';
			checkboxControl.style.alignItems = 'center';
			checkboxControl.style.flex = '0 0 auto';
			checkboxControl.style.height = 'auto';
			checkboxControl.style.margin = '0';
			checkbox.style.margin = '0';
			titleContent.appendChild(checkboxControl);
		}
	}

	titleContent.appendChild(E('button', {
		'type': 'button',
		'class': 'cbi-button',
		'title': _('View message example'),
		'aria-label': _('View message example'),
		'aria-expanded': 'false',
		'data-event-type': eventType,
		'data-openwrt2mqtt-message-example-button': 'true',
		'style': 'margin:0;padding:0 .35em;min-width:auto;line-height:1.3;display:inline-flex;align-items:center;justify-content:center;flex:0 0 auto',
		'click': toggleMessageExample
	}, 'ⓘ'));
}

function configurePositiveIntegerInput(node, optionName) {
	var row = node.querySelector('[data-name="' + optionName + '"]');
	var input = row !== null ? row.querySelector(':scope > .cbi-value-field input[type="text"]') : null;
	if (input === null)
		return;

	input.setAttribute('inputmode', 'numeric');
	input.setAttribute('pattern', '[0-9]*');
	if (input.dataset.openwrt2mqttPositiveInteger === 'true')
		return;

	input.dataset.openwrt2mqttPositiveInteger = 'true';
	input.addEventListener('input', function() {
		var digits = input.value.replace(/[^0-9]/g, '');
		if (digits !== input.value)
			input.value = digits;
	}, true);
}

function enhanceSettingsForm(node) {
	attachMessageExampleButton(node, '_device_event_enabled', 'device.connected');
	attachMessageExampleButton(node, '_device_disconnected_enabled', 'device.disconnected');
	configurePositiveIntegerInput(node, '_offline_timeout');
	return node;
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
			_('Publish OpenWrt system events to MQTT.'));
		var renderContents = m.renderContents.bind(m);
		m.renderContents = function() {
			return renderContents().then(enhanceSettingsForm);
		};

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
			'mqtt', 'broker', 'mqtt');
		o.default = '127.0.0.1:1883';
		o.placeholder = '127.0.0.1:1883';
		o.load = function() {
			return uci.get('openwrt2mqtt', 'mqtt', 'broker') || '127.0.0.1:1883';
		};
		o.rmempty = false;

		o = bindOption(s.taboption('quick', form.Value, '_username', _('Username')), 'mqtt', 'username', 'mqtt');
		o.optional = true;

		o = s.taboption('quick', form.Value, '_password', _('Password'));
		o.password = true;
		o.optional = true;
		o.load = function() { return ''; };
		o.write = function(sectionId, value) {
			if (value) {
				ensureSection('mqtt', 'mqtt');
				uci.set('openwrt2mqtt', 'mqtt', 'password', value);
			}
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
			_('Device connection'), _('Publish an event when a device is first observed or reconnects.')),
			'network_device_connected', 'enabled', 'event');
		deviceConnectionEnabled.default = deviceConnectionEnabled.enabled;
		deviceConnectionEnabled.rmempty = false;

		o = s.taboption('events', form.Value, 'interface', _('Capture interface'));
		o.default = 'br-lan';
		o.rmempty = false;
		o.retain = true;
		o.depends('_device_event_enabled', '1');
		o.validate = function(sectionId, value) {
			return value ? true : _('The capture interface must not be empty.');
		};

		o = bindOption(s.taboption('events', form.Flag, '_device_disconnected_enabled',
			_('Device disconnection'), _('Publish an event after a device remains unreachable for the configured offline time.')),
			'network_device_disconnected', 'enabled', 'event');
		o.default = o.enabled;
		o.rmempty = false;

		o = bindSecondsOption(s.taboption('events', form.Value, '_offline_timeout', _('Offline time (seconds)')),
			'network_device_disconnected', 'offline_timeout', 'event');
		o.default = '5';
		o.datatype = 'and(uinteger,min(3))';
		o.rmempty = false;
		o.retain = true;
		o.depends('_device_disconnected_enabled', '1');
		o.validate = function(sectionId, value) {
			return /^[1-9][0-9]*$/.test(value) && Number(value) >= 3
				? true : _('Offline time must be an integer of at least 3 seconds.');
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
