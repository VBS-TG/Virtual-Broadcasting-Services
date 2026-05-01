package auth

import "strings"

// NormalizeRBACPath normalizes URL paths for RBAC comparison:
// trims space, ensures leading '/', strips trailing slashes (except root).
func NormalizeRBACPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

// adminMiddlewareExact lists allowed (method, path) pairs for role admin at the
// withCORS middleware layer. Paths must be normalized (see NormalizeRBACPath).
// Keep in sync with httpserver.Server.routes().
var adminMiddlewareExact = []struct {
	Method string
	Path   string
}{
	{Method: "GET", Path: "/api/v1/auth/session"},
	{Method: "GET", Path: "/vbs/telemetry/ws"},
	{Method: "GET", Path: "/vbs/telemetry/events/ws"},
	{Method: "GET", Path: "/vbs/control/ws"},
	{Method: "GET", Path: "/api/v1/telemetry/latest"},
	{Method: "POST", Path: "/api/v1/stream/session-key"},
	{Method: "POST", Path: "/api/v1/pgm/route-buffer"},
	{Method: "POST", Path: "/api/v1/capture/bitrate"},
	{Method: "POST", Path: "/api/v1/capture/reboot"},
	{Method: "GET", Path: "/api/v1/route/metrics"},
	{Method: "POST", Path: "/api/v1/switch/program"},
	{Method: "POST", Path: "/api/v1/switch/preview"},
	{Method: "POST", Path: "/api/v1/switch/aux"},
	{Method: "GET", Path: "/api/v1/switch/state"},
	{Method: "POST", Path: "/api/v1/engine/reset"},
	{Method: "POST", Path: "/api/v1/engine/pgm/output"},
	{Method: "GET", Path: "/api/v1/guest/sessions"},
	{Method: "POST", Path: "/api/v1/guest/sessions"},
	{Method: "POST", Path: "/api/v1/guest/introspect"},
	{Method: "GET", Path: "/api/v1/runtime/config"},
	{Method: "PUT", Path: "/api/v1/runtime/config"},
	{Method: "POST", Path: "/api/v1/runtime/config/apply"},
	{Method: "GET", Path: "/api/v1/show-config"},
	{Method: "PUT", Path: "/api/v1/show-config/draft"},
	{Method: "POST", Path: "/api/v1/show-config/apply"},
	{Method: "POST", Path: "/api/v1/show-config/rollback"},
	{Method: "GET", Path: "/api/v1/show-config/history"},
}

// adminMiddlewarePrefixes: method + path prefix (normalized).
// MatchExact: if true, path == prefix also counts (e.g. /api/proxy); if false, only paths strictly under prefix/… .
var adminMiddlewarePrefixes = []struct {
	Method     string
	Prefix     string
	MatchExact bool
}{
	{Method: "GET", Prefix: "/api/proxy/", MatchExact: true},
	{Method: "POST", Prefix: "/api/proxy/", MatchExact: true},
	{Method: "PUT", Prefix: "/api/proxy/", MatchExact: true},
	{Method: "DELETE", Prefix: "/api/proxy/", MatchExact: true},
	{Method: "DELETE", Prefix: "/api/v1/guest/sessions/", MatchExact: false},
}

// AdminMiddlewareAllowed reports whether an admin may pass HTTP middleware for method+path.
// Used for auditability: every allowed route must appear in the tables above.
func AdminMiddlewareAllowed(method, path string) bool {
	method = strings.TrimSpace(strings.ToUpper(method))
	path = NormalizeRBACPath(path)
	for _, e := range adminMiddlewareExact {
		if strings.TrimSpace(strings.ToUpper(e.Method)) == method && NormalizeRBACPath(e.Path) == path {
			return true
		}
	}
	for _, p := range adminMiddlewarePrefixes {
		if strings.TrimSpace(strings.ToUpper(p.Method)) != method {
			continue
		}
		prefix := NormalizeRBACPath(p.Prefix)
		if p.MatchExact && path == prefix {
			return true
		}
		if strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
