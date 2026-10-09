# Platform Comparison

## Proxmox vs CloudStack for Labs

| Feature | Proxmox VE | CloudStack |
|---------|-----------|------------|
| **Snapshot Speed** | 1-5 seconds | 30-120 seconds |
| **Memory Snapshots** | Yes (live state) | Limited |
| **Clone Type** | Linked + Full | Full only |
| **Multi-tenancy** | Basic | Enterprise-grade |
| **API Complexity** | Simple REST | Signed requests |
| **Best For** | Fast-reset labs | Cloud training |

## When to Use Proxmox

- Security labs requiring frequent resets
- Incident response scenarios
- Network simulations with breakpoints
- Red team / blue team exercises
- Any lab where students need to reset within seconds

## When to Use CloudStack

- Cloud infrastructure training
- Multi-tenant concepts
- API-driven provisioning labs
- Production-like IaaS scenarios
- Teaching cloud architecture

## Performance Benchmarks

| Operation | Proxmox | CloudStack |
|-----------|---------|------------|
| Create Snapshot | 1-3s | 30-60s |
| Revert Snapshot | 2-5s | 60-120s |
| Clone VM (linked) | 5-10s | N/A |
| Clone VM (full) | 30-60s | 60-120s |
| Start VM | 3-5s | 10-20s |

## Hybrid Approach Benefits

By supporting both platforms, the lab system can:

1. **Optimize for use case**: Security labs get fast snapshots, cloud labs get realistic IaaS
2. **Teach both paradigms**: Students learn hypervisor management AND cloud APIs
3. **Balance resources**: Distribute load across platforms
4. **Provide flexibility**: Instructors choose the best platform per lab
