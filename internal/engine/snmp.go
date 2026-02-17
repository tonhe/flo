package engine

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tonhe/flo/internal/identity"
)

// SNMP OIDs for interface monitoring.
const (
	OIDifName        = "1.3.6.1.2.1.31.1.1.1.1"
	OIDifDescr       = "1.3.6.1.2.1.2.2.1.2"
	OIDifAlias       = "1.3.6.1.2.1.31.1.1.1.18"
	OIDifHCInOctets  = "1.3.6.1.2.1.31.1.1.1.6"
	OIDifHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10"
	OIDifHighSpeed   = "1.3.6.1.2.1.31.1.1.1.15"
	OIDifOperStatus  = "1.3.6.1.2.1.2.2.1.8"
)

// NewSNMPClient creates a gosnmp.GoSNMP client configured from an Identity.
func NewSNMPClient(host string, port int, id *identity.Identity, timeout time.Duration) (*gosnmp.GoSNMP, error) {
	if port == 0 {
		port = 161
	}
	client := &gosnmp.GoSNMP{
		Target:  host,
		Port:    uint16(port),
		Timeout: timeout,
		Retries: 2,
	}

	switch id.Version {
	case "1":
		client.Version = gosnmp.Version1
		client.Community = id.Community
	case "2c":
		client.Version = gosnmp.Version2c
		client.Community = id.Community
	case "3":
		client.Version = gosnmp.Version3
		client.SecurityModel = gosnmp.UserSecurityModel
		client.MsgFlags = snmpv3MsgFlags(id)
		client.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 id.Username,
			AuthenticationProtocol:   snmpv3AuthProto(id.AuthProto),
			AuthenticationPassphrase: id.AuthPass,
			PrivacyProtocol:          snmpv3PrivProto(id.PrivProto),
			PrivacyPassphrase:        id.PrivPass,
		}
	default:
		return nil, fmt.Errorf("unsupported SNMP version: %s", id.Version)
	}
	return client, nil
}

func snmpv3MsgFlags(id *identity.Identity) gosnmp.SnmpV3MsgFlags {
	if id.PrivProto != "" && id.PrivPass != "" {
		return gosnmp.AuthPriv
	}
	if id.AuthProto != "" && id.AuthPass != "" {
		return gosnmp.AuthNoPriv
	}
	return gosnmp.NoAuthNoPriv
}

func snmpv3AuthProto(proto string) gosnmp.SnmpV3AuthProtocol {
	switch proto {
	case "MD5":
		return gosnmp.MD5
	case "SHA":
		return gosnmp.SHA
	case "SHA256":
		return gosnmp.SHA256
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

func snmpv3PrivProto(proto string) gosnmp.SnmpV3PrivProtocol {
	switch proto {
	case "DES":
		return gosnmp.DES
	case "AES", "AES128":
		return gosnmp.AES
	case "AES192":
		return gosnmp.AES192
	case "AES256":
		return gosnmp.AES256
	default:
		return gosnmp.NoPriv
	}
}
