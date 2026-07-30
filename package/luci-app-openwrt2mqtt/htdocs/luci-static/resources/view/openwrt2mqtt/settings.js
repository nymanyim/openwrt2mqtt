'use strict';
'require view';
'require form';
'require rpc';
'require uci';
'require ui';

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

function validateDuration(sectionId, value) {
	if (!/^([0-9]+([.][0-9]+)?(ns|us|µs|μs|ms|s|m|h))+$/.test(value) || !/[1-9]/.test(value))
		return _('Enter a positive duration such as 500ms, 10s, 1m, or 1h30m.');
	return true;
}

function addStatus(section, optionName, title, value) {
	var option = section.option(form.DummyValue, optionName, title);
	option.cfgvalue = function() { return value; };
}

return view.extend({
	load: function() {
		return Promise.all([
			uci.load('openwrt2mqtt'),
			callStatus()
		]);
	},

	render: function(data) {
		var m, s, o, deviceEventEnabled;
		var status = data[1] || {};

		m = new form.Map('openwrt2mqtt', _('OpenWrt to MQTT'),
			_('Configure MQTT and enable the service. Advanced options already have safe defaults.'));

		s = m.section(form.NamedSection, 'main', 'openwrt2mqtt', _('Status'));
		s.anonymous = true;
		s.addremove = false;

		addStatus(s, '_service_status', _('Service'),
			status.running ? _('Running') : (status.enabled ? _('Not running') : _('Disabled')));
		addStatus(s, '_mqtt_status', _('MQTT configuration'),
			status.configured ? _('Configured') : _('Not configured'));
		addStatus(s, '_event_status', _('Event reporting'),
			status.device_connected_enabled ? _('Enabled') : _('Disabled'));
		addStatus(s, '_version', _('Version'), status.version || _('Unknown'));

		o = s.option(form.Button, '_reload', _('Reload service'),
			_('Validate the saved configuration and restart the service.'));
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

		o = bindOption(s.taboption('quick', form.Value, '_topic', _('Topic prefix')), 'mqtt', 'topic');
		o.default = 'openwrt2mqtt';
		o.rmempty = false;

		o = s.taboption('quick', form.Button, '_test_mqtt', _('Test saved connection'),
			_('Save and apply your changes before testing. No event will be published.'));
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

		o = s.taboption('advanced', form.Value, 'router_id', _('Router ID'),
			_('Leave empty to use the system hostname.'));
		o.optional = true;

		o = s.taboption('advanced', form.Value, 'interface', _('Capture interface'));
		o.default = 'br-lan';
		o.rmempty = false;
		o.validate = function(sectionId, value) {
			return value ? true : _('The capture interface must not be empty.');
		};

		o = bindOption(s.taboption('advanced', form.Value, '_client_id', _('Client ID'),
			_('Leave empty to use the Router ID.')), 'mqtt', 'client_id');
		o.optional = true;

		deviceEventEnabled = bindOption(s.taboption('advanced', form.Flag, '_device_event_enabled',
			_('Device connected event'),
			_('Publish an event after a device obtains a DHCP address. Normal renewals are ignored.')),
			'network_device_connected', 'enabled');
		deviceEventEnabled.default = deviceEventEnabled.enabled;
		deviceEventEnabled.rmempty = false;

		o = bindOption(s.taboption('advanced', form.ListValue, '_qos', _('QoS')), 'mqtt', 'qos');
		o.value('0', '0');
		o.value('1', '1');
		o.value('2', '2');
		o.default = '0';
		o.rmempty = false;

		o = bindOption(s.taboption('advanced', form.Value, '_timeout', _('Connection timeout')), 'mqtt', 'timeout');
		o.default = '10s';
		o.rmempty = false;
		o.validate = validateDuration;

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
