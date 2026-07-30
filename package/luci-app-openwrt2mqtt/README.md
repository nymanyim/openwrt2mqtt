# LuCI integration

This directory contains the stage-4 LuCI integration:

- service, event, and MQTT configuration form;
- read-only runtime status;
- controlled service reload;
- one-shot MQTT connection test;
- minimal UCI and ubus ACL permissions.

The password field never reads the saved password into the browser. Leaving it empty preserves the current UCI value.

Stage 5 adds the LuCI package definition for version `1.0.0`, depending on `luci-base` and the matching `openwrt2mqtt` core package.