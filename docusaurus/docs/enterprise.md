---
id: enterprise
title: Enterprise Edition
sidebar_position: 100
---

# Enterprise Edition

Kootenai Enterprise Edition provides additional features for organizations requiring multi-tenancy, compliance, and advanced integrations.

## Feature Comparison

| Feature | Community | Enterprise |
|---------|:---------:|:----------:|
| **Infrastructure** | | |
| Proxmox Integration | ✅ | ✅ |
| CloudStack Integration | ❌ | ✅ |
| High Availability Clustering | ❌ | ✅ |
| **Organizations** | | |
| Single Organization | ✅ | ✅ |
| Multi-Tenancy | ❌ | ✅ |
| Advanced Resource Quotas | ❌ | ✅ |
| **Authentication** | | |
| Local Authentication | ✅ | ✅ |
| Basic LDAP | ✅ | ✅ |
| SAML SSO | ❌ | ✅ |
| OIDC SSO | ❌ | ✅ |
| SCIM User Provisioning | ❌ | ✅ |
| **LMS Integration (LTI 1.3)** | | |
| Canvas LMS | ❌ | ✅ |
| Moodle | ❌ | ✅ |
| Blackboard | ❌ | ✅ |
| **Monitoring & Compliance** | | |
| Basic Audit Logging | ✅ | ✅ |
| SOC2-Compliant Audit Trails | ❌ | ✅ |
| Audit Log Export | ❌ | ✅ |
| Custom Analytics Dashboard | ❌ | ✅ |
| **Labs** | | |
| Self-Created Templates | ✅ | ✅ |
| Pre-Built Lab Library | ❌ | ✅ |
| **Simulation** | | |
| Test Student Simulation | ✅ | ✅ |
| AI Classroom Simulation | ❌ | ✅ |
| **Support** | | |
| Community Support (GitHub) | ✅ | ✅ |
| Priority Email Support | ❌ | ✅ |
| SLA Guarantees | ❌ | ✅ |
| Professional Services | ❌ | ✅ |

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

In addition to Proxmox, Enterprise Edition supports Apache CloudStack for cloud infrastructure training:

- **Multi-Zone Support**: Deploy labs across multiple availability zones
- **Network Isolation**: Advanced VPC and network ACL configurations
- **Storage Integration**: Support for CloudStack storage offerings

### Compliance Features

Meet your compliance requirements with advanced audit capabilities:

- **SOC2-Compliant Logging**: Comprehensive audit trails for all user and system actions
- **Audit Export**: Export logs in standard formats for compliance audits
- **Retention Policies**: Configurable log retention to meet regulatory requirements

### AI Classroom Simulation

Simulate an entire class of AI students with distinct personalities:

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

To ask about Enterprise Edition, open an issue on the project repository.

## Upgrading from Community to Enterprise

There is no Enterprise build to upgrade to, so the steps below describe the
mechanism rather than a route anyone can take today. They are kept because they
document how the edition switch works for anyone forking the extension points.

1. **Obtain a license key** — no issuer exists; an Enterprise build would define one
2. **Install EE**: Replace the CE binary with the EE binary
3. **Configure License**: Add your license key to the configuration
4. **Restart**: Restart the API server to activate enterprise features

Your existing data, users, and configurations are fully preserved during the upgrade.
