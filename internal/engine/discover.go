package engine

import (
	"encoding/hex"
	"fmt"
	"sort"
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

// NeighborInfo holds CDP or LLDP neighbor data for an interface.
type NeighborInfo struct {
	Protocol   string // "CDP" or "LLDP"
	DeviceID   string
	RemotePort string
	Platform   string // CDP only
}

// IPAddress holds an IP address and its subnet mask associated with an interface.
type IPAddress struct {
	Address string
	Mask    string
}

// DetailedInterface extends DiscoveredInterface with additional metadata
// fetched during extended discovery.
type DetailedInterface struct {
	DiscoveredInterface
	AdminStatus string // "up", "down", "testing"
	IfType      int
	IfTypeName  string
	MTU         int
	MACAddress  string
	IPAddresses []IPAddress
	Neighbors   []NeighborInfo
}

// DiscoverDetailedInterfaces performs an extended SNMP discovery, fetching
// all base interface data plus type, admin status, MTU, MAC, IP addresses,
// and CDP/LLDP neighbors. The optional progress callback is called with a
// status string for each phase of the discovery so the UI can show progress.
func DiscoverDetailedInterfaces(host string, port int, id *identity.Identity, progress func(string)) ([]DetailedInterface, error) {
	if progress == nil {
		progress = func(string) {}
	}

	progress("Connecting...")
	client, err := NewSNMPClient(host, port, id, 5*time.Second)
	if err != nil {
		return nil, err
	}
	client.Retries = 1
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect to %s: %w", host, err)
	}
	defer client.Conn.Close()

	interfaces := make(map[int]*DetailedInterface)

	// Phase 1: base discovery
	progress("Discovering interface names...")
	walkOID(client, OIDifName, func(idx int, val string) {
		if _, ok := interfaces[idx]; !ok {
			interfaces[idx] = &DetailedInterface{}
			interfaces[idx].IfIndex = idx
		}
		interfaces[idx].Name = val
	})

	progress("Reading descriptions...")
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

	progress("Reading speeds and status...")
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

	// Phase 2: extended attributes
	progress("Reading interface details...")
	walkOID(client, OIDifAdminStat, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			v, _ := strconv.Atoi(val)
			switch v {
			case 1:
				iface.AdminStatus = "up"
			case 2:
				iface.AdminStatus = "down"
			case 3:
				iface.AdminStatus = "testing"
			default:
				iface.AdminStatus = "unknown"
			}
		}
	})

	walkOID(client, OIDifType, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			v, _ := strconv.Atoi(val)
			iface.IfType = v
			iface.IfTypeName = ifTypeName(v)
		}
	})

	walkOID(client, OIDifMtu, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			v, _ := strconv.Atoi(val)
			iface.MTU = v
		}
	})

	walkOIDRaw(client, OIDifPhysAddr, func(idx int, val []byte) {
		if iface, ok := interfaces[idx]; ok && len(val) == 6 {
			iface.MACAddress = formatMAC(val)
		}
	})

	// Phase 3: IP addresses
	progress("Reading IP addresses...")
	ipToIfIndex := make(map[string]int)
	walkOIDMultiIndex(client, OIDipAdEntIfIdx, func(suffix string, val string) {
		ifIdx, _ := strconv.Atoi(val)
		ipToIfIndex[suffix] = ifIdx
	})

	ipMasks := make(map[string]string)
	walkOIDMultiIndex(client, OIDipAdEntMask, func(suffix string, val string) {
		ipMasks[suffix] = val
	})

	for ipSuffix, ifIdx := range ipToIfIndex {
		if iface, ok := interfaces[ifIdx]; ok {
			mask := ipMasks[ipSuffix]
			iface.IPAddresses = append(iface.IPAddresses, IPAddress{
				Address: ipSuffix,
				Mask:    mask,
			})
		}
	}

	// Phase 4: CDP neighbors (best-effort, short timeout)
	progress("Checking CDP neighbors...")
	cdpClient, cdpErr := NewSNMPClient(host, port, id, 3*time.Second)
	if cdpErr == nil {
		cdpClient.Retries = 0
		if cdpErr = cdpClient.Connect(); cdpErr == nil {
			defer cdpClient.Conn.Close()

			cdpDevices := make(map[string]string)
			walkOIDMultiIndex(cdpClient, OIDcdpCacheDevId, func(suffix string, val string) {
				cdpDevices[suffix] = val
			})

			if len(cdpDevices) > 0 {
				cdpPorts := make(map[string]string)
				walkOIDMultiIndex(cdpClient, OIDcdpCachePort, func(suffix string, val string) {
					cdpPorts[suffix] = val
				})
				cdpPlatforms := make(map[string]string)
				walkOIDMultiIndex(cdpClient, OIDcdpCachePlatform, func(suffix string, val string) {
					cdpPlatforms[suffix] = val
				})

				for suffix, deviceID := range cdpDevices {
					parts := strings.SplitN(suffix, ".", 2)
					if len(parts) < 1 {
						continue
					}
					ifIdx, _ := strconv.Atoi(parts[0])
					if iface, ok := interfaces[ifIdx]; ok {
						iface.Neighbors = append(iface.Neighbors, NeighborInfo{
							Protocol:   "CDP",
							DeviceID:   deviceID,
							RemotePort: cdpPorts[suffix],
							Platform:   cdpPlatforms[suffix],
						})
					}
				}
			}
		}
	}

	// Phase 5: LLDP neighbors (best-effort, short timeout)
	progress("Checking LLDP neighbors...")
	lldpClient, lldpErr := NewSNMPClient(host, port, id, 3*time.Second)
	if lldpErr == nil {
		lldpClient.Retries = 0
		if lldpErr = lldpClient.Connect(); lldpErr == nil {
			defer lldpClient.Conn.Close()

			lldpNames := make(map[string]string)
			walkOIDMultiIndex(lldpClient, OIDlldpRemSysName, func(suffix string, val string) {
				lldpNames[suffix] = val
			})

			if len(lldpNames) > 0 {
				lldpPortIds := make(map[string]string)
				walkOIDMultiIndex(lldpClient, OIDlldpRemPortId, func(suffix string, val string) {
					lldpPortIds[suffix] = val
				})
				lldpPortDescs := make(map[string]string)
				walkOIDMultiIndex(lldpClient, OIDlldpRemPortDesc, func(suffix string, val string) {
					lldpPortDescs[suffix] = val
				})

				for suffix, sysName := range lldpNames {
					parts := strings.SplitN(suffix, ".", 3)
					if len(parts) < 2 {
						continue
					}
					localPort, _ := strconv.Atoi(parts[1])
					if iface, ok := interfaces[localPort]; ok {
						remPort := lldpPortDescs[suffix]
						if remPort == "" {
							remPort = lldpPortIds[suffix]
						}
						iface.Neighbors = append(iface.Neighbors, NeighborInfo{
							Protocol:   "LLDP",
							DeviceID:   sysName,
							RemotePort: remPort,
						})
					}
				}
			}
		}
	}

	progress("Done")
	result := make([]DetailedInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		result = append(result, *iface)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].IfIndex < result[j].IfIndex
	})
	return result, nil
}

