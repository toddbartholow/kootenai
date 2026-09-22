---
id: enterprise
title: Enterprise Edition
sidebar_position: 100
---

# Enterprise Edition

No Enterprise Edition is distributed from this repository, and none ever will be — the
project is archived. What exists here is the **gate** a hypothetical Enterprise build
would open: `enterprise.Default` (`api/internal/enterprise/enterprise.go:144`) is
assigned `&communityFeatures{}`, whose `IsEnabled` returns `false` unconditionally
(`api/internal/enterprise/community.go:21-23`), and nothing in this repository ever
reassigns it.

This page describes what that gate actually does, because the answer is much narrower
than it used to claim.

## What is actually gated

`middleware.RequireEnterprise` has exactly three call sites, and all three deny in every
build produced from this repository:

| Feature | Constant | Gated at | Effect in this build |
|---------|----------|----------|----------------------|
| LTI 1.3 / Canvas LMS | `lti` | `api/internal/server/canvas_manager.go:150` (`/lti/*`), `:165` (`/api/v1/lti/*`) | Every LTI route returns **403** before its handler runs. The LTI service is never constructed either — `server.go:768-772` logs one `Info` line and skips it, even when Canvas is configured correctly. |
| AI classroom simulation | `ai_classroom` | `api/internal/server/classroom_handlers.go:51` | Route group returns **403**. |

Nothing else is gated. See "Known gaps" in the repository `README.md` for the two-line
change that opens the LTI gate in a fork.

## What is declared but never checked

The remaining nine `enterprise.Feature*` constants are defined in
`api/internal/enterprise/enterprise.go` and **never passed to `IsEnabled` or
`RequireEnterprise` anywhere in the module**:

`multi_tenancy`, `sso`, `scim`, `cloudstack`, `advanced_audit`, `advanced_quotas`,
`custom_analytics`, `ha_cluster`, `lab_library`

They record an intention that was never wired up. Treat them as design notes, not as
behaviour.

**CloudStack in particular is not gated in any way.** The client is constructed
unconditionally at `api/cmd/labctl/main.go:369` and handed to the orchestrator at `:419`,
in every build. An earlier version of this page claimed CloudStack was Enterprise-only;
that was simply wrong. What is true is that CloudStack is lightly exercised compared to
Proxmox — see the `README.md` gap list.

## A separate mechanism: per-organization feature flags

Do not confuse the edition gate above with the per-organization feature flags, which are
real, wired up, and unrelated. Those live in the `feature_flags` and
`organization_features` tables (seeded by
`api/internal/database/migrations/012_multi_tenancy.sql`), are served from
`GET /api/v1/organizations/{orgID}/features`, and drive `hasFeature('teams')` and friends
in the Vue app. Their identifiers are a different namespace entirely — `teams`,
`labs.custom`, `sso.saml`, `multi_org`, `api.webhooks` and so on — and the seed data
assigns each to `community`, `professional` or `enterprise` tiers that no running code
enforces.

## Design notes for the extension points

Everything below describes the shape the Enterprise extension points were designed
around. With two exceptions it is **aspirational** — multi-tenancy, SSO/SCIM, compliance
logging and advanced analytics have no implementation behind them in this repository, and
the `Get*` methods on the `Features` interface all return `nil` in Community
(`api/internal/enterprise/community.go`). The exceptions are AI classroom simulation, which
is implemented and gated, and CloudStack, which is implemented and not gated at all. The
section is kept so a fork adopting the `enterprise.Features` interface knows what the
interface was for.

## Enterprise Features

### Multi-Tenancy

Enterprise Edition supports full multi-tenancy with complete isolation between organizations:

- **Separate Organizations**: Each organization has its own users, labs, pods, and settings
- **Resource Quotas**: Set limits per organization on users, pods, CPU, memory, and storage
- **Delegated Administration**: Organization admins manage their own users without system-wide access

### SSO Integration

Integrate with your existing identity provider:

