package auth

import "strings"

var telemetrySenderRoles = map[string]struct{}{
	"node":    {},
	"capture": {},
	"route":   {},
	"engine":  {},
	"console": {},
}

func IsTelemetryRole(role string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	_, ok := telemetrySenderRoles[role]
	return ok
}

func IsAdminRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "admin")
}

func IsGuestRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "guest")
}

func CanControlPlane(role string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	return role == "admin" || role == "guest"
}