// ifTypeName returns a human-readable name for an IANAifType value.
func ifTypeName(t int) string {
	names := map[int]string{
		1: "other", 6: "ethernetCsmacd", 23: "ppp",
		24: "softwareLoopback", 53: "propVirtual", 71: "ieee80211",
		131: "tunnel", 135: "l2vlan", 136: "l3ipvlan",
		150: "mplsTunnel", 161: "ieee8023adLag", 209: "bridge",
	}
	if name, ok := names[t]; ok {
		return name
	}
	return fmt.Sprintf("type(%d)", t)
}

// formatMAC formats a 6-byte MAC address as a colon-separated hex string.
func formatMAC(b []byte) string {
	return hex.EncodeToString(b[:1]) + ":" +
		hex.EncodeToString(b[1:2]) + ":" +
		hex.EncodeToString(b[2:3]) + ":" +
		hex.EncodeToString(b[3:4]) + ":" +
		hex.EncodeToString(b[4:5]) + ":" +
		hex.EncodeToString(b[5:6])
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

// walkOIDRaw performs an SNMP BulkWalk and passes the raw byte value to the
// handler. Useful for binary fields like MAC addresses.
func walkOIDRaw(client *gosnmp.GoSNMP, oid string, handler func(int, []byte)) {
	_ = client.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) == 0 {
			return nil
		}
		idx, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return nil
		}
		if pdu.Type == gosnmp.OctetString {
			if b, ok := pdu.Value.([]byte); ok {
				handler(idx, b)
			}
		}
		return nil
	})
}

// walkOIDMultiIndex performs an SNMP BulkWalk and passes the OID suffix
// (everything after the base OID) plus the string value to the handler.
// This supports tables with multi-component indices like CDP and LLDP.
func walkOIDMultiIndex(client *gosnmp.GoSNMP, oid string, handler func(suffix string, val string)) {
	_ = client.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		suffix := strings.TrimPrefix(pdu.Name, "."+oid)
		suffix = strings.TrimPrefix(suffix, ".")
		if suffix == "" || suffix == pdu.Name {
			return nil
		}
		var val string
		switch pdu.Type {
		case gosnmp.OctetString:
			val = string(pdu.Value.([]byte))
		default:
			val = fmt.Sprintf("%d", gosnmp.ToBigInt(pdu.Value))
		}
		handler(suffix, val)
		return nil
	})
}
