package tenant

import (
	"github.com/leeforge/core"
	"github.com/leeforge/framework/logging"
	"github.com/leeforge/framework/plugin"
	"github.com/leeforge/plugins/tenant/shared"
	tenantmod "github.com/leeforge/plugins/tenant/tenant"
)

const ServiceKeyTenantFactory = "adapter.tenant.factory"

// ServiceFactory creates tenant plugin services using host-provided adapters.
type ServiceFactory interface {
	NewTenantService(
		domainSvc core.DomainWriter,
		events plugin.EventBus,
		logger logging.Logger,
	) *tenantmod.Service
	RoleSeeder() RoleSeeder
	UserLookup() UserLookup
	Models() []any
}

// Re-export interface types from shared so factory implementations import from this package.
type (
	RoleSeeder = shared.RoleSeeder
	UserLookup = shared.UserLookup
	UserInfo   = shared.UserInfo
)
