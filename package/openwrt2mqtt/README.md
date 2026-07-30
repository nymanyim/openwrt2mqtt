# Core OpenWrt integration

This directory contains the OpenWrt integration files implemented through stage 4:

- `files/etc/config/openwrt2mqtt`: disabled-by-default UCI configuration and event switch;
- `files/etc/init.d/openwrt2mqtt`: procd service, validation, UCI-to-environment adapter, and MQTT test command;
- `files/usr/libexec/rpcd/openwrt2mqtt`: `status`, `reload`, and `test_mqtt` ubus methods.

Stage 5 adds the OpenWrt package definition for version `1.0.0`, including Go cross-compilation, version injection, runtime dependencies, conffile preservation, and APK generation.