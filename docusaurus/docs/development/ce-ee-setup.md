# Community Edition / Enterprise Edition Setup

:::warning Archived repository -- no Enterprise Edition exists
This page describes a dual-edition business model that was planned and never
realised. No Enterprise Edition was ever built or distributed, there is no private
EE repository, and there is no vendor. The repository is archived, so there is
nobody to ask about any of it.

It is kept because the build tags, the `enterprise.Default` gate and the CE publish
pipeline it documents are all still in the source, and a fork needs to understand
them in order to remove or repurpose them. See [Enterprise Edition](../enterprise.md)
for what the gate actually enforces -- two features, not a product.
:::

This document describes the dual-edition architecture that was planned for Kootenai.

## Overview

Kootenai uses an **open core** business model:
- **Community Edition (CE)**: Public, open-source on GitHub under Apache 2.0
- **Enterprise Edition (EE)**: Private GitHub repo with proprietary license

---

## Decisions

| Aspect | Decision |
|--------|----------|
| Repository Structure | Separate repos (EE imports CE as dependency) |
| CE License | Apache 2.0 |
| EE Hosting | GitHub Private Repository |
| Model | Open Core (CE free, EE paid) |

---

## Feature Split

### Community Edition (Free, Apache 2.0)

| Category | Features |
|----------|----------|
| **Core** | Pod management, lab sessions, checkpoint tracking |
| **Infrastructure** | Proxmox integration only |
| **Organizations** | Single organization |
| **Authentication** | Local auth, basic LDAP |
| **LMS** | — (requires Enterprise) |
| **Monitoring** | Basic Wazuh integration |
| **Audit** | Basic logging |
| **Quotas** | Simple per-user limits |
| **Labs** | Self-created templates only |
| **Support** | Community (GitHub issues) |

### Enterprise Edition (Paid, Proprietary)

| Category | Features |
|----------|----------|
| **Core** | Everything in CE + |
| **Infrastructure** | + CloudStack integration, HA clustering |
| **Organizations** | Multi-tenant, multi-org |
| **Authentication** | + SAML/OIDC SSO, SCIM provisioning |
| **LMS** | LTI 1.3 integration (Canvas, Moodle, Blackboard) |
| **Monitoring** | Advanced Wazuh + custom analytics |
| **Audit** | SOC2-compliant audit trails |
| **Quotas** | Advanced resource quotas per org/user/group |
| **Labs** | + Pre-built lab template library |
| **Support** | SLAs, priority support, updates |

---

## Architecture

### Extension Point Pattern

CE defines interfaces with no-op implementations. EE imports CE and replaces the stubs via Go's `init()` mechanism.

```go
// CE: api/internal/enterprise/enterprise.go
package enterprise

type Features interface {
    GetMultiTenancy() MultiTenancyProvider
    GetSSOProvider() SSOProvider
    GetCloudStackProvider() CloudProvider
    IsEnabled(feature string) bool
    Edition() string
}

var Default Features = &communityFeatures{}
```

```go
// EE: internal/enterprise/init.go
package enterprise

import ce "github.com/toddbartholow/kootenai/api/internal/enterprise"

func init() {
    ce.Default = &enterpriseFeatures{
        multiTenancy: NewMultiTenancyProvider(),
        sso:          NewSSOProvider(),
        cloudstack:   NewCloudStackProvider(),
    }
}
```

### Feature Gating

Middleware checks enterprise features before allowing access:

```go
func RequireEnterprise(feature string) gin.HandlerFunc {
    return func(c *gin.Context) {
        if !enterprise.Default.IsEnabled(feature) {
            c.JSON(http.StatusForbidden, gin.H{
                "error": "This feature requires Kootenai Enterprise Edition",
                "feature": feature,
            })
            c.Abort()
            return
        }
        c.Next()
    }
}
```

---

## Repository Structure

### CE Repository (Public)

```
kootenai/
├── LICENSE                    # Apache 2.0
├── NOTICE                     # Copyright attribution
├── api/
│   └── internal/
│       └── enterprise/
│           ├── enterprise.go  # Interface definitions
│           └── community.go   # No-op implementations
└── ...
```

### EE Repository (Private)

```
kootenai-ee/
├── LICENSE                    # Proprietary
├── go.mod                     # Imports kootenai CE
├── cmd/
│   └── labctl-ee/
│       └── main.go            # EE entry point
├── internal/
│   ├── enterprise/
│   │   ├── init.go            # Replaces CE stubs
│   │   ├── multitenancy/
│   │   ├── sso/
│   │   ├── cloudstack/
│   │   └── audit/
│   └── licensing/
└── ...
```

---

## License Key System (EE)

Enterprise Edition validates license keys at startup:

```go
type License struct {
    CustomerID    string    `json:"customer_id"`
    CustomerName  string    `json:"customer_name"`
    Edition       string    `json:"edition"`
    Features      []string  `json:"features"`
    MaxUsers      int       `json:"max_users"`
    MaxOrgs       int       `json:"max_orgs"`
    ExpiresAt     time.Time `json:"expires_at"`
    Signature     string    `json:"signature"`
}
```

---

## Version Endpoint

The `/version` endpoint reports the edition:

```json
{
  "version": "1.0.0",
  "commit": "abc123",
  "build_time": "2024-01-01T00:00:00Z",
  "edition": "community"
}
```

---

## Why Apache 2.0?

- **Maximum adoption**: No license anxiety for educational institutions and enterprises
- **Patent protection**: Explicit patent grant protects contributors and users
- **Attribution required**: Ensures proper credit while remaining permissive
- **Permissive**: A fork may use, modify and redistribute the code without asking

---

## Getting Enterprise Edition

There is none to get. No Enterprise build was ever distributed, no private EE
repository exists, and there is no vendor behind the "open core" model described
above. The gate is inert: `enterprise.Default` is wired to an implementation whose
`IsEnabled` always returns `false`, and nothing reassigns it.

A fork that wants the two gated features -- Canvas LTI and the AI classroom -- has to
supply its own implementation. [Enterprise Edition](../enterprise.md) explains what
the gate covers and what it would take.
