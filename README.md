# Leeforge Plugins

[![GitHub](https://img.shields.io/badge/GitHub-JsonLee12138%2Fplugins-blue)](https://github.com/JsonLee12138/plugins)

Leeforge CMS 的可插拔业务扩展模块。每个插件是一个独立的 Go module，通过 framework 插件系统注册到宿主应用。

## Plugin Catalog

| Plugin | Module | Description |
|---|---|---|
| [tenant](./tenant/) | `github.com/leeforge/plugins/tenant` | Multi-tenant domain management, membership, domain resolution |
| [ou](./ou/) | `github.com/leeforge/plugins/ou` | Hierarchical organization unit, materialized-path tree, datascope |

## Architecture Overview

```
plugins/                        # https://github.com/JsonLee12138/plugins
├── tenant/                     # Tenant plugin (independent Go module)
│   ├── plugin.go               # Lifecycle: Enable / Disable / Install
│   ├── ports.go                # ServiceFactory interface
│   ├── shared/                 # Exported errors, events, public API types
│   ├── tenant/                 # Handler + Service + DTO
│   └── factory/                # Default Ent-backed factory
├── ou/                         # OU plugin (independent Go module)
│   ├── plugin.go               # Lifecycle: Enable
│   ├── ports.go                # ServiceFactory interface
│   ├── scope_resolver.go       # Datascope integration
│   ├── shared/                 # Exported errors
│   ├── organization/           # Handler + Service + DTO
│   └── factory/                # Default Ent-backed factory
└── README.md                   # This file
```

## Core Concepts

### Plugin Lifecycle

每个插件实现 `plugin.Plugin` 接口，由 framework runtime 按依赖顺序调用：

1. **Register** — 宿主调用 `runtime.Register(&plugin{})` 注册插件
2. **Enable** — Runtime 注入 `AppContext`（Logger、ServiceRegistry、EventBus），插件初始化内部服务
3. **RegisterRoutes** — 插件挂载 HTTP 路由到 Chi router
4. **Disable** (optional) — 优雅关闭

### ServiceFactory Pattern

插件不直接依赖宿主的 Ent client。通过 Ports & Adapters 模式解耦：

```
Host Application                          Plugin
┌──────────────────┐                ┌──────────────────┐
│ EntFactory        │ ──implements─▶│ ServiceFactory    │
│ (factory/ pkg)    │               │ (ports.go)        │
│                   │               │                   │
│ Uses: ent.Client  │               │ Creates: Service  │
└──────────────────┘                └──────────────────┘
```

- 每个插件在 `ports.go` 中定义 `ServiceFactory` 接口
- `factory/` 提供基于 `core/server/ent.Client` 的默认实现
- 宿主在 Enable 之前注册 factory 到 `ServiceRegistry`

### Service Registry Keys

| Key | Plugin | Type |
|---|---|---|
| `adapter.tenant.factory` | tenant | `tenant.ServiceFactory` |
| `tenant.service` | tenant | `shared.TenantServiceAPI` |
| `domain.plugin.tenant` | tenant | Domain resolution provider |
| `adapter.ou.factory` | ou | `ou.ServiceFactory` |
| `ou.organization.service` | ou | `*organization.Service` |
| `datascope.resolver.ou` | ou | `*ou.ScopeResolver` |

## Usage

### Install

```bash
go get github.com/leeforge/plugins/tenant@latest
go get github.com/leeforge/plugins/ou@latest
```

### Register Plugins in Host Application

```go
package bootstrap

import (
    frameplugin "github.com/leeforge/framework/plugin"
    frameworkruntime "github.com/leeforge/framework/runtime"

    tenant "github.com/leeforge/plugins/tenant"
    tenantfactory "github.com/leeforge/plugins/tenant/factory"

    ou "github.com/leeforge/plugins/ou"
    oufactory "github.com/leeforge/plugins/ou/factory"
)

func registerPlugins(
    rt *frameworkruntime.Runtime,
    services *frameplugin.ServiceRegistry,
    entClient *ent.Client,
) error {
    // --- Tenant ---
    services.Register(tenant.ServiceKeyTenantFactory, tenantfactory.NewEntFactory(entClient))
    if err := rt.Register(&tenant.TenantPlugin{}); err != nil {
        return err
    }

    // --- OU ---
    services.Register(ou.ServiceKeyOUFactory, oufactory.NewEntFactory(entClient))
    if err := rt.Register(&ou.OUPlugin{}); err != nil {
        return err
    }

    return nil
}
```

## Creating a New Plugin

1. 在仓库根目录创建子目录: `<name>/`
2. 初始化模块: `go mod init github.com/leeforge/plugins/<name>`
3. 添加 `core` 和 `framework` 依赖
4. 按照标准结构实现:

```
<name>/
├── go.mod
├── doc.go               # Package doc
├── plugin.go            # Implement plugin.Plugin
├── ports.go             # Define ServiceFactory interface
├── shared/
│   └── errors.go        # Error sentinels
├── <domain>/
│   ├── handler.go       # HTTP handlers
│   ├── service.go       # Business logic
│   └── dto.go           # Request/Response types
└── factory/
    └── ent_factory.go   # Default factory implementation
```

5. 在宿主应用的 plugin registrar 中注册

## Dependencies

All plugins depend on:

| Module | Purpose |
|---|---|
| `github.com/leeforge/core` | Ent schemas, context helpers, httplog, datascope |
| `github.com/leeforge/framework` | Plugin system, logging, HTTP responder |
| `github.com/go-chi/chi/v5` | Router for route registration |
| `github.com/google/uuid` | UUID types |
