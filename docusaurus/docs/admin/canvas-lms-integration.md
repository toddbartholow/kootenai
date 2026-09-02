# Canvas LMS Integration Guide

:::info Enterprise Edition Feature
LTI 1.3 integration (Canvas, Moodle, Blackboard) requires **Kootenai Enterprise Edition**.
:::

This guide covers setting up Canvas LMS integration with Kootenai using LTI 1.3.

## Overview

Kootenai integrates with Canvas LMS to provide:

- **Single Sign-On (SSO)** - Students launch labs directly from Canvas
- **Grade Passback** - Lab scores automatically sync to Canvas gradebook
- **Course Mapping** - Canvas courses map to Kootenai organizations
- **Section Sync** - Canvas sections can sync to Kootenai teams

## Architecture

```
┌─────────────────┐         ┌──────────────────┐
│   Canvas LMS    │         │   Kootenai    │
│                 │         │                  │
│  ┌───────────┐  │  LTI    │  ┌────────────┐  │
│  │  Course   │──┼─Launch──┼──│    Org     │  │
│  │           │  │         │  │            │  │
│  │ ┌───────┐ │  │         │  │ ┌────────┐ │  │
│  │ │Section│─┼──┼─────────┼──┼─│  Team  │ │  │
│  │ └───────┘ │  │         │  │ └────────┘ │  │
│  │           │  │         │  │            │  │
│  │ ┌───────┐ │  │  Grade  │  │ ┌────────┐ │  │
│  │ │Assign │◄┼──┼─Sync────┼──┼─│Session │ │  │
│  │ └───────┘ │  │         │  │ └────────┘ │  │
│  └───────────┘  │         │  └────────────┘  │
└─────────────────┘         └──────────────────┘
```

## Prerequisites

- Kootenai API running with LTI endpoints enabled
- Canvas LMS instance (cloud or self-hosted)
- RSA key pair for LTI authentication
- Admin access to both systems

---

## Option 1: Local Canvas Development Setup

For development and testing, you can run Canvas locally using Docker.

### Quick Setup

```bash
# Clone Canvas LMS
git clone https://github.com/instructure/canvas-lms.git
cd canvas-lms

# Create environment file
echo "COMPOSE_FILE=docker-compose.yml:docker-compose.override.yml" > .env

# Create admin setup config
cat > .canvas-setup.env << EOF
CANVAS_LMS_ADMIN_EMAIL=admin@example.com
CANVAS_LMS_ADMIN_PASSWORD=password123
CANVAS_LMS_ACCOUNT_NAME=Kootenai Dev
CANVAS_LMS_STATS_COLLECTION=opt_out
EOF

# Build and start containers
docker compose build
docker compose run --rm web bundle exec rake db:create db:initial_setup
docker compose up -d
```

### Configure Port Mapping

Edit `docker-compose.override.yml` to expose Canvas on port 3001:

```yaml
web:
  <<: *BASE
  ports:
    - "3001:80"
```

Restart the web container:

```bash
docker compose up -d --force-recreate web
```

### Access Canvas

- **URL**: http://localhost:3001
- **Login**: admin@example.com / password123

### Docker Containers

| Container | Purpose |
|-----------|---------|
| canvas-lms-web | Rails application server |
| canvas-lms-postgres | PostgreSQL database |
| canvas-lms-redis | Redis cache |
| canvas-lms-jobs | Background job processor |
| canvas-lms-webpack | Asset compilation |

### Useful Commands

```bash
# View logs
docker compose logs -f web

# Rails console
docker compose exec web bundle exec rails console

# Check container status
docker compose ps

# Stop all containers
docker compose down

# Reset database
docker compose run --rm web bundle exec rake db:reset
```

---

## Option 2: Canvas Cloud Setup

For production or if using Canvas cloud (instructure.com):

1. Log in to your Canvas instance as an admin
2. Navigate to **Admin > Developer Keys**
3. Click **+ Developer Key > LTI Key**
4. Configure as described in the LTI Configuration section below

---

## LTI 1.3 Configuration

### Step 1: Generate RSA Keys

Kootenai needs an RSA key pair for signing JWT tokens:

```bash
mkdir -p config/lti
cd config/lti

# Generate private key
openssl genrsa -out private.pem 2048

# Extract public key
openssl rsa -in private.pem -pubout -out public.pem
```

### Step 2: Create Developer Key in Canvas

**Via Canvas Admin UI:**

1. Go to **Admin > Developer Keys**
2. Click **+ Developer Key > LTI Key**
3. Configure:
   - **Key Name**: Kootenai LTI
   - **Redirect URIs**: `http://your-kootenai.example.com/lti/callback`
   - **Method**: Manual Entry

**Via Rails Console (Local Canvas):**

