---
title: OpenAPI Specification
description: Interactive OpenAPI/Swagger documentation for the Kootenai API
tags:
  - api
  - openapi
  - swagger
---

# OpenAPI Specification

Kootenai provides an OpenAPI 3.0 specification for the REST API.

## Interactive Documentation

### Swagger UI

Access the interactive Swagger UI at:

```
http://localhost:8080/swagger/
```

The Swagger UI provides:

- Browse all API endpoints
- View request/response schemas
- Try out API calls directly
- View authentication requirements

## OpenAPI Spec Files

### JSON — the only format served

```
http://localhost:8080/swagger/doc.json
```

### YAML — file only, not served

```
api/docs/swagger.yaml
```

:::caution `/swagger/doc.yaml` returns 404
The server mounts `httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json"))`
(`api/internal/server/server.go:1483-1485`). That handler special-cases only `index.html`
and `doc.json`; anything else falls through to the embedded swagger-ui assets, which contain
no `doc.yaml`. The YAML spec exists on disk at `api/docs/swagger.yaml` but is not wired to
any route. Read it from the repository, or fetch the JSON and convert.
:::

Or view directly in the repository:

```
api/docs/swagger.json
```

## Using the Spec

### Import into Postman

1. Open Postman
2. Click **Import**
3. Select **Link** tab
4. Enter: `http://localhost:8080/swagger/doc.json`
5. Click **Continue** → **Import**

### Import into Insomnia

1. Open Insomnia
2. Click **Create** → **Import from URL**
3. Enter: `http://localhost:8080/swagger/doc.json`
4. Click **Fetch and Import**

### Generate Client SDKs

Use OpenAPI Generator to create client libraries:

```bash
# Install OpenAPI Generator
npm install @openapitools/openapi-generator-cli -g

# Generate TypeScript client
openapi-generator-cli generate \
  -i http://localhost:8080/swagger/doc.json \
  -g typescript-fetch \
  -o ./generated/typescript-client

# Generate Python client
openapi-generator-cli generate \
  -i http://localhost:8080/swagger/doc.json \
  -g python \
  -o ./generated/python-client

# Generate Go client
openapi-generator-cli generate \
  -i http://localhost:8080/swagger/doc.json \
  -g go \
  -o ./generated/go-client
```

## API Versioning

The API uses URL versioning:

```
/api/v1/...
```

The OpenAPI spec covers v1 of the API. Future versions will have separate specs.

## Authentication in OpenAPI

The spec defines a Bearer token security scheme:

```yaml
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
```

Most endpoints require authentication:

```yaml
security:
  - BearerAuth: []
```

## Testing with curl

The spec can help you construct curl commands:

### Example: List Labs

From the spec:

```yaml
paths:
  /labs:
    get:
      summary: List all lab templates
      parameters:
        - name: platform
          in: query
          schema:
            type: string
        - name: active
          in: query
          schema:
            type: boolean
```

Translates to:

```bash
curl -X GET "http://localhost:8080/api/v1/labs?platform=proxmox&active=true" \
  -H "Authorization: Bearer $TOKEN"
```

### Example: Create Pod

From the spec:

```yaml
paths:
  /pods:
    post:
      summary: Create a new pod
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [labTemplate, owner]
              properties:
                labTemplate:
                  type: string
                owner:
                  type: string
```

Translates to:

```bash
curl -X POST "http://localhost:8080/api/v1/pods" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"labTemplate": "linux-basics-101", "owner": "jsmith"}'
```

## Schema Definitions

Key schemas defined in the spec:

| Schema | Description |
|--------|-------------|
| `Lab` | Lab template definition |
| `Pod` | Pod instance with VMs |
| `Session` | Lab session with progress |
| `User` | User account |
| `Achievement` | Achievement definition |
| `Organization` | Organization/tenant |
| `Team` | Team within organization |

## Keeping Spec Updated

The OpenAPI spec is generated from code annotations using [swaggo/swag](https://github.com/swaggo/swag).

### Regenerate Spec

```bash
mage build:swagger
```

Use the Mage target rather than calling `swag` directly. The bare
`swag init -g cmd/labctl/main.go -o docs` misses `--parseDependency`,
`--parseInternal` and `--dir ./,./internal/server`, and produces a spec with
76 definitions instead of 115 — which then fails `npm run check:api-types`.
The equivalent full command is:

```bash
cd api
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/labctl/main.go -o docs \
  --parseDependency --parseInternal --dir ./,./internal/server
```

### Annotation Example

```go
// ListLabs godoc
// @Summary List all lab templates
// @Description Get a list of all available lab templates
// @Tags labs
// @Accept json
// @Produce json
// @Param platform query string false "Filter by platform"
// @Param active query boolean false "Filter by active status"
// @Success 200 {object} LabListResponse
// @Failure 401 {object} ErrorResponse
// @Security BearerAuth
// @Router /labs [get]
func (s *Server) ListLabs(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

## Related Documentation

- [Authentication API](authentication.md) - Auth endpoints
- [Labs & Pods API](labs-pods.md) - Core endpoints
- [Sessions API](sessions.md) - Session management
- [Achievements API](achievements.md) - Achievements
- [Organizations API](organizations.md) - Multi-tenancy
