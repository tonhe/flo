package identity

// Identity represents an SNMP credential profile.
type Identity struct {
	Name      string `json:"name"`
	Version   string `json:"version"`    // "1", "2c", "3"
	Community string `json:"community"`  // v1/v2c
	Username  string `json:"username"`   // v3
	AuthProto string `json:"auth_proto"` // "MD5", "SHA", "SHA256", "SHA512"
	AuthPass  string `json:"auth_pass"`
	PrivProto string `json:"priv_proto"` // "DES", "AES128", "AES192", "AES256"
	PrivPass  string `json:"priv_pass"`
}

// Summary returns a safe representation without secrets.
type Summary struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Username  string `json:"username,omitempty"`
	AuthProto string `json:"auth_proto,omitempty"`
	PrivProto string `json:"priv_proto,omitempty"`
}

// Summarize returns a Summary without sensitive fields.
func (id *Identity) Summarize() Summary {
	return Summary{
		Name:      id.Name,
		Version:   id.Version,
		Username:  id.Username,
		AuthProto: id.AuthProto,
		PrivProto: id.PrivProto,
	}
}

// Provider is the interface for identity storage backends.
type Provider interface {
	List() ([]Summary, error)
	Get(name string) (*Identity, error)
	Add(id Identity) error
	Update(name string, id Identity) error
	Remove(name string) error
}
