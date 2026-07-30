package hostapd

import (
	"bufio"
	"os"
	"strings"
)

const leaseFile = "/tmp/dhcp.leases"

func resolveLease(mac string) map[string]any {
	file, err := os.Open(leaseFile)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || !strings.EqualFold(fields[1], mac) {
			continue
		}
		data := map[string]any{"ip": fields[2]}
		if fields[3] != "*" {
			data["hostname"] = fields[3]
		}
		return data
	}
	return nil
}
