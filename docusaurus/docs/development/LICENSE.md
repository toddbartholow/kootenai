# License Rationale

This document explains why Kootenai Community Edition uses the **Apache License 2.0** rather than a copyleft license like AGPL v3 or GPL v3.

## Summary

| Aspect | Decision |
|--------|----------|
| **License** | Apache License 2.0 |
| **Type** | Permissive (not copyleft) |
| **SPDX Identifier** | `Apache-2.0` |

---

## Why Apache 2.0?

### 1. Maximum Adoption for Educational Tools

Kootenai targets **educational institutions**—universities, community colleges, training centers, and K-12 schools. These organizations often have:

- Complex procurement processes with legal review
- Blanket policies against copyleft licenses (especially AGPL)
- IT departments hesitant to deploy software with viral licensing clauses
- Partnerships with commercial vendors who avoid copyleft

A permissive license removes friction. Institutions can adopt, modify, and deploy Kootenai without legal anxiety.

### 2. Weak Threat Model for Copyleft

The traditional argument for AGPL is preventing "SaaS exploitation"—where a company modifies your code, hosts it as a service, and never contributes back. However, this threat is minimal for Kootenai:

- **Niche market**: Virtual lab orchestration for education isn't a lucrative SaaS opportunity
- **Complex infrastructure**: Running Kootenai requires Proxmox/CloudStack expertise, making drive-by commercialization unlikely
- **Network effects favor us**: Contributors benefit from staying in the ecosystem rather than forking
- **Enterprise features differentiate**: SSO, multi-tenancy, compliance logging, and support are why organizations pay—not because we lock them in with licensing

### 3. Community Growth Over Control

Open source projects thrive on contributions. Permissive licenses:

- Encourage corporate contributions (many companies prohibit contributing to copyleft projects)
- Attract developers who dislike navigating copyleft compliance
- Enable integration into proprietary educational platforms
- Allow forks that may eventually contribute improvements back

For a project in its growth phase, maximizing potential contributors matters more than preventing hypothetical exploitation.

### 4. Patent Protection (Why Not MIT?)

Apache 2.0 includes an **explicit patent grant**, protecting users from patent claims by contributors. MIT does not. For a project that may receive contributions from various organizations, this protection matters.

Apache 2.0 also requires **attribution** (preserving copyright notices), which:
- Ensures proper credit to contributors
- Maintains project visibility
- Creates a professional appearance for institutional adoption

---

## Why NOT AGPL v3?

We initially considered AGPL v3. Here's why we moved away:

### 1. "License Anxiety" in Education

Many educational institutions have IT policies that specifically exclude AGPL software. The network-use provision (Section 13) triggers concerns about:

- What constitutes "interacting with [the software] remotely through a computer network"?
- Does running Kootenai for students require releasing all modifications?
- How does this affect integrations with proprietary LMS systems?

These questions, even when the answers favor the institution, create adoption friction.

### 2. Contributor Hesitation

Some developers and organizations avoid contributing to AGPL projects due to:

- Corporate policies prohibiting AGPL contributions
- Concerns about license contamination in their other projects
- General discomfort with copyleft compliance obligations

### 3. Complexity for Integrations

Kootenai integrates with:
- Canvas LMS (proprietary)
- Proxmox VE (AGPL, ironically)
- Apache CloudStack (Apache 2.0)
- Various proprietary institutional systems

AGPL's network-use provision creates uncertainty about where the licensing boundaries lie in a complex integration environment.

### 4. Not Actually Needed

The AGPL's primary purpose—preventing SaaS exploitation—doesn't match our threat model. Our business protection comes from:

1. **Enterprise Edition features** (multi-tenancy, SSO, compliance)
2. **Support and SLAs** (what institutions actually pay for)
3. **Pre-built content** (lab template library)
4. **Brand and community** (being the canonical source)

License restrictions don't drive Enterprise Edition purchases—genuine enterprise needs do.

---

## Why NOT GPL v3?

GPL v3 (without the "Affero" network clause) was also considered:

- **Still copyleft**: Shares many of AGPL's adoption friction issues
- **No SaaS protection anyway**: GPL v3 doesn't require source disclosure for SaaS usage, so it provides no additional protection over Apache 2.0 for our use case
- **Worst of both worlds**: Copyleft friction without the network-use provision that justified the complexity

If we wanted copyleft, AGPL would be the logical choice. Since we don't, Apache 2.0 is cleaner.

---

## How This Fits the Open Core Model

Kootenai uses an **open core** business model:

| Edition | License | Features |
|---------|---------|----------|
| **Community Edition** | Apache 2.0 | Core platform, Proxmox integration, single-org |
| **Enterprise Edition** | Proprietary | Multi-tenancy, SSO, CloudStack, compliance, support |

This model succeeds because:

1. **CE is genuinely useful**: Not crippled or demo-ware
2. **EE solves real problems**: Features enterprises actually need
3. **No artificial restrictions**: Licensing doesn't gate features—capability does
4. **Growth feeds both**: CE adoption creates EE customers

The permissive license supports this by maximizing CE adoption, which grows the user base that eventually needs EE features.

---

## Comparison with Similar Projects

| Project | License | Model | Notes |
|---------|---------|-------|-------|
| **Kubernetes** | Apache 2.0 | Open source | Industry standard for orchestration |
| **Proxmox VE** | AGPL v3 | Open core | Infrastructure software, different market dynamics |
| **GitLab** | MIT (CE) | Open core | Permissive CE, proprietary EE |
| **Moodle** | GPL v3 | Open source | LMS, educational market, different era |
| **Canvas LMS** | AGPL v3 | Open core | LMS, Instructure commercial backing |
| **Docker** | Apache 2.0 | Open source | Developer tooling, broad adoption focus |

Most modern infrastructure projects targeting broad adoption choose permissive licenses.

---

## Practical Implications

### For Users

- **Use freely**: Deploy, modify, and integrate without licensing concerns
- **No disclosure requirement**: Modifications can remain private
- **Commercial use allowed**: Build products and services on top of Kootenai
- **Attribution required**: Keep copyright notices and the LICENSE file

### For Contributors

- **Your contributions**: Licensed under Apache 2.0
- **Patent grant**: You grant users a license to any patents covering your contributions
- **Credit preserved**: Your copyright notices remain in the code

### For Enterprises

- **Clear compliance**: Apache 2.0 is well-understood by legal teams
- **Integration friendly**: No concerns about viral licensing
- **Enterprise features available**: see the Enterprise Edition documentation

---

## References

- [Apache License 2.0 Full Text](https://www.apache.org/licenses/LICENSE-2.0)
- [Choose a License - Apache 2.0](https://choosealicense.com/licenses/apache-2.0/)
- [SPDX License List](https://spdx.org/licenses/Apache-2.0.html)
- [CE/EE Architecture](./ce-ee-setup.md)
