package auth

import "testing"

func TestAdminMiddlewareAllowed_Switch(t *testing.T) {
	if !AdminMiddlewareAllowed("POST", "/api/v1/switch/program") {
		t.Fatal("expected POST /api/v1/switch/program")
	}
	if !AdminMiddlewareAllowed("POST", "/api/v1/switch/preview") {
		t.Fatal("expected POST /api/v1/switch/preview")
	}
	if AdminMiddlewareAllowed("GET", "/api/v1/switch/program") {
		t.Fatal("method must not match")
	}
}

func TestAdminMiddlewareAllowed_ProxyPrefix(t *testing.T) {
	if !AdminMiddlewareAllowed("GET", "/api/proxy/healthz") {
		t.Fatal("expected bff proxy prefix")
	}
	if !AdminMiddlewareAllowed("POST", "/api/proxy/api/v1/switch/program") {
		t.Fatal("expected bff proxy prefix for nested path")
	}
}

func TestAdminMiddlewareAllowed_GuestSessionDelete(t *testing.T) {
	if !AdminMiddlewareAllowed("DELETE", "/api/v1/guest/sessions/abc-123") {
		t.Fatal("expected delete session by id")
	}
	if AdminMiddlewareAllowed("DELETE", "/api/v1/guest/sessions") {
		t.Fatal("should not match collection path for prefix rule")
	}
	if AdminMiddlewareAllowed("DELETE", "/api/v1/guest/sessions/") {
		t.Fatal("should not match bare sessions path")
	}
}

func TestNormalizeRBACPath(t *testing.T) {
	if p := NormalizeRBACPath("/api/v1/switch/program/"); p != "/api/v1/switch/program" {
		t.Fatalf("trailing slash: got %q", p)
	}
}
