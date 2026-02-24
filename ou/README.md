# OU Plugin

Organization Unit plugin for Leeforge CMS. Provides hierarchical organization management with materialized-path tree structure, membership tracking, and datascope integration.

## Quick Start

```go
import (
    ou "github.com/leeforge/plugins/ou"
    oufactory "github.com/leeforge/plugins/ou/factory"
)

// 1. Register factory (before plugin Enable)
services.Register(ou.ServiceKeyOUFactory, oufactory.NewEntFactory(entClient))

// 2. Register plugin
runtime.Register(&ou.OUPlugin{})
```

## Plugin Metadata

| Field | Value |
|---|---|
| Name | `ou` |
| Version | `1.0.0` |
| Dependencies | None |
| Module | `github.com/leeforge/plugins/ou` |

## Architecture

```
server/plugins/ou/
├── plugin.go              # Plugin lifecycle (Enable)
├── ports.go               # ServiceFactory interface
├── scope_resolver.go      # Datascope resolver (OU_SELF / OU_SUBTREE)
├── doc.go                 # Package documentation
├── shared/
│   └── errors.go          # Shared error sentinels
├── organization/
│   ├── handler.go         # HTTP handlers
│   ├── service.go         # Business logic (tree, members, subtree)
│   └── dto.go             # Request/Response DTOs
└── factory/
    └── ent_factory.go     # Default Ent-backed factory
```

## ServiceFactory

Host application must provide a `ServiceFactory` registered under `ServiceKeyOUFactory`:

```go
const ServiceKeyOUFactory = "adapter.ou.factory"

type ServiceFactory interface {
    NewOrganizationService() *organizationmod.Service
    Models() []any
}
```

The built-in `factory.EntFactory` provides a default implementation backed by `core/server/ent.Client`.

## HTTP Routes

All routes are registered under `/ou/organizations`:

| Method | Path | Handler | Description |
|---|---|---|---|
| POST | `/ou/organizations` | `CreateOrganization` | Create organization (supports parent for hierarchy) |
| GET | `/ou/organizations/tree` | `GetOrganizationTree` | Get full organization tree for current domain |
| POST | `/ou/organizations/{id}/members` | `AddOrganizationMember` | Add user as organization member |

## Request/Response DTOs

### CreateOrganizationRequest

```json
{
  "code": "engineering",
  "name": "Engineering Department",
  "parentId": "uuid-of-parent (optional)"
}
```

### AddOrganizationMemberRequest

```json
{
  "userId": "uuid-of-user",
  "isPrimary": true
}
```

### OrganizationResponse

```json
{
  "id": "uuid",
  "domainId": "uuid",
  "parentId": "uuid or null",
  "code": "engineering",
  "name": "Engineering Department",
  "path": "company/engineering"
}
```

### OrganizationTreeNode

```json
{
  "id": "uuid",
  "domainId": "uuid",
  "parentId": null,
  "code": "company",
  "name": "Company",
  "path": "company",
  "children": [
    {
      "id": "uuid",
      "domainId": "uuid",
      "parentId": "uuid",
      "code": "engineering",
      "name": "Engineering",
      "path": "company/engineering",
      "children": []
    }
  ]
}
```

## Datascope Integration

The plugin registers a `ScopeResolver` for OU-based data filtering:

| Scope Type | Constant | Description |
|---|---|---|
| `OU_SELF` | `datascope.ScopeOUSelf` | Data within user's primary organization |
| `OU_SUBTREE` | `datascope.ScopeOUSubtree` | Data within user's organization and all children |

The resolver relies on these service methods:

- `GetPrimaryOrganizationID(ctx, domainID, userID)` — Find user's primary org
- `ListOrganizationUserIDs(ctx, domainID, orgID)` — All users in one org
- `ListSubtreeUserIDs(ctx, domainID, orgID)` — All users in org subtree

## Service Keys

| Key | Type | Description |
|---|---|---|
| `adapter.ou.factory` | `ServiceFactory` | Resolved during Enable |
| `ou.organization.service` | `*organization.Service` | Organization CRUD + queries |
| `datascope.resolver.ou` | `*ScopeResolver` | OU scope resolver |

## Error Sentinels

### Organization errors (`organization` package)

```go
organization.ErrDomainContextMissing  // Missing domain context in request
organization.ErrInvalidDomainID       // Invalid domain ID format
organization.ErrOrganizationNotFound  // Organization not found in domain
organization.ErrMemberAlreadyExists   // User already member of organization
```

### Plugin errors (`shared` package)

```go
shared.ErrNilAppContext       // Plugin Enable called with nil AppContext
shared.ErrNilServiceRegistry  // Plugin Enable called with nil ServiceRegistry
```

## Materialized Path

Organizations use materialized-path hierarchy. The `path` field stores the full ancestry chain separated by `/`:

```
company
company/engineering
company/engineering/backend
company/engineering/frontend
company/sales
```

This enables efficient subtree queries using `PathHasPrefix`.

## Ent Models

The plugin requires these Ent schemas (provided by `core/server/ent`):

- `Organization` — Organization entity with code, name, path, domain, parent reference
- `OrganizationMember` — Membership (user-organization-domain mapping with `isPrimary` flag)
