# Why Go for Kootenai Platform

Looking at this project's requirements, Go is well-suited for several reasons.

## 1. Concurrency for Orchestration

The platform manages multiple pods, VMs, and sessions simultaneously. Go's goroutines handle this elegantly:

```go
// Creating a pod spins up multiple VMs concurrently
func (o *Orchestrator) CreatePod(ctx context.Context, template *LabTemplate, owner string) (*Pod, error) {
    var wg sync.WaitGroup
    errCh := make(chan error, len(template.VMs))

    for _, vmSpec := range template.VMs {
        wg.Add(1)
        go func(spec VMSpec) {
            defer wg.Done()
            if err := o.proxmox.CloneVM(ctx, spec); err != nil {
                errCh <- err
            }
        }(vmSpec)
    }
    wg.Wait()
    // ...
}
```

Creating 5 VMs takes the same time as creating 1. Python/Node would need explicit thread pools or async complexity.

## 2. Real-Time Event Processing

NATS consumers process thousands of Wazuh events per second:

```go
// Each consumer runs in its own goroutine
go eventStoreConsumer.Start(ctx)
go checkpointConsumer.Start(ctx)
go wsBroadcastConsumer.Start(ctx)
```

Go's channel-based communication makes this natural:

```go
// WebSocket hub broadcasts to all connected clients
func (h *Hub) Run(ctx context.Context) {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
        case client := <-h.unregister:
            delete(h.clients, client)
        case message := <-h.broadcast:
            for client := range h.clients {
                client.send <- message
            }
        case <-ctx.Done():
            return
        }
    }
}
```

## 3. Single Binary Deployment

From CLAUDE.md: *"Go for the core orchestration API (performance, type safety, single binary)"*

```bash
# Build once, deploy anywhere
go build -o labctl ./cmd/labctl

# Result: one 15MB binary with zero dependencies
scp labctl server:/usr/local/bin/
```

No runtime to install, no dependency conflicts, no virtual environments. Compare to Python requiring pip, venv, and matching system libraries.

## 4. Low Memory Footprint

Each lab session maintains WebSocket connections and checkpoint state. Go's efficiency matters:

| Runtime | Memory per connection | 1000 concurrent users |
|---------|----------------------|----------------------|
| Go | ~4KB goroutine stack | ~4MB |
| Node.js | ~1MB per connection | ~1GB |
| Python | ~8MB per thread | Threading issues |

## 5. Strong Typing for API Contracts

Complex nested structures stay consistent between API layers:

```go
type Pod struct {
    ID          string       `json:"id"`
    LabTemplate string       `json:"labTemplate"`
    Platform    PlatformType `json:"platform"`  // Enum - can't be invalid
    Status      PodStatus    `json:"status"`    // Enum
    VMs         []PodVM      `json:"vms"`
    CreatedAt   time.Time    `json:"createdAt"`
    ExpiresAt   *time.Time   `json:"expiresAt,omitempty"`
}
```

The compiler catches mismatches. Python would silently pass wrong types until runtime.

## 6. Excellent HTTP/Network Libraries

Standard library covers most needs without external dependencies:

```go
// Built-in HTTP server with timeouts
srv := &http.Server{
    Addr:         ":8080",
    Handler:      router,
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
}

// Context propagation for cancellation
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
result, err := proxmox.CloneVM(ctx, vmSpec)  // Automatically cancelled if slow
```

## 7. Infrastructure Ecosystem

Go dominates infrastructure tooling - compatible libraries exist for everything this project needs:

| Component | Go Library |
|-----------|-----------|
| Proxmox API | Direct HTTP (REST API) |
| CloudStack | `github.com/apache/cloudstack-go` |
| NATS/JetStream | `github.com/nats-io/nats.go` |
| PostgreSQL | `github.com/lib/pq` |
| JWT Auth | `github.com/golang-jwt/jwt` |
| WebSockets | `github.com/gorilla/websocket` |
| HTTP Router | `github.com/go-chi/chi` |

Kubernetes, Docker, Terraform, Prometheus - all Go. The ecosystem understands infrastructure problems.

## 8. Graceful Shutdown

Long-running operations (VM provisioning) need clean termination:

```go
// Handle shutdown signals
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    cancel()  // Propagates to all operations via context
}()

// Server waits for in-flight requests
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
srv.Shutdown(shutdownCtx)
```

## 9. Fast Compilation & Testing

Development cycle stays quick:

```bash
go build ./cmd/labctl    # ~2 seconds
go test ./...            # Parallel by default
```

No waiting for transpilation, bundling, or container rebuilds during development.

## Why Not Alternatives?

| Language | Limitation for This Project |
|----------|---------------------------|
| **Python** | GIL limits true parallelism, deployment complexity, runtime errors |
| **Node.js** | Single-threaded, callback complexity, memory overhead |
| **Rust** | Slower development, steeper learning curve, overkill for I/O-bound work |
| **Java** | JVM startup time, memory overhead, deployment complexity |

## Summary

Go fits because this project is:

- **I/O-bound** (API calls to Proxmox/CloudStack, database, NATS) - goroutines excel
- **Infrastructure-focused** - Go's ecosystem dominates this space
- **Ops-deployed** - single binary simplifies deployment
- **Real-time** - efficient WebSocket/event handling
- **Type-sensitive** - complex nested configs need compile-time safety

The project wouldn't be impossible in Python or Node, but Go reduces operational complexity while handling concurrency naturally.
