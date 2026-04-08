package nodes

import (
	"fmt"
	"strings"
)

// NodeCredential is one node bootstrap identity.
type NodeCredential struct {
	NodeID string
	Role   string
	Secret string
}

// Registry stores bootstrap identities from env.
type Registry struct {
	byNode map[string]NodeCredential
}

// ParseRegistry parses env format: "node1:role:secret,node2:role:secret".
func ParseRegistry(raw string) (*Registry, error) {
	out := &Registry{byNode: map[string]NodeCredential{}}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	items := strings.Split(raw, ",")
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		parts := strings.SplitN(it, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid VBS_CONSOLE_NODE_CREDENTIALS item: %q", it)
		}
		n := NodeCredential{
			NodeID: strings.TrimSpace(parts[0]),
			Role:   strings.TrimSpace(strings.ToLower(parts[1])),
			Secret: strings.TrimSpace(parts[2]),
		}
		if n.NodeID == "" || n.Role == "" || n.Secret == "" {
			return nil, fmt.Errorf("invalid empty field in VBS_CONSOLE_NODE_CREDENTIALS item: %q", it)
		}
		out.byNode[n.NodeID] = n
	}
	return out, nil
}

func (r *Registry) Check(nodeID, role, secret string) bool {
	n, ok := r.byNode[strings.TrimSpace(nodeID)]
	if !ok {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(role), n.Role) && secret == n.Secret
}

