package dhcp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/nymanyim/openwrt2mqtt/internal/event"
)

const defaultTransactionTTL = 2 * time.Minute

type transaction struct {
	discovered  bool
	requested   bool
	expiresAt   time.Time
	requestedIP net.IP
	hostname    string
}

// Tracker correlates DHCP messages and suppresses lease renewals.
type Tracker struct {
	routerID     string
	source       string
	ttl          time.Duration
	transactions map[string]transaction
}

func NewTracker(routerID, source string) *Tracker {
	return &Tracker{
		routerID:     routerID,
		source:       source,
		ttl:          defaultTransactionTTL,
		transactions: make(map[string]transaction),
	}
}

// Observe emits only after DISCOVER -> REQUEST -> ACK for one transaction.
func (t *Tracker) Observe(message Message, now time.Time) *event.Event {
	t.removeExpired(now)
	key := transactionKey(message)

	switch message.Type {
	case MessageDiscover:
		t.transactions[key] = transaction{
			discovered: true,
			expiresAt:  now.Add(t.ttl),
			hostname:   message.Hostname,
		}
	case MessageRequest:
		current, exists := t.transactions[key]
		if !exists || !current.discovered {
			return nil
		}
		current.requested = true
		current.expiresAt = now.Add(t.ttl)
		current.requestedIP = cloneIP(message.RequestedIP)
		if message.Hostname != "" {
			current.hostname = message.Hostname
		}
		t.transactions[key] = current
	case MessageACK:
		current, exists := t.transactions[key]
		if !exists || !current.discovered || !current.requested {
			return nil
		}
		delete(t.transactions, key)
		return t.connectedEvent(message, current, now)
	case MessageNAK:
		delete(t.transactions, key)
	}
	return nil
}

func (t *Tracker) connectedEvent(message Message, current transaction, now time.Time) *event.Event {
	ip := message.YourIP
	if len(ip) == 0 {
		ip = current.requestedIP
	}
	data := map[string]any{
		"mac":            message.ClientMAC,
		"transaction_id": fmt.Sprintf("%08x", message.Transaction),
	}
	if len(ip) != 0 {
		data["ip"] = ip.String()
	}
	if current.hostname != "" {
		data["hostname"] = current.hostname
	}
	if len(message.ServerIP) != 0 {
		data["server_ip"] = message.ServerIP.String()
	}

	timestamp := now.UTC()
	return &event.Event{
		SchemaVersion: "1",
		ID:            eventID(t.routerID, message, timestamp),
		RouterID:      t.routerID,
		Category:      "network",
		Type:          "device.connected",
		Source:        t.source,
		Timestamp:     timestamp,
		Data:          data,
	}
}

func (t *Tracker) removeExpired(now time.Time) {
	for key, current := range t.transactions {
		if !current.expiresAt.After(now) {
			delete(t.transactions, key)
		}
	}
}

func transactionKey(message Message) string {
	return fmt.Sprintf("%08x/%s", message.Transaction, message.ClientMAC)
}

func eventID(routerID string, message Message, timestamp time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%08x/%s/%d", routerID, message.Transaction, message.ClientMAC, timestamp.UnixNano())))
	return hex.EncodeToString(sum[:16])
}

func cloneIP(ip net.IP) net.IP {
	return append(net.IP(nil), ip...)
}
