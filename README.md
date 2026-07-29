# openwrt2mqtt

`openwrt2mqtt` converts OpenWrt system events into transport-neutral events and publishes them to MQTT.

## Status

The project is in its initial architecture stage. Event collectors, MQTT transport, OpenWrt packaging, and LuCI integration are not implemented yet.

## Design

```text
Collector -> Event Bus -> Processor -> Publisher
```

- Collectors observe OpenWrt event sources without depending on MQTT.
- Processors validate, enrich, filter, or deduplicate normalized events.
- Publishers deliver normalized events without depending on collectors.
- UCI and ubus adapters will provide native OpenWrt configuration and control.

## Planned first event

```text
network.device.connected
```

It will represent a device completing a DHCP address-acquisition exchange. Normal DHCP lease renewal will not emit this event.

## License

MIT
