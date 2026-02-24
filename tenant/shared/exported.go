package shared

import (
	"context"

	"github.com/google/uuid"
)

// TenantServiceAPI is the public interface exposed to other plugins.
// Consumers resolve it via: plugin.Resolve[TenantServiceAPI](services, "tenant.service")
type TenantServiceAPI interface {
	GetTenant(ctx context.Context, id uuid.UUID) (*TenantInfo, error)
	GetTenantByCode(ctx context.Context, code string) (*TenantInfo, error)
	IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error)
	GetDomainID(ctx context.Context, tenantCode string) (uuid.UUID, error)
}

// TenantInfo is the cross-plugin tenant summary.
type TenantInfo struct {
	ID       uuid.UUID `json:"id"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	DomainID uuid.UUID `json:"domainId"`
}
