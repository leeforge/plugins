package ou

import (
	"context"

	"github.com/google/uuid"

	"github.com/leeforge/core/services/datascope"
)

type organizationScopeService interface {
	GetPrimaryOrganizationID(ctx context.Context, domainID, userID uuid.UUID) (uuid.UUID, error)
	ListOrganizationUserIDs(ctx context.Context, domainID, orgID uuid.UUID) ([]uuid.UUID, error)
	ListSubtreeUserIDs(ctx context.Context, domainID, orgID uuid.UUID) ([]uuid.UUID, error)
}

// ScopeResolver resolves OU-specific data scopes.
type ScopeResolver struct {
	orgSvc organizationScopeService
}

func NewScopeResolver(orgSvc organizationScopeService) *ScopeResolver {
	return &ScopeResolver{orgSvc: orgSvc}
}

func (r *ScopeResolver) ScopeTypes() []datascope.ScopeType {
	return []datascope.ScopeType{
		datascope.ScopeOUSelf,
		datascope.ScopeOUSubtree,
	}
}

func (r *ScopeResolver) Resolve(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
	scopeType datascope.ScopeType,
	_ string,
) (*datascope.FilterCondition, error) {
	switch scopeType {
	case datascope.ScopeOUSelf, datascope.ScopeOUSubtree:
		return &datascope.FilterCondition{
			Type:   scopeType,
			UserID: userID,
		}, nil
	default:
		return nil, nil
	}
}
