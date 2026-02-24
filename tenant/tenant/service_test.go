package tenant

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/leeforge/plugins/tenant/shared"
)

// mockRoleSeeder is a no-op role seeder for testing.
type mockRoleSeeder struct{}

func (mockRoleSeeder) SeedBaselineRoles(_ context.Context, _ uuid.UUID) error { return nil }

// mockUserLookup returns a stub user for testing.
type mockUserLookup struct{}

func (mockUserLookup) GetUser(_ context.Context, userID uuid.UUID) (*shared.UserInfo, error) {
	return &shared.UserInfo{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
		Nickname: "Test User",
		Status:   "active",
	}, nil
}

func TestService_New(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, mockRoleSeeder{}, mockUserLookup{})
	require.NotNil(t, svc)
}

func TestService_Ping_NilClient(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, mockRoleSeeder{}, mockUserLookup{})
	err := svc.Ping(context.Background())
	require.Error(t, err, "Ping should return an error when client is nil")
}
