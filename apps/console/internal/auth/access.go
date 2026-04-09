package auth

import (
	"fmt"
	"net/http"
	"strings"
)

type CFAccessIdentity struct {
	NodeID      string
	Role        string
	ClientID    string
	Subject     string
	Email       string
	Authorized  bool
	AuthType    string
}

type CFAccessVerifier struct {
	mode       string
	teamDomain string // reserved for future JWT assertion verification
	aud        string // reserved for future JWT assertion verification
	clients    map[string]allowedAccessIdentity
}

type allowedAccessIdentity struct {
	Role      string
	ClientID  string
	Subject   string
	Email     string
}

func NewCFAccessVerifier(mode, teamDomain, aud, clientsRaw string) (*CFAccessVerifier, error) {
	cm, err := parseAccessClients(clientsRaw)
	if err != nil {
		return nil, err
	}
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "", "disabled", "service_token", "jwt":
	default:
		return nil, fmt.Errorf("unsupported VBS_CF_ACCESS_MODE=%q", mode)
	}
	return &CFAccessVerifier{
		mode:       mode,
		teamDomain: strings.TrimSpace(teamDomain),
		aud:        strings.TrimSpace(aud),
		clients:    cm,
	}, nil
}

func (v *CFAccessVerifier) Mode() string { return v.mode }

func (v *CFAccessVerifier) VerifyRequest(r *http.Request) (*CFAccessIdentity, error) {
	switch v.mode {
	case "", "disabled":
		return nil, fmt.Errorf("cloudflare access disabled")
	case "service_token":
		return v.verifyServiceToken(r)
	case "jwt":
		return nil, fmt.Errorf("jwt assertion mode is not enabled yet; use service_token mode")
	default:
		return nil, fmt.Errorf("unsupported mode")
	}
}

func (v *CFAccessVerifier) verifyServiceToken(r *http.Request) (*CFAccessIdentity, error) {
	clientID := strings.TrimSpace(r.Header.Get("CF-Access-Client-Id"))
	clientSecret := strings.TrimSpace(r.Header.Get("CF-Access-Client-Secret"))
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("missing cf access service token headers")
	}
	nodeID := strings.TrimSpace(r.Header.Get("X-VBS-Node-ID"))
	if nodeID == "" {
		return nil, fmt.Errorf("missing X-VBS-Node-ID")
	}
	allowed, ok := v.clients[nodeID]
	if !ok {
		return nil, fmt.Errorf("unknown node_id")
	}
	if allowed.ClientID == "" {
		return nil, fmt.Errorf("node has no allowed client_id configured")
	}
	if clientID != allowed.ClientID {
		return nil, fmt.Errorf("client_id mismatch")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("client_secret empty")
	}
	return &CFAccessIdentity{
		NodeID:     nodeID,
		Role:       allowed.Role,
		ClientID:   clientID,
		Authorized: true,
		AuthType:   "service_token",
	}, nil
}

func parseAccessClients(raw string) (map[string]allowedAccessIdentity, error) {
	out := map[string]allowedAccessIdentity{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out, nil
	}
	// format: node_id:role:client_id[:subject][:email],...
	items := strings.Split(raw, ",")
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		parts := strings.Split(it, ":")
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid VBS_CF_ACCESS_CLIENTS item: %q", it)
		}
		nodeID := strings.TrimSpace(parts[0])
		role := strings.TrimSpace(strings.ToLower(parts[1]))
		clientID := strings.TrimSpace(parts[2])
		subject := ""
		email := ""
		if len(parts) > 3 {
			subject = strings.TrimSpace(parts[3])
		}
		if len(parts) > 4 {
			email = strings.TrimSpace(parts[4])
		}
		if nodeID == "" || role == "" {
			return nil, fmt.Errorf("invalid VBS_CF_ACCESS_CLIENTS item: %q", it)
		}
		out[nodeID] = allowedAccessIdentity{
			Role:     role,
			ClientID: clientID,
			Subject:  subject,
			Email:    email,
		}
	}
	return out, nil
}