```ruby
# In canvas-lms directory
docker compose exec web bundle exec rails runner -

public_jwk = {
  "kty" => "RSA",
  "e" => "AQAB",
  "n" => "<your-public-key-modulus>",
  "kid" => "kootenai-lti-key",
  "alg" => "RS256",
  "use" => "sig"
}

placements = [
  {
    "placement" => "course_navigation",
    "message_type" => "LtiResourceLinkRequest",
    "target_link_uri" => "http://host.docker.internal:8080/lti/launch",
    "text" => "Kootenai",
    "enabled" => true
  }
]

scopes = [
  "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem",
  "https://purl.imsglobal.org/spec/lti-ags/scope/score",
  "https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly"
]

dk = DeveloperKey.create!(
  name: "Kootenai LTI",
  account: Account.default,
  redirect_uris: ["http://host.docker.internal:8080/lti/callback"],
  public_jwk: public_jwk,
  is_lti_key: true,
  scopes: scopes
)

Lti::ToolConfiguration.create!(
  developer_key: dk,
  title: "Kootenai",
  description: "Kootenai Platform",
  oidc_initiation_url: "http://host.docker.internal:8080/lti/launch",
  target_link_uri: "http://host.docker.internal:8080/lti/launch",
  public_jwk: public_jwk,
  placements: placements,
  scopes: scopes,
  privacy_level: "public"
)

dk.update!(workflow_state: "active")
puts "Client ID: #{dk.global_id}"
```

### Step 3: Enable Developer Key

```ruby
# Enable for the account
DeveloperKeyAccountBinding.create!(
  account: Account.default,
  developer_key: dk,
  workflow_state: "on"
)
```

### Step 4: Configure Kootenai

Create `config/lti/canvas-config.yaml`:

```yaml
canvas:
  # Canvas instance URL
  url: "http://localhost:3001"  # or https://your-canvas.instructure.com

  # LTI 1.3 credentials (from Developer Key)
  client_id: "10000000000001"
  deployment_id: "1"

  # Canvas platform endpoints
  authorization_url: "http://localhost:3001/api/lti/authorize_redirect"
  token_url: "http://localhost:3001/login/oauth2/token"
  jwks_url: "http://localhost:3001/api/lti/security/jwks"

  # Kootenai tool configuration
  tool_issuer: "http://localhost:8080"
  tool_public_key: "/config/lti/public.pem"
  tool_private_key: "/config/lti/private.pem"

  # Timeouts
  token_expiry: "1h"
  request_timeout: "30s"
```

### Step 5: Install Tool in Course

**Via Canvas UI:**

1. Go to Course > Settings > Apps
2. Click **+ App**
3. Configuration Type: **By Client ID**
4. Enter your Developer Key Client ID
5. Click **Submit**

**Via Rails Console:**

```ruby
course = Course.find(1)  # or Course.find_by(name: "Your Course")
dk = DeveloperKey.find_by(name: "Kootenai LTI")

ContextExternalTool.create!(
  context: course,
  name: "Kootenai",
  consumer_key: dk.global_id.to_s,
  shared_secret: "not_used_for_lti_1_3",
  url: "http://host.docker.internal:8080/lti/launch",
  developer_key: dk,
  workflow_state: "public",
  lti_version: "1.3"
)
```

---

## Kootenai Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /lti/launch` | LTI launch endpoint (OIDC initiation) |
| `GET /lti/jwks` | JSON Web Key Set for token verification |
| `POST /lti/token` | Token exchange endpoint |
| `GET /lti/callback` | OAuth callback after Canvas auth |

---

## Course-to-Organization Mapping

When a student launches Kootenai from Canvas, the platform:

1. Extracts course info from LTI launch request
2. Finds or creates a matching organization
3. Auto-enrolls the user in that organization
4. Sets organization context for the session

### Mapping Rules

| Canvas | Kootenai |
|--------|-------------|
| Course ID | Organization external ID |
| Course Name | Organization name |
| Course Code | Organization slug |
| Section | Team (if enabled) |
| Enrollment Role | Organization role |

### Role Mapping

| Canvas Role | Kootenai Role |
|-------------|-----------------|
| Teacher | instructor |
| TA | instructor |
| Designer | instructor |
| Student | member |
| Observer | member |

---

## Grade Passback (AGS)

Kootenai uses LTI Assignment and Grade Services (AGS) to sync scores.

### How It Works

1. Student completes lab and submits
2. Kootenai calculates final score
3. Score is posted to Canvas via AGS
4. Canvas updates gradebook

### AGS Scopes Required

- `https://purl.imsglobal.org/spec/lti-ags/scope/lineitem`
- `https://purl.imsglobal.org/spec/lti-ags/scope/score`
- `https://purl.imsglobal.org/spec/lti-ags/scope/result.readonly`

### Score Format

```json
{
  "userId": "canvas-user-id",
  "scoreGiven": 85.0,
  "scoreMaximum": 100.0,
  "activityProgress": "Completed",
  "gradingProgress": "FullyGraded",
  "timestamp": "2026-01-08T12:00:00Z"
}
```

