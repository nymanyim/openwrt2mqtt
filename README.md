# openwrt2mqtt

`openwrt2mqtt` converts OpenWrt system events into transport-neutral events and publishes them to MQTT.

## Status

Stage 2 is implemented and verified on a real OpenWrt device:

- DHCP traffic collection through Linux `AF_PACKET` with a classic BPF filter;
- normalized `network.device.connected` events;
- bounded in-memory event bus and processing pipeline;
- MQTT publishing with environment-driven runtime configuration;
- suppression of normal DHCP lease-renewal events;
- Linux `arm64` and `amd64` builds.

Stage 3 adds disabled-by-default UCI configuration and procd lifecycle management. Stage 4 adds event control, status/reload/MQTT-test ubus methods, a LuCI configuration view, and minimal ACL permissions. Stage 5 adds version `1.0.0` OpenWrt and LuCI package definitions plus a reproducible OpenWrt 25.12.5 APK build workflow for `rockchip/armv8`.

## Design

```text
Collector -> Event Bus -> Processor -> Publisher
```

- Collectors observe OpenWrt event sources without depending on MQTT.
- Processors validate, enrich, filter, or deduplicate normalized events.
- Publishers deliver normalized events without depending on collectors.
- UCI and ubus adapters will provide native OpenWrt configuration and control.

## Implemented event

```text
network.device.connected
```

It represents a device completing a DHCP address-acquisition exchange. Normal DHCP lease renewal does not emit this event.

## License

MIT