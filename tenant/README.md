# Tenant Plugin

Multi-tenant domain management plugin for Leeforge CMS. Provides tenant CRUD, membership management, domain resolution, and event-driven lifecycle hooks.

## Quick Start

```go
import (
    tenant "github.com/leeforge/plugins/tenant"
    tenantfactory "github.com/leeforge/plugins/tenant/factory"
)

// 1. Register factory (before plugin Enable)
services.Register(tenant.ServiceKeyTenantFactory, tenantfactory.NewEntFactory(entClient))

// 2. Register plugin
runtime.Register(&tenant.TenantPlugin{})
```

## Plugin Metadata

| Field | Value |
|---|---|
| Name | `tenant` |
| Version | `1.0.0` |
| Dependencies | None |
| Module | `github.com/leeforge/plugins/tenant` |

## Architecture

```
server/plugins/tenant/
├── plugin.go              # Plugin lifecycle (Enable/Disable/Install)
├── ports.go               # ServiceFactory interface
├── shared/
│   ├── errors.go          # Exported error sentinels
│   ├── events.go          # Event constants and payloads
│   ├── exported.go        # Re-exported public types
│   └── ports.go           # RoleSeeder / UserLookup interfaces
├── tenant/
│   ├── handler.go         # HTTP handlers
│   ├── service.go         # Business logic
│   └── dto.go             # Request/Response DTOs
└── factory/
    └── ent_factory.go     # Default Ent-backed factory
```

## ServiceFactory

Host application must provide a `ServiceFactory` implementation registered under `ServiceKeyTenantFactory`:

```go
const ServiceKeyTenantFactory = "adapter.tenant.factory"

type ServiceFactory interface {
    NewTenantService(domainSvc core.DomainWriter, events plugin.EventBus, logger logging.Logger) *tenantmod.Service
    RoleSeeder() RoleSeeder
    UserLookup() UserLookup
    Models() []any
}
```

The built-in `factory.EntFactory` provides a default implementation backed by `core/server/ent.Client`.

## HTTP Routes

All routes are registered under `/tenants`:

| Method | Path | Handler | Description |
|---|---|---|---|
| GET | `/tenants/me` | `ListMyTenants` | List tenants for current user |
| GET | `/tenants/` | `ListTenants` | List all tenants (platform domain only) |
| POST | `/tenants/` | `CreateTenant` | Create new tenant |
| GET | `/tenants/{id}` | `GetTenant` | Get tenant by ID |
| PUT | `/tenants/{id}` | `UpdateTenant` | Update tenant |
| DELETE | `/tenants/{id}` | `DeleteTenant` | Soft-delete tenant |
| POST | `/tenants/{id}/members` | `AddMember` | Add member to tenant |
| GET | `/tenants/{id}/members` | `ListMembers` | List tenant members (paginated) |
| DELETE | `/tenants/{id}/members/{userId}` | `RemoveMember` | Remove member |

## Events

### Published

| Event | Constant | Payload |
|---|---|---|
| `tenant.created` | `EventTenantCreated` | `TenantEventData` |
| `tenant.updated` | `EventTenantUpdated` | `TenantEventData` |
| `tenant.deleted` | `EventTenantDeleted` | `TenantEventData` |
| `tenant.member.added` | `EventTenantMemberAdded` | `MemberEventData` |
| `tenant.member.removed` | `EventTenantMemberRemoved` | `MemberEventData` |

### Subscribed

| Event | Handler |
|---|---|
| `user.deleted` | Cleans up all memberships for deleted user |

### Event Payloads

```go
type TenantEventData struct {
    TenantID   uuid.UUID `json:"tenantId"`
    TenantCode string    `json:"tenantCode"`
    DomainID   uuid.UUID `json:"domainId"`
    ActorID    uuid.UUID `json:"actorId"`
}

type MemberEventData struct {
    TenantID uuid.UUID `json:"tenantId"`
    UserID   uuid.UUID `json:"userId"`
    Role     string    `json:"role"`
    ActorID  uuid.UUID `json:"actorId"`
}
```

## Service Keys

| Key | Type | Description |
|---|---|---|
| `adapter.tenant.factory` | `ServiceFactory` | Resolved during Enable |
| `tenant.service` | `TenantServiceAPI` | Public tenant query API |
| `domain.plugin.tenant` | `TenantPlugin` | Domain resolution provider |

## Public API (Cross-Plugin)

Other plugins can resolve `TenantServiceAPI` from the service registry:

```go
type TenantServiceAPI interface {
    GetTenant(ctx context.Context, id uuid.UUID) (*TenantInfo, error)
    GetTenantByCode(ctx context.Context, code string) (*TenantInfo, error)
    IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error)
    GetDomainID(ctx context.Context, tenantCode string) (uuid.UUID, error)
}
```

## Domain Resolution

The tenant plugin implements the domain plugin pattern:

- `ResolveDomain()` — Resolves tenant domain from `X-Tenant-ID` header
- `ValidateMembership()` — Checks if subject is member of domain
- `TypeCode()` — Returns `"tenant"`

## Error Sentinels

```go
import "github.com/leeforge/plugins/tenant/shared"

shared.ErrTenantNotFound       // Tenant not found
shared.ErrTenantCodeExists     // Tenant code already exists
shared.ErrInvalidTenant        // Invalid tenant data
shared.ErrMemberExists         // User is already a member
shared.ErrMemberNotFound       // Membership not found
shared.ErrPlatformDomainOnly   // Operation requires platform domain
shared.ErrParentTenantInvalid  // Invalid parent tenant
```

## Framework Interfaces

`TenantPlugin` implements:

| Interface | Purpose |
|---|---|
| `plugin.Plugin` | Core lifecycle (Enable) |
| `plugin.Installable` | First-run setup |
| `plugin.Disableable` | Shutdown cleanup |
| `plugin.RouteProvider` | HTTP route registration |
| `plugin.EventSubscriber` | Domain event subscriptions |
| `plugin.HealthReporter` | Health check via `Ping()` |
| `plugin.Configurable` | Plugin options metadata |
| `plugin.ModelProvider` | Ent model export |

## Ent Models

The plugin requires these Ent schemas (provided by `core/server/ent`):

- `Tenant` — Tenant entity with code, name, domain reference
- `TenantUser` — Tenant membership (user-tenant-role mapping)