---

## Section-to-Team Sync

Enable automatic team creation from Canvas sections:

```yaml
# In canvas-config.yaml
canvas:
  sync:
    sections_to_teams: true
    auto_enroll: true
```

When enabled:
- Each Canvas section creates a corresponding team
- Students are auto-enrolled in their section's team
- Team membership updates when Canvas enrollment changes

---

## Troubleshooting

### LTI Launch Fails

1. **Check Developer Key is active**
   ```ruby
   DeveloperKey.find(client_id).workflow_state
   # Should be "active"
   ```

2. **Verify account binding**
   ```ruby
   DeveloperKeyAccountBinding.where(developer_key_id: dk.id).first.workflow_state
   # Should be "on"
   ```

3. **Check redirect URI matches**
   - Canvas Developer Key redirect_uris
   - Kootenai configured callback URL

4. **Review logs**
   ```bash
   # Canvas logs
   docker compose logs web | grep -i lti

   # Kootenai logs
   docker compose logs api | grep -i lti
   ```

### Grade Passback Fails

1. **Verify AGS scopes are enabled** in Developer Key

2. **Check session has Canvas context**
   - Session must have `canvas_course_id` and `canvas_assignment_id`

3. **Verify token exchange**
   - Check Kootenai can obtain Canvas access token

4. **Review AGS endpoint**
   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     "https://canvas/api/lti/courses/$COURSE_ID/line_items"
   ```

### Tool Not Appearing in Course

1. **Check tool is installed in course**
   ```ruby
   Course.find(1).context_external_tools.pluck(:name)
   ```

2. **Verify placement configuration**
   - `course_navigation` placement must be enabled

3. **Check privacy level**
   - Should be "public" for full user info

### Canvas Container Issues

```bash
# Check all containers are running
docker compose ps

# Restart specific container
docker compose restart web

# View container logs
docker compose logs -f web

# Access Rails console for debugging
docker compose exec web bundle exec rails console
```

---

## Security Considerations

1. **Private Key Protection**
   - Never commit `private.pem` to version control
   - Use environment variables or secrets management
   - Rotate keys periodically

2. **Validate All LTI Claims**
   - Verify issuer matches Canvas
   - Check deployment ID
   - Validate JWT signature

3. **HTTPS in Production**
   - Always use HTTPS for production Canvas
   - LTI 1.3 requires secure connections

4. **Scope Limitations**
   - Only request necessary AGS/NRPS scopes
   - Audit scope usage

---

## Configuration Reference

### LTI Tool Configuration JSON

```json
{
  "title": "Kootenai",
  "description": "Cybersecurity and Network Operations Kootenai Platform",
  "oidc_initiation_url": "https://your-kootenai.example.com/lti/launch",
  "target_link_uri": "https://your-kootenai.example.com/lti/launch",
  "scopes": [
    "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem",
    "https://purl.imsglobal.org/spec/lti-ags/scope/lineitem.readonly",
    "https://purl.imsglobal.org/spec/lti-ags/scope/result.readonly",
    "https://purl.imsglobal.org/spec/lti-ags/scope/score",
    "https://purl.imsglobal.org/spec/lti-nrps/scope/contextmembership.readonly"
  ],
  "extensions": [
    {
      "platform": "canvas.instructure.com",
      "privacy_level": "public",
      "settings": {
        "placements": [
          {
            "placement": "course_navigation",
            "message_type": "LtiResourceLinkRequest",
            "text": "Kootenai",
            "enabled": true
          },
          {
            "placement": "assignment_selection",
            "message_type": "LtiDeepLinkingRequest",
            "text": "Kootenai Assignment"
          }
        ]
      }
    }
  ],
  "public_jwk_url": "https://your-kootenai.example.com/lti/jwks",
  "custom_fields": {
    "canvas_user_id": "$Canvas.user.id",
    "canvas_course_id": "$Canvas.course.id",
    "canvas_assignment_id": "$Canvas.assignment.id"
  }
}
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CANVAS_URL` | Canvas instance URL | - |
| `LTI_CLIENT_ID` | Developer Key Client ID | - |
| `LTI_DEPLOYMENT_ID` | Deployment ID | - |
| `LTI_PRIVATE_KEY_PATH` | Path to private key | `/config/lti/private.pem` |
| `LTI_PUBLIC_KEY_PATH` | Path to public key | `/config/lti/public.pem` |

---

## Related Documentation

- [Instructor Guide](../instructor/guide.md) - Using LTI from instructor perspective
- [API Reference](../api/reference.md) - LTI endpoint details
- [Multi-Tenancy Plan](../architecture/multi-tenancy-plan.md) - Organization mapping
- [Canvas LTI 1.3 Documentation](https://canvas.instructure.com/doc/api/file.lti_dev_key_config.html)
