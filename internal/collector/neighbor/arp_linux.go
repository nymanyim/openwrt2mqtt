//go:build linux

package neighbor

import (
 "encoding/binary"
 "errors"
 "net"
 "syscall"
 "time"
)

const ( etherTypeARP = 0x0806; etherTypeIPv4 = 0x0800; arpPacketSize = 42 )

func probeARP(device *net.Interface, sourceIP, targetIP net.IP, targetMAC net.HardwareAddr, timeout time.Duration) bool {
 fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(etherTypeARP))); if err != nil { return false }; defer syscall.Close(fd)
 if syscall.Bind(fd, &syscall.SockaddrLinklayer{Protocol: htons(etherTypeARP), Ifindex: device.Index}) != nil { return false }; deadline := time.Now().Add(timeout)
 packet := make([]byte, arpPacketSize); copy(packet[0:6], []byte{255,255,255,255,255,255}); copy(packet[6:12], device.HardwareAddr); binary.BigEndian.PutUint16(packet[12:14], etherTypeARP); binary.BigEndian.PutUint16(packet[14:16], 1); binary.BigEndian.PutUint16(packet[16:18], etherTypeIPv4); packet[18],packet[19]=6,4; binary.BigEndian.PutUint16(packet[20:22],1); copy(packet[22:28],device.HardwareAddr); copy(packet[28:32],sourceIP.To4()); copy(packet[38:42],targetIP.To4())
 if syscall.Sendto(fd,packet,0,&syscall.SockaddrLinklayer{Protocol:htons(etherTypeARP),Ifindex:device.Index,Halen:6,Addr:[8]uint8{255,255,255,255,255,255}}) != nil { return false }
 buffer:=make([]byte,256); for { remaining:=time.Until(deadline); if remaining<=0{return false}; if syscall.SetsockoptTimeval(fd,syscall.SOL_SOCKET,syscall.SO_RCVTIMEO,&syscall.Timeval{Sec:int64(remaining/time.Second),Usec:int64((remaining%time.Second)/time.Microsecond)})!=nil{return false}; n,_,err:=syscall.Recvfrom(fd,buffer,0); if err!=nil{return false}; if n<arpPacketSize||binary.BigEndian.Uint16(buffer[20:22])!=2{continue}; if net.IP(buffer[28:32]).Equal(targetIP)&&net.HardwareAddr(buffer[22:28]).String()==targetMAC.String(){return true} }
}
func interfaceIPv4(device *net.Interface)(net.IP,error){ addresses,err:=device.Addrs();if err!=nil{return nil,err};for _,address:=range addresses{var ip net.IP;switch value:=address.(type){case *net.IPNet:ip=value.IP;case *net.IPAddr:ip=value.IP};if ip4:=ip.To4();ip4!=nil{return ip4,nil}};return nil,errors.New("interface has no IPv4 address") }
func htons(value uint16)uint16{return value<<8|value>>8}
