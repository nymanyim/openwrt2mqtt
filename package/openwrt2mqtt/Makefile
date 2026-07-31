include $(TOPDIR)/rules.mk

PKG_NAME:=openwrt2mqtt
PKG_VERSION:=1.0.1
PKG_RELEASE:=1

PKG_MAINTAINER:=nymanyim
PKG_LICENSE:=MIT
PKG_LICENSE_FILES:=LICENSE

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1
PKG_BUILD_FLAGS:=no-mips16
PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)
OPENWRT2MQTT_SOURCE_DIR?=$(CURDIR)/../..

GO_PKG:=github.com/nymanyim/openwrt2mqtt
GO_PKG_BUILD_PKG:=$(GO_PKG)/cmd/openwrt2mqtt
GO_PKG_LDFLAGS_X:=main.version=$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/openwrt2mqtt
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Publish OpenWrt network events to MQTT
  URL:=https://github.com/nymanyim/openwrt2mqtt
  DEPENDS:=$(GO_ARCH_DEPENDS) +ca-bundle +procd +rpcd +uci
endef

define Package/openwrt2mqtt/description
 openwrt2mqtt publishes selected OpenWrt network events to an MQTT broker.
endef

define Package/openwrt2mqtt/conffiles
/etc/config/openwrt2mqtt
endef

define Build/Prepare
	$(RM) -r $(PKG_BUILD_DIR)
	$(INSTALL_DIR) $(PKG_BUILD_DIR)
	$(CP) $(OPENWRT2MQTT_SOURCE_DIR)/cmd $(PKG_BUILD_DIR)/
	$(CP) $(OPENWRT2MQTT_SOURCE_DIR)/internal $(PKG_BUILD_DIR)/
	$(CP) $(OPENWRT2MQTT_SOURCE_DIR)/go.mod $(OPENWRT2MQTT_SOURCE_DIR)/go.sum $(PKG_BUILD_DIR)/
	$(CP) $(OPENWRT2MQTT_SOURCE_DIR)/LICENSE $(PKG_BUILD_DIR)/
endef

define Package/openwrt2mqtt/install
	$(INSTALL_DIR) $(1)/usr/sbin
	$(INSTALL_BIN) $(GO_PKG_BUILD_BIN_DIR)/openwrt2mqtt $(1)/usr/sbin/openwrt2mqtt
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/etc/config/openwrt2mqtt $(1)/etc/config/openwrt2mqtt
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/openwrt2mqtt $(1)/etc/init.d/openwrt2mqtt
	$(INSTALL_DIR) $(1)/usr/libexec/rpcd
	$(INSTALL_BIN) ./files/usr/libexec/rpcd/openwrt2mqtt $(1)/usr/libexec/rpcd/openwrt2mqtt
	$(INSTALL_DIR) $(1)/usr/libexec/openwrt2mqtt
	$(INSTALL_BIN) ./files/usr/libexec/openwrt2mqtt/migrate-config $(1)/usr/libexec/openwrt2mqtt/migrate-config
endef

define Package/openwrt2mqtt/postinst
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] || {
	/usr/libexec/openwrt2mqtt/migrate-config || exit 1
	/etc/init.d/rpcd reload
}
endef

$(eval $(call BuildPackage,openwrt2mqtt))
