---
title: Common Issues
description: Troubleshooting guide for common Kootenai problems
tags:
  - troubleshooting
  - faq
  - errors
---

# Common Issues

This guide covers frequently encountered issues and their solutions.

## Quick Diagnostics

Run the diagnostic tool to check system health:

```bash
# Install labtest
cd tools/labtest && pip install -e .

# Run diagnostics
labtest diagnose
labtest health
```

## API Issues

### API Returns 401 Unauthorized

**Symptoms:**
- All API calls fail with 401
- "invalid token" or "token expired" errors

**Solutions:**

1. **Check token expiration**
   ```bash
   # Decode JWT to check expiry
   echo $TOKEN | cut -d'.' -f2 | base64 -d 2>/dev/null | jq .exp
   ```

2. **Verify JWT secret matches**
   ```bash
   # On infra VM
   grep JWT_SECRET ~/kootenai/deploy/.env
   ```

3. **For demo mode**, ensure no Authorization header is sent
   ```bash
   # Demo mode only works WITHOUT auth header
   curl http://<INFRA_VM_IP>:8080/api/v1/labs
   ```

### API Returns 500 Internal Server Error

**Symptoms:**
- Intermittent 500 errors
- "database connection" errors in logs

**Solutions:**

1. **Check database connectivity**
   ```bash
   docker exec kootenai-api pg_isready -h postgres -U labadmin
   ```

2. **Check API logs**
   ```bash
   docker logs kootenai-api --tail 100
   ```

3. **Restart API container**
   ```bash
   cd ~/kootenai/deploy
   docker compose restart api
   ```

### API Container Keeps Restarting

**Symptoms:**
- Container status shows "Restarting"
- Health checks failing

**Solutions:**

1. **Check logs for startup errors**
   ```bash
   docker logs kootenai-api 2>&1 | head -50
   ```

2. **Common causes:**
   - Missing environment variables
   - Database migration failures
   - Port conflicts

3. **Verify dependencies are healthy**
   ```bash
   docker compose ps
   # Ensure postgres and nats show "healthy"
   ```

## Database Issues

### Migration Failures

**Symptoms:**
- "migration failed" errors on startup
- Schema version mismatch

**Solutions:**

1. **Check migration status**
   ```bash
   mage db:status
   ```

2. **Run pending migrations**
   ```bash
   mage db:migrate
   ```

3. **For stuck migrations**, check the schema_migrations table:
   ```sql
   SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;
   ```

### Connection Pool Exhausted

**Symptoms:**
- "too many connections" errors
- Slow API responses

**Solutions:**

1. **Check active connections**
   ```sql
   SELECT count(*) FROM pg_stat_activity WHERE datname = 'kootenai';
   ```

2. **Increase pool size** in `.env`:
   ```
   DATABASE_MAX_CONNECTIONS=50
   ```

3. **Restart API to reset connections**
   ```bash
   docker compose restart api
   ```

## Pod Issues

### Pod Stuck in "Provisioning"

**Symptoms:**
- Pod status never changes to "running"
- Timeout errors after 5+ minutes

**Solutions:**

1. **Check Proxmox connectivity**
   ```bash
   mage proxmox:testConn
   ```

2. **Check Proxmox task status**
   ```bash
   # On Proxmox host
   pvesh get /nodes/pve/tasks --limit 10
   ```

3. **Check API logs for Proxmox errors**
   ```bash
   docker logs kootenai-api 2>&1 | grep -i proxmox
   ```

4. **Verify template exists**
   ```bash
   mage proxmox:listTemplates
   ```

### Pod VMs Not Starting

**Symptoms:**
- Pod shows "running" but VMs are stopped
- Console shows "VM not found"

**Solutions:**

1. **Check VM status on Proxmox**
   ```bash
   pvesh get /nodes/pve/qemu
   ```

2. **Manually start VMs**
   ```bash
   qm start <vmid>
   ```

3. **Check storage availability**
   ```bash
   pvesh get /nodes/pve/storage
   ```

### Cannot Connect to Pod Console

**Symptoms:**
- VNC console shows black screen
- "Connection refused" errors

**Solutions:**

1. **Verify VM is running**
   ```bash
   qm status <vmid>
   ```

2. **Check VNC port**
   ```bash
   qm config <vmid> | grep vnc
   ```

3. **Restart VM's display**
   ```bash
   qm set <vmid> -vga std
   qm reboot <vmid>
   ```

## Network Issues

### Pods Cannot Reach Internet

**Symptoms:**
- VMs can ping each other but not external IPs
- DNS resolution fails

**Solutions:**

1. **Check NAT configuration on Proxmox**
   ```bash
   iptables -t nat -L -n | grep MASQUERADE
   ```

2. **Verify routing**
   ```bash
   ip route show
   ```

3. **Check firewall rules**
   ```bash
   iptables -L FORWARD -n
   ```

### Pods Cannot Communicate

**Symptoms:**
- VMs in same pod can't ping each other
- VLAN isolation issues

**Solutions:**

1. **Verify VLAN assignment**
   ```bash
   qm config <vmid> | grep net
   ```

2. **Check bridge configuration**
   ```bash
   brctl show
   ```

3. **Verify network isolation settings** in lab template

## Web UI Issues

