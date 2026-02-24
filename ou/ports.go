package ou

import (
	organizationmod "github.com/leeforge/plugins/ou/organization"
)

const ServiceKeyOUFactory = "adapter.ou.factory"

// ServiceFactory creates OU plugin services using app-owned adapters.
type ServiceFactory interface {
	NewOrganizationService() *organizationmod.Service
	Models() []any
}
