# openwrt2mqtt

`openwrt2mqtt` 是运行于 OpenWrt 的事件桥接服务，用于采集系统事件、统一消息格式，并通过 MQTT 提供给监控、自动化及其他订阅端。

## 支持事件

项目当前支持以下系统事件：

| 事件 | 消息类型 | MQTT 主题 |
| --- | --- | --- |
| 设备接入 | `device.connected` | `openwrt2mqtt/OpenWrt/network/device/connected` |
| 设备断开 | `device.disconnected` | `openwrt2mqtt/OpenWrt/network/device/disconnected` |

订阅全部设备事件：

```text
openwrt2mqtt/OpenWrt/network/device/+
```

## 安装

构建产物包含三个 APK：

- `openwrt2mqtt-*.apk`：后台服务；
- `luci-app-openwrt2mqtt-*.apk`：LuCI 管理页面；
- `luci-i18n-openwrt2mqtt-zh-cn-*.apk`：简体中文翻译。

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

## 构建

仓库工作流当前使用 OpenWrt `25.12.5`、`rockchip/armv8` SDK 构建 APK，并生成 `SHA256SUMS`。

## 许可证

本项目使用 [MIT License](LICENSE)。
