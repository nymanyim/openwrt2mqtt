package neighbor

import (
 "crypto/sha256"; "encoding/binary"; "encoding/hex"; "fmt"; "net"; "strings"; "syscall"; "time"
 "github.com/nymanyim/openwrt2mqtt/internal/event"
)
const neighborHeaderSize=12
type neighborObservation struct{ip net.IP;mac net.HardwareAddr;active bool}
func parseNeighbor(interfaceIndex int,message syscall.NetlinkMessage)*neighborObservation{if message.Header.Type!=rtmNewNeighbor&&message.Header.Type!=rtmDelNeighbor{return nil};if len(message.Data)<neighborHeaderSize||message.Data[0]!=syscall.AF_INET||int(int32(binary.NativeEndian.Uint32(message.Data[4:8])))!=interfaceIndex{return nil};state:=nativeUint16(message.Data[8:10]);o:=&neighborObservation{active:message.Header.Type==rtmNewNeighbor&&state&(nudReachable|nudStale|nudDelay|nudProbe|nudNoARP|nudPermanent)!=0};for offset:=neighborHeaderSize;offset+syscall.SizeofRtAttr<=len(message.Data);{length:=int(binary.NativeEndian.Uint16(message.Data[offset:offset+2]));typ:=binary.NativeEndian.Uint16(message.Data[offset+2:offset+4]);if length<syscall.SizeofRtAttr||offset+length>len(message.Data){return nil};value:=message.Data[offset+syscall.SizeofRtAttr:offset+length];if typ==ndaDestination&&len(value)==net.IPv4len{o.ip=append(net.IP(nil),value...)};if typ==ndaLinkAddress&&len(value)==6{o.mac=append(net.HardwareAddr(nil),value...)};offset+=(length+3)&^3};if o.ip==nil||len(o.mac)!=6{return nil};return o}
func neighborData(interfaceName string,ip net.IP,mac net.HardwareAddr)map[string]any{data:=map[string]any{"connection_type":"network","interface":interfaceName,"ip":ip.String(),"mac":strings.ToLower(mac.String())};for k,v:=range resolveLease(mac.String()){data[k]=v};return data}
func newEvent(routerID,interfaceName,eventType string,data map[string]any,now time.Time)event.Event{timestamp:=now.UTC();mac,_:=data["mac"].(string);copied:=make(map[string]any,len(data));for k,v:=range data{copied[k]=v};return event.Event{SchemaVersion:"1",ID:neighborEventID(routerID,eventType,mac,timestamp),RouterID:routerID,Category:"network",Type:eventType,Source:"neighbor/"+interfaceName,Timestamp:timestamp,Data:copied}}
func neighborEventID(routerID,eventType,mac string,timestamp time.Time)string{sum:=sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d",routerID,eventType,mac,timestamp.UnixNano())));return hex.EncodeToString(sum[:16])}
