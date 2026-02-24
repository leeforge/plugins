package tenant

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/leeforge/framework/logging"
	"github.com/leeforge/framework/plugin"

	"github.com/leeforge/core"
	"github.com/leeforge/plugins/tenant/shared"
	tenantmod "github.com/leeforge/plugins/tenant/tenant"
)

type mockDomainWriter struct {
	domains map[string]*core.ResolvedDomain
	members map[string]bool
}

func newMockDomainWriter() *mockDomainWriter {
	return &mockDomainWriter{
		domains: make(map[string]*core.ResolvedDomain),
		members: make(map[string]bool),
	}
}

func (m *mockDomainWriter) ResolveDomain(_ context.Context, typeCode, key string) (*core.ResolvedDomain, error) {
	if d, ok := m.domains[typeCode+":"+key]; ok {
		return d, nil
	}
	return nil, shared.ErrTenantNotFound
}

func (m *mockDomainWriter) ResolveDomainByID(_ context.Context, domainID uuid.UUID) (*core.ResolvedDomain, error) {
	for _, d := range m.domains {
		if d.DomainID == domainID {
			return d, nil
		}
	}
	return nil, shared.ErrTenantNotFound
}

func (m *mockDomainWriter) CheckMembership(_ context.Context, domainID, subjectID uuid.UUID) (bool, error) {
	return m.members[domainID.String()+":"+subjectID.String()], nil
}

func (m *mockDomainWriter) GetUserDefaultDomain(context.Context, uuid.UUID) (*core.ResolvedDomain, error) {
	return nil, nil
}

func (m *mockDomainWriter) GetDomainString(typeCode, key string) string { return typeCode + ":" + key }

func (m *mockDomainWriter) ListUserDomains(context.Context, uuid.UUID) ([]*core.UserDomainInfo, error) {
	return nil, nil
}

func (m *mockDomainWriter) EnsureDomain(_ context.Context, typeCode, key, displayName string) (*core.ResolvedDomain, error) {
	res := &core.ResolvedDomain{
		DomainID:    uuid.New(),
		TypeCode:    typeCode,
		Key:         key,
		DisplayName: displayName,
	}
	m.domains[typeCode+":"+key] = res
	return res, nil
}

func (m *mockDomainWriter) AddMembership(_ context.Context, domainID, subjectID uuid.UUID, _ string, _ bool) error {
	m.members[domainID.String()+":"+subjectID.String()] = true
	return nil
}

func (m *mockDomainWriter) RemoveMembership(_ context.Context, domainID, subjectID uuid.UUID) error {
	delete(m.members, domainID.String()+":"+subjectID.String())
	return nil
}

type noopSub struct{}

func (noopSub) Unsubscribe() {}

type noopEvents struct{}

func (noopEvents) Publish(context.Context, plugin.Event) error { return nil }
func (noopEvents) Subscribe(string, plugin.EventHandler) plugin.Subscription {
	return noopSub{}
}
func (noopEvents) Close() error { return nil }

// mockRoleSeeder is a no-op role seeder for plugin tests.
type mockRoleSeeder struct{}

func (mockRoleSeeder) SeedBaselineRoles(_ context.Context, _ uuid.UUID) error { return nil }

// mockUserLookup returns a stub user for plugin tests.
type mockUserLookup struct{}

func (mockUserLookup) GetUser(_ context.Context, userID uuid.UUID) (*shared.UserInfo, error) {
	return &shared.UserInfo{ID: userID, Username: "testuser", Email: "test@example.com", Status: "active"}, nil
}

type mockFactory struct{}

func (mockFactory) NewTenantService(
	domainSvc core.DomainWriter,
	events plugin.EventBus,
	logger logging.Logger,
) *tenantmod.Service {
	return tenantmod.NewService(nil, domainSvc, events, logger, mockRoleSeeder{}, mockUserLookup{})
}

func (mockFactory) RoleSeeder() shared.RoleSeeder { return mockRoleSeeder{} }
func (mockFactory) UserLookup() shared.UserLookup { return mockUserLookup{} }
func (mockFactory) Models() []any                 { return []any{"tenant"} }

func TestPlugin_Enable_Success(t *testing.T) {
	sr := plugin.NewServiceRegistry()
	require.NoError(t, sr.Register(ServiceKeyTenantFactory, mockFactory{}))
	require.NoError(t, sr.Register("domain.service", core.DomainWriter(newMockDomainWriter())))

	app := &plugin.AppContext{
		Logger:   zap.NewNop(),
		Services: sr,
		Events:   noopEvents{},
	}

	p := &TenantPlugin{}
	require.NoError(t, p.Enable(context.Background(), app))

	_, err := plugin.Resolve[TenantServiceAPI](sr, "tenant.service")
	require.NoError(t, err)
}

func TestPlugin_Enable_MissingFactory(t *testing.T) {
	sr := plugin.NewServiceRegistry()
	require.NoError(t, sr.Register("domain.service", core.DomainWriter(newMockDomainWriter())))

	p := &TenantPlugin{}
	err := p.Enable(context.Background(), &plugin.AppContext{
		Logger:   zap.NewNop(),
		Services: sr,
		Events:   noopEvents{},
	})
	require.Error(t, err)
}

func TestPlugin_HealthCheck_NotInitialized(t *testing.T) {
	p := &TenantPlugin{}
	require.Error(t, p.HealthCheck(context.Background()))
}
