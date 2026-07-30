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

function statusText(status) {
	return [
		_('Enabled: %s').format(status.enabled ? _('yes') : _('no')),
		_('Running: %s').format(status.running ? _('yes') : _('no')),
		_('Configured: %s').format(status.configured ? _('yes') : _('no')),
		_('Device connected event: %s').format(status.device_connected_enabled ? _('enabled') : _('disabled')),
		_('Version: %s').format(status.version || _('unknown'))
	].join(' · ');
}

function validateDuration(sectionId, value) {
	if (!/^([0-9]+([.][0-9]+)?(ns|us|µs|μs|ms|s|m|h))+$/.test(value) || !/[1-9]/.test(value))
		return _('Enter a positive Go duration such as 500ms, 10s, 1m, or 1h30m.');
	return true;
}

return view.extend({
	load: function() {
		return Promise.all([
			uci.load('openwrt2mqtt'),
			callStatus()
		]);
	},

	render: function(data) {
		var m, s, o, serviceEnabled, deviceEventEnabled;
		var status = data[1] || {};

		m = new form.Map('openwrt2mqtt', _('OpenWrt to MQTT'),
			_('Publish normalized OpenWrt events to an MQTT broker.'));

		s = m.section(form.NamedSection, 'main', 'openwrt2mqtt', _('Service status'));
		s.anonymous = true;
		s.addremove = false;

		o = s.option(form.DummyValue, '_status', _('Current status'));
		o.cfgvalue = function() { return statusText(status); };

		o = s.option(form.Button, '_reload', _('Reload service'),
			_('Validate the saved configuration and restart the managed service.'));
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

		o = s.option(form.Button, '_test_mqtt', _('Test MQTT connection'),
			_('Tests the saved configuration without publishing a business event. Save changes before testing.'));
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

		s = m.section(form.NamedSection, 'main', 'openwrt2mqtt', _('Basic settings'));
		s.anonymous = true;
		s.addremove = false;

		serviceEnabled = s.option(form.Flag, 'enabled', _('Enable service'));
		serviceEnabled.default = serviceEnabled.disabled;
		serviceEnabled.rmempty = false;

		o = s.option(form.Value, 'router_id', _('Router ID'),
			_('Leave empty to use the system hostname.'));
		o.optional = true;

		o = s.option(form.Value, 'interface', _('Capture interface'));
		o.default = 'br-lan';
		o.rmempty = false;
		o.validate = function(sectionId, value) {
			return value ? true : _('The capture interface must not be empty.');
		};

		o = s.option(form.ListValue, 'log_level', _('Log level'));
		o.value('debug', _('Debug'));
		o.value('info', _('Info'));
		o.value('warn', _('Warning'));
		o.value('error', _('Error'));
		o.default = 'info';
		o.rmempty = false;

		o = s.option(form.Value, 'bus_capacity', _('Event bus capacity'));
		o.datatype = 'min(1)';
		o.default = '128';
		o.rmempty = false;

		s = m.section(form.NamedSection, 'network_device_connected', 'event', _('Events'));
		s.anonymous = true;
		s.addremove = false;

		deviceEventEnabled = s.option(form.Flag, 'enabled', _('Device connected'),
			_('Emit network.device.connected after a DHCP address-acquisition exchange. Normal renewals are excluded.'));
		deviceEventEnabled.default = deviceEventEnabled.enabled;
		deviceEventEnabled.rmempty = false;

		s = m.section(form.NamedSection, 'mqtt', 'mqtt', _('MQTT settings'));
		s.anonymous = true;
		s.addremove = false;

		o = s.option(form.Value, 'broker', _('Broker URI'));
		o.placeholder = 'tcp://192.0.2.1:1883';
		o.optional = true;
		o.validate = function(sectionId, value) {
			if (serviceEnabled.formvalue('main') === '1' &&
				deviceEventEnabled.formvalue('network_device_connected') === '1' && !value)
				return _('The MQTT broker is required while the service and device connected event are enabled.');
			return true;
		};

		o = s.option(form.Value, 'client_id', _('Client ID'),
			_('Leave empty to use the Router ID.'));
		o.optional = true;

		o = s.option(form.Value, 'username', _('Username'));
		o.optional = true;

		o = s.option(form.Value, 'password', _('Password'),
			_('Leave empty to keep the saved password.'));
		o.password = true;
		o.optional = true;
		o.cfgvalue = function() { return ''; };
		o.write = function(sectionId, value) {
			if (value)
				uci.set('openwrt2mqtt', sectionId, 'password', value);
		};
		o.remove = function() {};

		o = s.option(form.Value, 'topic', _('Topic prefix'));
		o.default = 'openwrt2mqtt';
		o.rmempty = false;

		o = s.option(form.ListValue, 'qos', _('QoS'));
		o.value('0', '0');
		o.value('1', '1');
		o.value('2', '2');
		o.default = '0';
		o.rmempty = false;

		o = s.option(form.Value, 'timeout', _('Timeout'));
		o.default = '10s';
		o.rmempty = false;
		o.validate = validateDuration;

		return m.render();
	}
});