# openwrt2mqtt

`openwrt2mqtt` 是运行于 OpenWrt 的事件桥接服务，用于采集系统事件、统一消息格式，并通过 MQTT 提供给监控、自动化及其他订阅端。

当前版本支持设备接入与离线事件，可实时识别 Wi-Fi 关联变化，并通过 DHCP 与 ARP 邻居状态覆盖动态及静态 IPv4 设备。

## 功能

- 监听 hostapd，实时识别 Wi-Fi 设备接入与离线；
- 监听 ARP 邻居状态，识别有线及静态 IPv4 设备；
- 监听 DHCP 首次租约并补充 IP、主机名等信息；
- 按 MAC 维护在线状态，过滤续租及多事件源产生的重复消息；
- 将事件标准化为 JSON 并发布到 MQTT；
- 支持 MQTT 认证、QoS、连接超时和 Topic 前缀；
- 使用 UCI 管理配置，使用 procd 管理服务；
- 提供 LuCI 管理页面、运行状态和连接测试；
- 提供简体中文界面。

## 安装

构建产物包含三个 APK：

- `openwrt2mqtt-*.apk`：后台服务；
- `luci-app-openwrt2mqtt-*.apk`：LuCI 管理页面；
- `luci-i18n-openwrt2mqtt-zh-cn-*.apk`：简体中文翻译。

上传到路由器后执行：

```sh
apk add --allow-untrusted \
  ./openwrt2mqtt-*.apk \
  ./luci-app-openwrt2mqtt-*.apk \
  ./luci-i18n-openwrt2mqtt-zh-cn-*.apk
```

安装完成后进入 LuCI：

```text
服务 -> OpenWrt to MQTT
```

## 配置

常用默认值：

| 配置项 | 默认值 |
| --- | --- |
| MQTT 服务器 | `127.0.0.1:1883` |
| 设备监听接口 | `br-lan` |

MQTT 用户名和密码默认留空。

UCI 配置文件：

```text
/etc/config/openwrt2mqtt
```

修改配置后执行：

```sh
/etc/init.d/openwrt2mqtt reload
```

## 服务管理

```sh
/etc/init.d/openwrt2mqtt start       # 启动
/etc/init.d/openwrt2mqtt stop        # 停止
/etc/init.d/openwrt2mqtt restart     # 重启
/etc/init.d/openwrt2mqtt mqtt_test   # 测试 MQTT 连接
logread -e openwrt2mqtt              # 查看日志
```

## MQTT 主题

项目当前支持以下 MQTT 主题：

| 事件 | 消息类型 | MQTT 主题 |
| --- | --- | --- |
| 设备接入 | `device.connected` | `openwrt2mqtt/OpenWrt/network/device/connected` |
| 设备断开 | `device.disconnected` | `openwrt2mqtt/OpenWrt/network/device/disconnected` |

设备断开事件会在设备持续不可达并超过配置的离线时间后发布。

订阅全部设备事件：

```text
openwrt2mqtt/OpenWrt/network/device/+
```

## 消息示例

```json
{
  "schema_version": "1",
  "event_id": "8f69af77935d0b4a1902c203179348fa",
  "router_id": "OpenWrt",
  "category": "network",
  "type": "device.connected",
  "source": "dhcp/br-lan",
  "timestamp": "2026-07-30T05:18:21Z",
  "data": {
    "mac": "AA:BB:CC:DD:EE:FF",
    "ip": "192.168.1.100",
    "hostname": "example-device"
  }
}
```

## 构建

仓库工作流当前使用 OpenWrt `25.12.5`、`rockchip/armv8` SDK 构建 APK，并生成 `SHA256SUMS`。

## 许可证

本项目使用 [MIT License](LICENSE)。