- **SAML 2.0**: Connect to Okta, Azure AD, OneLogin, PingIdentity, and other SAML providers
- **OIDC**: Support for OpenID Connect providers including Google Workspace and Auth0
- **SCIM 2.0**: Automatic user provisioning and deprovisioning from your identity provider

### CloudStack Integration

**Not an Enterprise feature — CloudStack works in every build.** The client is constructed
unconditionally (`api/cmd/labctl/main.go:369`) and passed to the orchestrator (`:419`); the
`enterprise.FeatureCloudStack` constant exists but is never checked. Configure
`CLOUDSTACK_*` and it runs.

Apache CloudStack is the target for cloud-infrastructure coursework, alongside Proxmox for
snapshot-heavy lab work:

- **Multi-Zone Support**: Deploy labs across multiple availability zones
- **Network Isolation**: VPC and network ACL configuration
- **Storage Integration**: CloudStack storage offerings

The caveat is coverage, not licensing: nearly all real-world mileage on this platform is on
Proxmox. See "Known gaps" in the repository `README.md`.

### Compliance Features

Meet your compliance requirements with advanced audit capabilities:

- **SOC2-Compliant Logging**: Comprehensive audit trails for all user and system actions
- **Audit Export**: Export logs in standard formats for compliance audits
- **Retention Policies**: Configurable log retention to meet regulatory requirements

### AI Classroom Simulation

Implemented, and genuinely gated — `api/internal/server/classroom_handlers.go:51` denies the
whole route group in this build. Simulates a class of AI students with distinct
personalities:

- **3 Personality Types**: High Performer, Struggling Student, Industry Professional — each with unique traits, skills, and behavioral patterns
- **LLM-Generated Content**: Anthropic Claude generates persona-appropriate assignment submissions, discussion posts, and quiz answers
- **Canvas Integration**: Auto-creates users, enrolls in courses, submits assignments, posts discussions, and takes quizzes
- **VM Lab Execution**: Students complete labs on real VMs via the labtest simulator
- **Behavior Engine**: Models energy, stress, quality modifiers, and personality-driven timing
- **Live Monitoring**: Real-time activity feed with per-student progress tracking

See [AI Classroom Simulation Guide](guides/classroom-simulation.md) for details.

### Advanced Analytics

Gain insights into platform usage:

- **Custom Dashboards**: Organization-specific analytics dashboards
- **Usage Reports**: Detailed reports on lab usage, completion rates, and resource consumption
- **Export Capabilities**: Export analytics data for external analysis

## Checking Your Edition

You can check which edition you're running via the API:

```bash
curl http://localhost:8080/version
```

Response:
```json
{
  "version": "1.0.0",
  "commit": "abc123",
  "build_time": "2024-01-01T00:00:00Z",
  "go_version": "go1.24",
  "os": "linux",
  "arch": "amd64",
  "edition": "community"
}
```

## Enterprise Feature Gating

When you attempt to use an enterprise feature in Community Edition, the API returns a helpful error:

```json
{
  "error": "This feature requires Kootenai Enterprise Edition",
  "feature": "multi_tenancy",
  "edition": "community"
}
```

The response carries no upgrade link. Enterprise Edition is not distributed from
this repository, so there is no destination to point at.

## Getting Enterprise Edition

Enterprise Edition is not distributed from this repository. Community Edition
ships the extension points described above with no-op implementations; an
Enterprise build replaces them at `init()`.

There is nobody to ask: the repository is archived and issues are closed.

## Upgrading from Community to Enterprise

There is no Enterprise build to upgrade to, so the steps below describe the
mechanism rather than a route anyone can take today. They are kept because they
document how the edition switch works for anyone forking the extension points.

1. **Obtain a license key** — no issuer exists; an Enterprise build would define one
2. **Install EE**: Replace the CE binary with the EE binary
3. **Configure License**: Add your license key to the configuration
4. **Restart**: Restart the API server to activate enterprise features

Your existing data, users, and configurations are fully preserved during the upgrade.
