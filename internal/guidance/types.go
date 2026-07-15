// Package guidance defines the transport-neutral continuation and truncation
// contracts shared by Bob's read-only repository guidance services.
package guidance

type Notice struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Code         string   `json:"code"`
	Message      string   `json:"message"`
	CapabilityID string   `json:"capability_id,omitempty"`
	Paths        []string `json:"paths,omitempty"`
}

type Action struct {
	ID                        string   `json:"id"`
	Kind                      string   `json:"kind"`
	Effect                    string   `json:"effect"`
	CWD                       string   `json:"cwd"`
	Argv                      []string `json:"argv"`
	ReasonCode                string   `json:"reason_code"`
	RequiresExplicitAuthority bool     `json:"requires_explicit_authority"`
	BlockedBy                 []string `json:"blocked_by"`
}

type Truncation struct {
	Profile   string         `json:"profile"`
	ByteLimit int            `json:"byte_limit"`
	Truncated bool           `json:"truncated"`
	Omitted   map[string]int `json:"omitted"`
}
