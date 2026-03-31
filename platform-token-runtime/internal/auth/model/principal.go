package model

import "strings"

const (
	RoleOwner  = "owner"
	RoleViewer = "viewer"
	RoleAdmin  = "admin"
)

type Principal struct {
	RequestID string
	TokenID   string
	SubjectID string
	Role      string
	Scope     []string
}

func (p Principal) HasScope(required string) bool {
	if required == "" {
		return true
	}
	for _, scope := range p.Scope {
		if scope == required {
			return true
		}
		if strings.HasSuffix(scope, ":*") {
			prefix := strings.TrimSuffix(scope, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}
