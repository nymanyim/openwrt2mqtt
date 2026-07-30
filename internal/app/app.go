package app

import (
 "context";"fmt"
 "github.com/nymanyim/openwrt2mqtt/internal/bus/memory"
 "github.com/nymanyim/openwrt2mqtt/internal/collector"
 "github.com/nymanyim/openwrt2mqtt/internal/collector/dhcp"
 "github.com/nymanyim/openwrt2mqtt/internal/collector/hostapd"
 "github.com/nymanyim/openwrt2mqtt/internal/collector/neighbor"
 "github.com/nymanyim/openwrt2mqtt/internal/config"
 "github.com/nymanyim/openwrt2mqtt/internal/pipeline"
 "github.com/nymanyim/openwrt2mqtt/internal/processor"
 "github.com/nymanyim/openwrt2mqtt/internal/publisher"
 mqttPublisher "github.com/nymanyim/openwrt2mqtt/internal/publisher/mqtt"
)
type App struct{version string;collector collector.Collector;pipeline *pipeline.Pipeline;publisher publisher.Publisher}
func New(version string)*App{return &App{version:version}}
func NewRuntime(ctx context.Context,version string,runtimeConfig config.Runtime)(*App,error){if !runtimeConfig.DeviceConnectedEnabled&&!runtimeConfig.DeviceDisconnectedEnabled{return New(version),nil};output,err:=mqttPublisher.New(ctx,mqttPublisher.Config{Broker:runtimeConfig.MQTTBroker,ClientID:runtimeConfig.MQTTClientID,Username:runtimeConfig.MQTTUsername,Password:runtimeConfig.MQTTPassword,TopicPrefix:runtimeConfig.MQTTTopic,QoS:runtimeConfig.MQTTQoS,Timeout:runtimeConfig.MQTTTimeout});if err!=nil{return nil,err};eventBus:=memory.New(runtimeConfig.BusCapacity);return &App{version:version,collector:collector.NewMulti(dhcp.NewCollector(runtimeConfig.Interface,runtimeConfig.RouterID),hostapd.NewCollector(runtimeConfig.RouterID),neighbor.NewCollector(runtimeConfig.Interface,runtimeConfig.RouterID,runtimeConfig.OfflineTimeout,runtimeConfig.DeviceDisconnectedEnabled)),pipeline:pipeline.New(eventBus,output,processor.NewDeviceState(runtimeConfig.DeviceConnectedEnabled,runtimeConfig.DeviceDisconnectedEnabled)),publisher:output},nil}
func(a *App)Run(ctx context.Context)error{if a.version==""{return fmt.Errorf("version must not be empty")};if a.collector==nil&&a.pipeline==nil{<-ctx.Done();return nil};if a.collector==nil||a.pipeline==nil||a.publisher==nil{return fmt.Errorf("runtime components must be configured together")};defer a.publisher.Close();runCtx,cancel:=context.WithCancel(ctx);defer cancel();errors:=make(chan error,2);go func(){errors<-a.pipeline.Run(runCtx)}();go func(){errors<-a.collector.Start(runCtx,a.pipeline)}();firstError:=<-errors;cancel();secondError:=<-errors;if firstError!=nil{return firstError};return secondError}
