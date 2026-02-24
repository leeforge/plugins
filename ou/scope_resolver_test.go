package ou

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/leeforge/core/services/datascope"
)

func TestScopeResolver_ScopeTypes_ContainsOUScopes(t *testing.T) {
	r := NewScopeResolver(nil)
	scopeTypes := r.ScopeTypes()

	require.Contains(t, scopeTypes, datascope.ScopeOUSelf)
	require.Contains(t, scopeTypes, datascope.ScopeOUSubtree)
}

func TestScopeResolver_Resolve_OUSubtree(t *testing.T) {
	r := NewScopeResolver(nil)
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000401")
	domainID := uuid.MustParse("00000000-0000-0000-0000-000000000402")

	fc, err := r.Resolve(context.Background(), userID, domainID, datascope.ScopeOUSubtree, "dept-a")
	require.NoError(t, err)
	require.NotNil(t, fc)
	require.Equal(t, datascope.ScopeOUSubtree, fc.Type)
	require.Equal(t, userID, fc.UserID)
}
