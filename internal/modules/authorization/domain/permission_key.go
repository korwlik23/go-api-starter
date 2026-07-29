package domain

import (
	"errors"
	"strings"
)

var ErrInvalidPermissionKey = errors.New("invalid permission key")

type PermissionScope string

const (
	PermissionScopeOwn    PermissionScope = "own"
	PermissionScopeAny    PermissionScope = "any"
	PermissionScopeSystem PermissionScope = "system"
)

type PermissionKey struct {
	Module   string
	Resource string
	Action   string
	Scope    PermissionScope
}

func ParsePermissionKey(value string) (PermissionKey, error) {
	if value == "" || value != strings.ToLower(value) {
		return PermissionKey{}, ErrInvalidPermissionKey
	}
	segments := strings.Split(value, ".")
	if len(segments) != 4 ||
		!validPermissionSegment(segments[0], 40) ||
		!validPermissionSegment(segments[1], 0) ||
		!validPermissionSegment(segments[2], 0) {
		return PermissionKey{}, ErrInvalidPermissionKey
	}
	scope := PermissionScope(segments[3])
	switch scope {
	case PermissionScopeOwn, PermissionScopeAny, PermissionScopeSystem:
	default:
		return PermissionKey{}, ErrInvalidPermissionKey
	}
	return PermissionKey{
		Module:   segments[0],
		Resource: segments[1],
		Action:   segments[2],
		Scope:    scope,
	}, nil
}

func (key PermissionKey) String() string {
	return key.Module + "." + key.Resource + "." + key.Action + "." + string(key.Scope)
}

func validPermissionSegment(value string, maxLength int) bool {
	if value == "" ||
		(maxLength > 0 && len(value) > maxLength) ||
		value[0] < 'a' ||
		value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '_' {
			continue
		}
		return false
	}
	return true
}
