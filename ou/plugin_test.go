//go:build integration
// +build integration

package ou

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/leeforge/framework/plugin"

	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/enttest"
	organizationmod "github.com/leeforge/plugins/ou/organization"

	_ "github.com/mattn/go-sqlite3"
)

type mockOUFactory struct {
	client *ent.Client
}

func (m *mockOUFactory) NewOrganizationService() *organizationmod.Service {
	return organizationmod.NewService(m.client)
}

func (m *mockOUFactory) Models() []any {
	return []any{"organization", "organization_member"}
}

func TestOUPlugin_Enable_RegistersServices(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ou_plugin_enable?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	services := plugin.NewServiceRegistry()
	services.MustRegister(ServiceKeyOUFactory, &mockOUFactory{client: client})

	app := &plugin.AppContext{
		Logger:   zap.NewNop(),
		Services: services,
	}

	p := &OUPlugin{}
	err := p.Enable(context.Background(), app)
	require.NoError(t, err)
	require.True(t, app.Services.Has("ou.organization.service"))
	require.True(t, app.Services.Has("datascope.resolver.ou"))
}

func TestOUPlugin_Enable_MissingServiceFactory(t *testing.T) {
	app := &plugin.AppContext{
		Logger:   zap.NewNop(),
		Services: plugin.NewServiceRegistry(),
	}

	err := (&OUPlugin{}).Enable(context.Background(), app)
	require.Error(t, err)
}

func TestOUPlugin_RegisterModels_IncludesOrganizationEntities(t *testing.T) {
	client := enttest.Open(t, dialect.SQLite, "file:ou_plugin_models?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	p := &OUPlugin{factory: &mockOUFactory{client: client}}
	models := p.RegisterModels()
	require.GreaterOrEqual(t, len(models), 2)
}
