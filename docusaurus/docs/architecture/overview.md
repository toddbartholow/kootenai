# Architecture Overview

## Platform Selection

The platform automatically selects the appropriate virtualization backend based on lab requirements:

```mermaid
flowchart TD
    A[Lab Request] --> B{Platform Specified?}
    B -->|proxmox| C[Proxmox]
    B -->|cloudstack| D[CloudStack]
    B -->|any| E{Lab Type?}
    E -->|Security/Fast Reset| C
    E -->|Cloud Training| D
    E -->|Other| F[Default Platform]
```

## Components

### Go Orchestration API

The central service managing:

- Lab template loading and validation
- Pod lifecycle (create, monitor, destroy)
- Unified snapshot operations
- Authentication via FreeIPA
- Canvas LTI integration

### Vue.js Frontend

Student and instructor interface providing:

- Lab catalog browsing
- Pod management
- VM console access (VNC/SPICE)
- Snapshot controls
- Progress tracking and achievements
- Reservation scheduling

Built with Vue.js 3, Volt PrimeVue components, and Tailwind CSS v4. See [Frontend Architecture](frontend.md) for details.

### Python Utilities

Supporting tools for:

- Lab template parsing and validation
- Canvas integration helpers
- Automation scripts

## Data Flow

```mermaid
sequenceDiagram
    participant Student
    participant Canvas
    participant API
    participant Platform
    
    Student->>Canvas: Start Assignment
    Canvas->>API: LTI Launch
    API->>API: Validate User
    API->>Platform: Provision Pod
    Platform-->>API: Pod Ready
    API-->>Canvas: Redirect to Lab
    Student->>API: Access VMs
```

## Network Architecture

| VLAN | Subnet | Purpose |
|------|--------|---------|

