package tenant

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/leeforge/framework/logging"
	"github.com/leeforge/framework/plugin"

	"github.com/leeforge/core"
	"github.com/leeforge/plugins/tenant/shared"
	tenantmod "github.com/leeforge/plugins/tenant/tenant"
)

// Re-export shared types so external consumers can import from this package.
type (
	TenantServiceAPI = shared.TenantServiceAPI
	TenantInfo       = shared.TenantInfo
	TenantEventData  = shared.TenantEventData
	MemberEventData  = shared.MemberEventData
)

// Re-export sentinel errors.
var (
	ErrTenantNotFound      = shared.ErrTenantNotFound
	ErrTenantCodeExists    = shared.ErrTenantCodeExists
	ErrInvalidTenant       = shared.ErrInvalidTenant
	ErrMemberExists        = shared.ErrMemberExists
	ErrMemberNotFound      = shared.ErrMemberNotFound
	ErrPlatformDomainOnly  = shared.ErrPlatformDomainOnly
	ErrParentTenantInvalid = shared.ErrParentTenantInvalid
)

// Re-export event constants.
const (
	EventTenantCreated       = shared.EventTenantCreated
	EventTenantUpdated       = shared.EventTenantUpdated
	EventTenantDeleted       = shared.EventTenantDeleted
	EventTenantMemberAdded   = shared.EventTenantMemberAdded
	EventTenantMemberRemoved = shared.EventTenantMemberRemoved
)

// TenantPlugin implements the framework plugin contracts.
type TenantPlugin struct {
	logger    logging.Logger
	factory   ServiceFactory
	domainSvc core.DomainWriter
	events    plugin.EventBus

	tenantSvc *tenantmod.Service
	tenantH   *tenantmod.Handler
}

func (p *TenantPlugin) Name() string           { return "tenant" }
func (p *TenantPlugin) Version() string        { return "1.0.0" }
func (p *TenantPlugin) Dependencies() []string { return nil }

func (p *TenantPlugin) Enable(ctx context.Context, app *plugin.AppContext) error {
	if app == nil || app.Services == nil {
		return fmt.Errorf("plugin app context is incomplete")
	}

	if app.Logger == nil {
		p.logger = logging.FromZap(zap.NewNop())
	} else {
		p.logger = logging.FromZap(app.Logger)
	}
	p.events = app.Events

	factory, err := plugin.Resolve[ServiceFactory](app.Services, ServiceKeyTenantFactory)
	if err != nil {
		return fmt.Errorf("resolve tenant service factory: %w", err)
	}
	p.factory = factory

	domainSvc, err := plugin.Resolve[core.DomainWriter](app.Services, "domain.service")
	if err != nil {
		return fmt.Errorf("resolve domain service: %w", err)
	}
	p.domainSvc = domainSvc

	p.tenantSvc = p.factory.NewTenantService(p.domainSvc, p.events, p.logger)
	p.tenantH = tenantmod.NewHandler(p.tenantSvc, p.logger)

	if err := app.Services.Register("tenant.service", p.exportedService()); err != nil {
		return fmt.Errorf("register tenant service: %w", err)
	}
	if err := app.Services.Register("domain.plugin.tenant", p); err != nil {
		return fmt.Errorf("register domain plugin: %w", err)
	}

	p.logger.Info("tenant plugin enabled")
	return nil
}

// Install seeds domain types and default tenants on first run.
func (p *TenantPlugin) Install(ctx context.Context, app *plugin.AppContext) error {
	return nil
}

// Disable performs cleanup on plugin shutdown.
func (p *TenantPlugin) Disable(ctx context.Context, app *plugin.AppContext) error {
	p.logger.Info("tenant plugin: shutting down")
	return nil
}

// SubscribeEvents registers event handlers.
func (p *TenantPlugin) SubscribeEvents(bus plugin.EventBus) {
	bus.Subscribe("user.deleted", func(ctx context.Context, e plugin.Event) error {
		return p.tenantSvc.OnUserDeleted(ctx, e.Data)
	})
}