### Page Shows Blank or Loading Forever

**Symptoms:**
- White screen after login
- Spinner never stops

**Solutions:**

1. **Check browser console** (F12) for JavaScript errors

2. **Clear browser cache**
   ```
   Ctrl+Shift+Delete → Clear cached images and files
   ```

3. **Verify API is accessible**
   ```bash
   curl http://<INFRA_VM_IP>:8080/health
   ```

### Login Redirects to Error Page

**Symptoms:**
- OAuth callback fails
- "Invalid state" errors

**Solutions:**

1. **Clear cookies and try again**

2. **Verify redirect URLs** in OAuth configuration

3. **Check time synchronization** (JWT validation fails with clock skew)

### Real-time Updates Not Working

**Symptoms:**
- Progress doesn't update automatically
- Must refresh to see changes

**Solutions:**

1. **Check WebSocket connection** in browser DevTools → Network → WS

2. **Verify NATS is running**
   ```bash
   docker logs kootenai-nats
   ```

3. **Check for WebSocket proxy issues** in nginx configuration

## Canvas LTI Issues

### "Invalid Audience" Error

**Cause:** Wrong `CANVAS_CLIENT_ID` in configuration

**Solution:**
```bash
# Get correct client ID
docker compose exec -T web rails runner 'dk = DeveloperKey.find(2); puts dk.global_id'
# Use this value (e.g., 10000000000002) as CANVAS_CLIENT_ID
```

### "512-bit Keys Are Insecure"

**Cause:** Canvas has old RSA keys

**Solution:**
```bash
# Regenerate Canvas RSA keys
docker compose exec -T web rails runner '
  proxy = DynamicSettings.kv_proxy("lti-keys", tree: :store)
  3.times.map { |i| ["jwk-#{%w[past present future][i]}.json", CanvasSecurity::RSAKeyPair.new.to_jwk.to_json] }.to_h
    .then { |keys| proxy.set_keys(keys, global: true) }
  DynamicSettings.reset_cache!
'
docker compose restart web
```

### Frame Embedding Blocked

**Symptoms:**
- CSP "frame-ancestors" error
- Kootenai won't load in Canvas iframe

**Solution:**
Set correct domain in Canvas configuration:
```yaml
# canvas-lms/config/domain.yml
domain: "localhost"  # For local development
```

## Wazuh/Checkpoint Issues

### Checkpoints Not Detected

**Symptoms:**
- Completed tasks don't register
- Progress stays at 0%

**Solutions:**

1. **Verify Wazuh agent is running** in VM
   ```bash
   systemctl status wazuh-agent
   ```

2. **Check syscheck configuration**
   ```bash
   cat /var/ossec/etc/ossec.conf | grep -A5 syscheck
   ```

3. **Verify monitored paths** match checkpoint triggers

4. **Check Wazuh manager logs**
   ```bash
   docker logs wazuh-manager --tail 100
   ```

### Delayed Checkpoint Updates

**Symptoms:**
- Checkpoints take minutes to register
- Inconsistent detection timing

**Solutions:**

1. **Reduce syscheck interval** in ossec.conf:
   ```xml
   <syscheck>
     <frequency>60</frequency>
   </syscheck>
   ```

2. **Force immediate scan**
   ```bash
   /var/ossec/bin/agent_control -r -a
   ```

## Docker Issues

### Container Not Reflecting Code Changes

**Solution:**
```bash
# Rebuild container
docker compose build api

# Force recreate
docker compose up -d --force-recreate api
```

### Disk Space Issues

**Symptoms:**
- "No space left on device" errors
- Container creation fails

**Solutions:**

1. **Clean unused images**
   ```bash
   docker system prune -a
   ```

2. **Check disk usage**
   ```bash
   docker system df
   ```

3. **Remove old volumes**
   ```bash
   docker volume prune
   ```

## Performance Issues

### Slow API Responses

**Solutions:**

1. **Check database query performance**
   ```sql
   SELECT query, mean_time, calls
   FROM pg_stat_statements
   ORDER BY mean_time DESC
   LIMIT 10;
   ```

2. **Add database indexes** for slow queries

3. **Enable connection pooling**

4. **Check for N+1 query patterns** in logs

### High Memory Usage

**Solutions:**

1. **Check container memory**
   ```bash
   docker stats
   ```

2. **Adjust container limits** in docker-compose.yml:
   ```yaml
   deploy:
     resources:
       limits:
         memory: 512M
   ```

3. **Check for memory leaks** in API logs

## If none of this resolves it

This repository is archived. Issues and pull requests are closed, so there is no one to
escalate to — the remaining steps are diagnostic.

1. **Collect the full picture**
   ```bash
   labtest diagnose > diagnostics.txt
   docker compose logs > docker-logs.txt
   ```

2. **Read the failure directly.** `labtest diagnose` reports which of the API, database
   and NATS are actually reachable, which narrows most problems to one container.

3. **Work from the source.** The code is Apache 2.0 and the test suites run locally
   (`mage test:all`), so a fork can be patched in place. That is the only path left.

## Related Documentation

- [LTI & VNC Troubleshooting](lti-vnc-console.md)
- [Wazuh Syscheck Fix](wazuh-syscheck-parsing-fix.md)
- [Production Deployment](../admin/production-deployment.md)
