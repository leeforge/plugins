package shared

import (
	"context"

	"github.com/google/uuid"
)

// RoleSeeder seeds baseline roles for a new tenant domain.
type RoleSeeder interface {
	SeedBaselineRoles(ctx context.Context, domainID uuid.UUID) error
}

// UserLookup resolves user info for membership validation.
type UserLookup interface {
	GetUser(ctx context.Context, userID uuid.UUID) (*UserInfo, error)
}

// UserInfo is a minimal user representation for membership checks.
type UserInfo struct {
	ID       uuid.UUID
	Username string
	Email    string
	Nickname string
	Status   string
}