func (p *TenantPlugin) RegisterRoutes(router chi.Router) {
	router.Route("/tenants", func(r chi.Router) {
		r.Get("/me", p.tenantH.ListMyTenants)
		r.Get("/", p.tenantH.ListTenants)
		r.Post("/", p.tenantH.CreateTenant)
		r.Get("/{id}", p.tenantH.GetTenant)
		r.Put("/{id}", p.tenantH.UpdateTenant)
		r.Delete("/{id}", p.tenantH.DeleteTenant)
		r.Post("/{id}/members", p.tenantH.AddMember)
		r.Get("/{id}/members", p.tenantH.ListMembers)
		r.Delete("/{id}/members/{userId}", p.tenantH.RemoveMember)
	})
}

func (p *TenantPlugin) HealthCheck(ctx context.Context) error {
	if p.tenantSvc == nil {
		return fmt.Errorf("tenant plugin: tenant service not initialized")
	}
	return p.tenantSvc.Ping(ctx)
}

func (p *TenantPlugin) PluginOptions() plugin.PluginOptions {
	return plugin.PluginOptions{
		Description: "Multi-tenant domain management plugin",
	}
}

func (p *TenantPlugin) RegisterModels() []any {
	if p.factory == nil {
		return nil
	}
	return p.factory.Models()
}

func (p *TenantPlugin) exportedService() TenantServiceAPI {
	return &tenantServiceAdapter{svc: p.tenantSvc}
}

type tenantServiceAdapter struct {
	svc *tenantmod.Service
}

func (a *tenantServiceAdapter) GetTenant(ctx context.Context, id uuid.UUID) (*TenantInfo, error) {
	dto, err := a.svc.GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}
	return &TenantInfo{
		ID:       dto.ID,
		Code:     dto.Code,
		Name:     dto.Name,
		Status:   dto.Status,
		DomainID: dto.DomainID,
	}, nil
}

func (a *tenantServiceAdapter) GetTenantByCode(ctx context.Context, code string) (*TenantInfo, error) {
	dto, err := a.svc.GetTenantByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return &TenantInfo{
		ID:       dto.ID,
		Code:     dto.Code,
		Name:     dto.Name,
		Status:   dto.Status,
		DomainID: dto.DomainID,
	}, nil
}

func (a *tenantServiceAdapter) IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	return a.svc.IsMember(ctx, tenantID, userID)
}

func (a *tenantServiceAdapter) GetDomainID(ctx context.Context, tenantCode string) (uuid.UUID, error) {
	return a.svc.GetDomainID(ctx, tenantCode)
}

func (p *TenantPlugin) TypeCode() string { return "tenant" }

func (p *TenantPlugin) ResolveDomain(ctx context.Context, r *http.Request) (*core.ResolvedDomain, bool, error) {
	if p.domainSvc == nil || r == nil {
		return nil, false, nil
	}
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		return nil, false, nil
	}
	resolved, err := p.domainSvc.ResolveDomain(ctx, "tenant", tenantID)
	if err != nil {
		return nil, false, err
	}
	return resolved, true, nil
}

func (p *TenantPlugin) ValidateMembership(ctx context.Context, domainID, subjectID uuid.UUID) (bool, error) {
	if p.domainSvc == nil {
		return false, fmt.Errorf("domain service is nil")
	}
	return p.domainSvc.CheckMembership(ctx, domainID, subjectID)
}

var (
	_ plugin.Plugin          = (*TenantPlugin)(nil)
	_ plugin.Installable     = (*TenantPlugin)(nil)
	_ plugin.Disableable     = (*TenantPlugin)(nil)
	_ plugin.RouteProvider   = (*TenantPlugin)(nil)
	_ plugin.EventSubscriber = (*TenantPlugin)(nil)
	_ plugin.HealthReporter  = (*TenantPlugin)(nil)
	_ plugin.Configurable    = (*TenantPlugin)(nil)
	_ plugin.ModelProvider   = (*TenantPlugin)(nil)
	_ TenantServiceAPI       = (*tenantServiceAdapter)(nil)
)
