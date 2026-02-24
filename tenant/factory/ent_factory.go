package factory

import (
	"context"

	"github.com/google/uuid"

	"github.com/leeforge/framework/logging"
	"github.com/leeforge/framework/plugin"

	"github.com/leeforge/core"
	coreent "github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/role"
	"github.com/leeforge/core/server/ent/user"

	tenantplugin "github.com/leeforge/plugins/tenant"
	"github.com/leeforge/plugins/tenant/shared"
	tenantmod "github.com/leeforge/plugins/tenant/tenant"
)

// EntFactory adapts ent-backed dependencies to tenant plugin services.
type EntFactory struct {
	client *coreent.Client
}

func NewEntFactory(client *coreent.Client) *EntFactory {
	return &EntFactory{client: client}
}

func (f *EntFactory) NewTenantService(
	domainSvc core.DomainWriter,
	events plugin.EventBus,
	logger logging.Logger,
) *tenantmod.Service {
	return tenantmod.NewService(f.client, domainSvc, events, logger, f.RoleSeeder(), f.UserLookup())
}

func (f *EntFactory) RoleSeeder() shared.RoleSeeder {
	return &entRoleSeeder{client: f.client}
}

func (f *EntFactory) UserLookup() shared.UserLookup {
	return &entUserLookup{client: f.client}
}

func (f *EntFactory) Models() []any {
	return []any{"tenant", "tenant_user"}
}

var _ tenantplugin.ServiceFactory = (*EntFactory)(nil)

// --- RoleSeeder ---

type entRoleSeeder struct {
	client *coreent.Client
}

// SeedBaselineRoles creates the baseline owner and member roles for a new domain.
// It is idempotent: existing roles are skipped if they already exist.
func (s *entRoleSeeder) SeedBaselineRoles(ctx context.Context, domainID uuid.UUID) error {
	type roleSpec struct {
		name string
		code string
	}
	baseline := []roleSpec{
		{name: "Owner", code: "owner"},
		{name: "Member", code: "member"},
	}
	for _, spec := range baseline {
		exists, err := s.client.Role.Query().
			Where(
				role.OwnerDomainID(domainID),
				role.Code(spec.code),
			).
			Exist(ctx)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := s.client.Role.Create().
			SetOwnerDomainID(domainID).
			SetName(spec.name).
			SetCode(spec.code).
			SetIsSystem(true).
			SetPermissions([]string{}).
			Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// --- UserLookup ---

type entUserLookup struct {
	client *coreent.Client
}

// GetUser fetches a user by ID and maps it to shared.UserInfo.
func (l *entUserLookup) GetUser(ctx context.Context, userID uuid.UUID) (*shared.UserInfo, error) {
	u, err := l.client.User.Query().
		Where(user.ID(userID)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return &shared.UserInfo{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		Nickname: u.Nickname,
		Status:   u.Status.String(),
	}, nil
}
