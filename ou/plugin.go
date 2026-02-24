package ou

import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"

	"github.com/leeforge/framework/logging"
	"github.com/leeforge/framework/plugin"

	organizationmod "github.com/leeforge/plugins/ou/organization"
	"github.com/leeforge/plugins/ou/shared"
)

const (
	serviceKeyOrganization  = "ou.organization.service"
	serviceKeyScopeResolver = "datascope.resolver.ou"
)

// OUPlugin implements the optional organization-unit plugin.
type OUPlugin struct {
	logger  logging.Logger
	factory ServiceFactory
	orgSvc  *organizationmod.Service
	orgHdlr *organizationmod.Handler
}

func (p *OUPlugin) Name() string           { return "ou" }
func (p *OUPlugin) Version() string        { return "1.0.0" }
func (p *OUPlugin) Dependencies() []string { return nil }

func (p *OUPlugin) Enable(_ context.Context, app *plugin.AppContext) error {
	if app == nil {
		return shared.ErrNilAppContext
	}
	if app.Services == nil {
		return shared.ErrNilServiceRegistry
	}
	p.logger = logging.FromZap(app.Logger)

	factory, err := plugin.Resolve[ServiceFactory](app.Services, ServiceKeyOUFactory)
	if err != nil {
		return fmt.Errorf("resolve ou service factory: %w", err)
	}
	p.factory = factory
	p.orgSvc = p.factory.NewOrganizationService()
	p.orgHdlr = organizationmod.NewHandler(p.orgSvc, p.logger)

	if err := app.Services.Register(serviceKeyOrganization, p.orgSvc); err != nil {
		return err
	}
	if err := app.Services.Register(serviceKeyScopeResolver, NewScopeResolver(p.orgSvc)); err != nil {
		return err
	}
	return nil
}

func (p *OUPlugin) RegisterRoutes(router chi.Router) {
	if router == nil || p.orgHdlr == nil {
		return
	}

	router.Route("/ou/organizations", func(r chi.Router) {
		r.Post("/", p.orgHdlr.CreateOrganization)
		r.Get("/tree", p.orgHdlr.GetOrganizationTree)
		r.Post("/{id}/members", p.orgHdlr.AddOrganizationMember)
	})
}

// RegisterModels declares OU-related Ent models for plugin runtime collection.
func (p *OUPlugin) RegisterModels() []any {
	if p.factory == nil {
		return nil
	}
	return p.factory.Models()
}
