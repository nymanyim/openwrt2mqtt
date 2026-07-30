//go:build linux

package dhcp

import (
	"encoding/binary"
	"testing"

	"golang.org/x/net/bpf"
)

func TestDHCPFilter(t *testing.T) {
	instructions := dhcpFilter()
	raw := make([]bpf.RawInstruction, len(instructions))
	for index, instruction := range instructions {
		raw[index] = bpf.RawInstruction{
			Op: instruction.Code,
			Jt: instruction.Jt,
			Jf: instruction.Jf,
			K:  instruction.K,
		}
	}
	decoded, allDecoded := bpf.Disassemble(raw)
	if !allDecoded {
		t.Fatal("BPF program contains an unknown instruction")
	}
	vm, err := bpf.NewVM(decoded)
	if err != nil {
		t.Fatalf("NewVM() error = %v", err)
	}

	dhcpFrame := fixtureFrame(MessageDiscover, 1, nil, nil, "test-device")
	accepted, err := vm.Run(dhcpFrame)
	if err != nil {
		t.Fatalf("Run(DHCP) error = %v", err)
	}
	if accepted == 0 {
		t.Fatal("DHCP frame was rejected")
	}

	nonDHCP := append([]byte(nil), dhcpFrame...)
	ipHeaderLen := int(nonDHCP[14]&0x0f) * 4
	udpOffset := 14 + ipHeaderLen
	binary.BigEndian.PutUint16(nonDHCP[udpOffset:udpOffset+2], 12345)
	binary.BigEndian.PutUint16(nonDHCP[udpOffset+2:udpOffset+4], 54321)
	accepted, err = vm.Run(nonDHCP)
	if err != nil {
		t.Fatalf("Run(non-DHCP) error = %v", err)
	}
	if accepted != 0 {
		t.Fatal("non-DHCP UDP frame was accepted")
	}
}
