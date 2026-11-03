package auth

import (
	"strings"

	"github.com/thun888/gdocs-go/internal/store"
)

var roleRank = map[string]int{"viewer": 1, "editor": 2, "admin": 3}

func roleAtLeast(role, want string) bool {
	return roleRank[role] >= roleRank[want]
}

func ParseGroupCode(code string) (typ, name string) {
	typ, rest, ok := strings.Cut(code, ":")
	if !ok {
		return "", code
	}
	if i := strings.LastIndex(rest, "/"); i >= 0 {
		return typ, rest[i+1:]
	}
	return typ, rest
}

func RoleFromGroups(groups []store.Group) string {
	role := "viewer"
	for _, g := range groups {
		if g.Ignored {
			continue
		}
		if roleRank[g.Role] > roleRank[role] {
			role = g.Role
		}
	}
	return role
}

func IsBootstrapAdmin(subject string, list []string) bool {
	for _, item := range list {
		if item == subject {
			return true
		}
	}
	return false
}

func GroupAllowed(codes, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, code := range codes {
		for _, pattern := range patterns {
			if pattern == code {
				return true
			}
			if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
				if strings.HasPrefix(code, prefix) {
					return true
				}
			}
		}
	}
	return false
}
