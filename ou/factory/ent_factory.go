package factory

import (
	"github.com/leeforge/core/server/ent"
	organizationmod "github.com/leeforge/plugins/ou/organization"
)

// EntFactory implements ou.ServiceFactory using a core Ent client.
type EntFactory struct {
	client *ent.Client
}

// NewEntFactory creates a factory backed by the given Ent client.
func NewEntFactory(client *ent.Client) *EntFactory {
	return &EntFactory{client: client}
}

func (f *EntFactory) NewOrganizationService() *organizationmod.Service {
	return organizationmod.NewService(f.client)
}

func (f *EntFactory) Models() []any {
	return []any{"organization", "organization_member"}
}
