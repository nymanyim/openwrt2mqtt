# openwrt2mqtt

`openwrt2mqtt` 用于采集 OpenWrt 网络事件，将事件标准化后发布到 MQTT Broker。

当前版本为 `1.0.0`，提供 OpenWrt 后台服务、UCI 配置、ubus 接口、LuCI 管理页面和简体中文翻译。

## 功能特性

- 通过 Linux `AF_PACKET` 和经典 BPF 过滤器采集 DHCP 流量；
- 识别设备完成 DHCP 地址获取的过程；
- 忽略普通 DHCP 租约续期，减少重复消息；
- 使用有界内存队列处理事件；
- 将标准化事件发布到 MQTT；
- 支持 MQTT 账号、密码、QoS、连接超时和 Topic 前缀；
- 使用 UCI 保存配置，使用 procd 管理服务生命周期；
- 提供服务状态、服务重载和 MQTT 连接测试接口；
- 提供 LuCI 配置页面和简体中文界面；
- 提供 OpenWrt `25.12.5`、`rockchip/armv8` APK 构建工作流。

## 架构

```text
Collector -> Event Bus -> Processor -> Publisher
```

- `Collector`：采集 OpenWrt 网络事件；
- `Event Bus`：使用有界队列传递标准化事件；
- `Processor`：验证、过滤和去重事件；
- `Publisher`：将事件发布到 MQTT。

各模块通过标准化事件解耦，采集逻辑不依赖 MQTT，发布逻辑也不依赖具体采集方式。

## 安装

构建产物包含三个 APK：

- `openwrt2mqtt-*.apk`：后台服务；
- `luci-app-openwrt2mqtt-*.apk`：LuCI 管理页面；
- `luci-i18n-openwrt2mqtt-zh-cn-*.apk`：简体中文翻译。

将三个 APK 上传到路由器后执行：

```sh
apk add --allow-untrusted ./openwrt2mqtt-*.apk
apk add --allow-untrusted ./luci-app-openwrt2mqtt-*.apk
apk add --allow-untrusted ./luci-i18n-openwrt2mqtt-zh-cn-*.apk
```

安装完成后，在 LuCI 中进入：

```text
服务 -> OpenWrt to MQTT
```

## LuCI 配置

### 常用设置

- 启用或停用服务；
- 配置 MQTT 服务器；
- 配置 MQTT 用户名和密码；
- 测试当前已保存的 MQTT 连接。

MQTT 服务器默认值为：

```text
127.0.0.1:1883
```

MQTT 用户名和密码默认均为空。

为避免泄露凭据，LuCI 不会把已保存的密码读取到浏览器。只有输入新密码时才会更新 UCI 中的密码。

### 事件设置

当前支持设备接入事件：

```text
network.device.connected
```

设备通过 DHCP 获取地址后会产生该事件，普通租约续期不会重复发布。

“设备接入”标题后的信息按钮可以查看 JSON 消息示例。

### 高级设置

- DHCP 监听接口，默认 `br-lan`；
- 日志级别，支持 `debug`、`info`、`warn`、`error`；
- 事件队列容量，默认 `128`。

## UCI 配置

配置文件路径：

```text
/etc/config/openwrt2mqtt
```

默认配置：

```uci
config openwrt2mqtt 'main'
	option enabled '0'
	option router_id ''
	option interface 'br-lan'
	option log_level 'info'
	option bus_capacity '128'

config event 'network_device_connected'
	option enabled '1'

config mqtt 'mqtt'
	option broker '127.0.0.1:1883'
	option client_id ''
	option username ''
	option password ''
	option topic 'openwrt2mqtt'
	option qos '0'
	option timeout '10s'
```

`router_id` 为空时使用 OpenWrt 系统主机名；`client_id` 为空时使用最终的 `router_id`。

修改配置后执行：

```sh
/etc/init.d/openwrt2mqtt reload
```

## 服务管理

```sh
# 启动服务
/etc/init.d/openwrt2mqtt start

# 停止服务
/etc/init.d/openwrt2mqtt stop

# 重启服务
/etc/init.d/openwrt2mqtt restart

# 重载配置
/etc/init.d/openwrt2mqtt reload

# 检查启用状态下的配置
/etc/init.d/openwrt2mqtt validate

# 检查完整配置
/etc/init.d/openwrt2mqtt configured

# 测试已保存的 MQTT 连接
/etc/init.d/openwrt2mqtt mqtt_test
```

查看日志：

```sh
logread -e openwrt2mqtt
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
    "transaction_id": "1234abcd",
    "ip": "192.168.1.100",
    "hostname": "example-device",
    "server_ip": "192.168.1.1"
  }
}
```

实际消息内容以设备和 DHCP 交互结果为准。

## GitHub Actions 构建与发布

工作流文件：

```text
.github/workflows/build-openwrt.yml
```

工作流会执行以下操作：

1. 运行 Go 测试；
2. 下载并校验 OpenWrt SDK；
3. 构建后台服务、LuCI 页面和简体中文翻译三个 APK；
4. 校验 APK 数量并生成 `SHA256SUMS`；
5. 上传 GitHub Actions Artifact。

### 手动构建

在 GitHub 仓库中进入：

```text
Actions -> Build OpenWrt APKs -> Run workflow
```

选择目标分支，不勾选 Release 开关即可只构建 Artifact。

### 正式发布 Release

正式发布仅允许从 `main` 分支执行：

1. 确认 `package/openwrt2mqtt/Makefile` 中的 `PKG_VERSION` 是准备发布的新版本；
2. 进入 `Actions -> Build OpenWrt APKs -> Run workflow`；
3. 分支选择 `main`；
4. 勾选“构建成功后发布 GitHub Release”；
5. 启动工作流。

构建和 APK 校验全部成功后，工作流会：

- 创建 `v<PKG_VERSION>` 标签；
- 创建正式 GitHub Release；
- 自动生成 Release Notes；
- 上传三个 APK、`SHA256SUMS` 和 `FILE_TYPES.txt`。

如果对应版本标签已经存在，发布作业会终止。发布新版本前必须先更新 `PKG_VERSION`。

## 开发验证

```sh
go test ./...
node --check package/luci-app-openwrt2mqtt/htdocs/luci-static/resources/view/openwrt2mqtt/settings.js
```

## 许可证

本项目使用 [MIT License](LICENSE)。
