package httpserver

import "testing"

func TestServicePathAllowed(t *testing.T) {
	s := &Server{}
	tests := []struct {
		name   string
		role   string
		method string
		path   string
		want   bool
	}{
		{name: "bff proxy allowed", role: "bff", method: "GET", path: "/api/proxy/api/v1/route/metrics", want: true},
		{name: "bff direct api denied", role: "bff", method: "GET", path: "/api/v1/route/metrics", want: false},
		{name: "engine telemetry ws allowed", role: "engine", method: "GET", path: "/vbs/telemetry/ws", want: true},
		{name: "engine events denied", role: "engine", method: "GET", path: "/vbs/telemetry/events/ws", want: false},
		{name: "route introspect allowed", role: "route", method: "POST", path: "/api/v1/guest/introspect", want: true},
		{name: "route introspect wrong method denied", role: "route", method: "GET", path: "/api/v1/guest/introspect", want: false},
		{name: "console control ws allowed", role: "console", method: "GET", path: "/vbs/control/ws", want: true},
		{name: "capture runtime denied", role: "capture", method: "PUT", path: "/api/v1/runtime/config", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.servicePathAllowed(tt.role, tt.method, tt.path)
			if got != tt.want {
				t.Fatalf("servicePathAllowed(%q,%q,%q)=%v want=%v", tt.role, tt.method, tt.path, got, tt.want)
			}
		})
	}
}
