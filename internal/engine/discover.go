package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tonhe/flo/internal/identity"
)

// DiscoveredInterface holds the metadata for a single interface found via SNMP.
type DiscoveredInterface struct {
	IfIndex     int
	Name        string
	Description string
	Alias       string
	Speed       uint64
	Status      string
}

// DiscoverInterfaces walks a device's interface table and returns all
// discovered interfaces with their names, descriptions, speeds, and status.
func DiscoverInterfaces(host string, port int, id *identity.Identity) ([]DiscoveredInterface, error) {
	client, err := NewSNMPClient(host, port, id, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect to %s: %w", host, err)
	}
	defer client.Conn.Close()

	interfaces := make(map[int]*DiscoveredInterface)

	walkOID(client, OIDifName, func(idx int, val string) {
		if _, ok := interfaces[idx]; !ok {
			interfaces[idx] = &DiscoveredInterface{IfIndex: idx}
		}
		interfaces[idx].Name = val
	})

	walkOID(client, OIDifDescr, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok && iface.Name == "" {
			iface.Name = val
		}
		if iface, ok := interfaces[idx]; ok {
			iface.Description = val
		}
	})

	walkOID(client, OIDifAlias, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			iface.Alias = val
		}
	})

	walkOID(client, OIDifHighSpeed, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			speed, _ := strconv.ParseUint(val, 10, 64)
			iface.Speed = speed
		}
	})

	walkOID(client, OIDifOperStatus, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			v, _ := strconv.Atoi(val)
			switch v {
			case 1:
				iface.Status = "up"
			case 2:
				iface.Status = "down"
			default:
				iface.Status = "unknown"
			}
		}
	})

	result := make([]DiscoveredInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		result = append(result, *iface)
	}
	return result, nil
}

// walkOID performs an SNMP BulkWalk on the given OID and calls handler for
// each PDU, extracting the ifIndex from the last OID component.
func walkOID(client *gosnmp.GoSNMP, oid string, handler func(int, string)) {
	_ = client.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) == 0 {
			return nil
		}
		idx, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return nil
		}
		var val string
		switch pdu.Type {
		case gosnmp.OctetString:
			val = string(pdu.Value.([]byte))
		default:
			val = fmt.Sprintf("%d", gosnmp.ToBigInt(pdu.Value))
		}
		handler(idx, val)
		return nil
	})
}
