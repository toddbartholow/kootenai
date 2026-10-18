# Secret Rotation Procedures

This guide covers the procedures for rotating secrets and credentials in the Kootenai platform. Regular secret rotation is a security best practice that limits the exposure window if credentials are compromised.

## Quick Reference

| Secret | Rotation Frequency | Downtime | Priority |
|--------|-------------------|----------|----------|
| JWT Secret | Quarterly or after breach | Brief (~30s) | High |
| Database Password | Quarterly | Brief (~1min) | High |
| Proxmox API Token | Annually | None | Medium |
| Canvas LTI Keys | Annually | None | Medium |
| NATS Credentials | Annually | Brief (~30s) | Low |

---

## 1. JWT Secret Rotation

The JWT secret is used to sign authentication tokens. It must be at least 32 characters.

### Prerequisites
- Access to the infra VM
- No active critical operations

### Procedure

1. **Generate a new secret:**
   ```bash
   openssl rand -base64 32
   ```

2. **Update the secret on the infra VM:**
   ```bash
   ssh labadmin@<INFRA_VM_IP>
   cd ~/kootenai/deploy

   # Backup current .env
   cp .env .env.backup-$(date +%Y%m%d)

   # Edit .env and update JWT_SECRET
   nano .env
   ```

3. **Restart the API service:**
   ```bash
   docker compose restart api
   ```

4. **Verify the service is healthy:**
   ```bash
   curl -s http://localhost:8080/health | jq .
   ```

### Impact
- All existing user sessions will be invalidated
- Users will need to log in again
- Token blacklist in Redis will be reset (if using Redis)

### Grace Period
The system does not support dual JWT secrets. All tokens signed with the old secret will immediately become invalid. Plan rotation during low-usage periods.

---

## 2. Database Password Rotation

### Prerequisites
- Access to the infra VM with sudo privileges
- Brief maintenance window

### Procedure

1. **Generate a new password:**
   ```bash
   openssl rand -base64 24
   ```

2. **Update PostgreSQL password:**
   ```bash
   ssh labadmin@<INFRA_VM_IP>
   cd ~/kootenai/deploy

   # Connect to PostgreSQL
   docker compose exec postgres psql -U labadmin -d virtuallab

   # In psql, change the password:
   ALTER USER labadmin WITH PASSWORD 'new_password_here';
   \q
   ```

3. **Update the .env file:**
   ```bash
   # Edit .env and update DATABASE_PASSWORD
   nano .env
   ```

4. **Restart services that connect to the database:**
   ```bash
   docker compose restart api
   ```

5. **Verify connectivity:**
   ```bash
   docker compose exec api /app/labctl health
   # Or check logs
   docker compose logs api | tail -20
   ```

### Rollback
If issues occur, restore the backup .env and restart services:
```bash
cp .env.backup-YYYYMMDD .env
docker compose restart api
```

---

## 3. Proxmox API Token Rotation

Proxmox API tokens are used to manage lab VMs.

### Prerequisites
- Access to Proxmox web UI or CLI
- Admin privileges in Proxmox

### Procedure

1. **Create a new API token in Proxmox:**

   **Via Web UI:**
   - Navigate to Datacenter > Permissions > API Tokens
   - Click "Add"
   - User: `automation@pve` (or your automation user)
   - Token ID: `lab-api-v2` (use versioned names)
   - Privilege Separation: Unchecked (inherit user permissions)
   - Click "Add" and **copy the token secret immediately**

   **Via CLI:**
   ```bash
   pveum user token add automation@pve lab-api-v2
   ```

2. **Update the Kootenai configuration:**
   ```bash
   ssh labadmin@<INFRA_VM_IP>
   cd ~/kootenai/deploy

   # Edit .env
   nano .env
   # Update:
   # PROXMOX_TOKEN_ID=<PROXMOX_TOKEN_ID>-v2
   # PROXMOX_TOKEN=<new-token-secret>
   ```

3. **Restart the API:**
   ```bash
   docker compose restart api
   ```

4. **Verify Proxmox connectivity:**
   ```bash
   # Check API can list VMs
   curl -s http://localhost:8080/api/v1/pods | jq .
   ```

5. **Delete the old token in Proxmox** (after verifying new token works):
   - Navigate to Datacenter > Permissions > API Tokens
   - Select the old token and click "Remove"

### Zero-Downtime Rotation
Proxmox tokens can coexist. Create the new token first, update the config, verify it works, then delete the old token.

---

## 4. Canvas LTI RSA Keys Rotation

LTI uses RSA key pairs for secure communication with Canvas LMS.

### Prerequisites
- Access to both Kootenai server and Canvas admin

### Procedure

1. **Generate new RSA key pair:**
   ```bash
   # On your local machine or infra VM
   openssl genrsa -out private-new.pem 2048
   openssl rsa -in private-new.pem -pubout -out public-new.pem
   ```

2. **Update Kootenai keys:**
   ```bash
   ssh labadmin@<INFRA_VM_IP>
   cd ~/kootenai/config/lti

   # Backup existing keys
   cp private.pem private.pem.backup-$(date +%Y%m%d)
   cp public.pem public.pem.backup-$(date +%Y%m%d)

   # Copy new keys (from your local machine)
   # scp private-new.pem labadmin@<INFRA_VM_IP>:~/kootenai/config/lti/private.pem
   # scp public-new.pem labadmin@<INFRA_VM_IP>:~/kootenai/config/lti/public.pem
   ```

3. **Restart the API:**
   ```bash
   cd ~/kootenai/deploy
   docker compose restart api
   ```

4. **Update the Canvas Developer Key:**
   - Log into Canvas as an admin
   - Navigate to Admin > Developer Keys
   - Edit the Kootenai developer key
   - Update the public JWK/public key
   - Save changes

5. **Test LTI launch:**
   - Open a course with a Kootenai assignment
   - Verify the LTI launch works correctly

### Canvas Key Regeneration
If Canvas has RSA key issues (e.g., 512-bit keys error), regenerate Canvas's internal keys:
```bash
cd ~/canvas-lms
docker compose exec -T web rails runner '
  proxy = DynamicSettings.kv_proxy("lti-keys", tree: :store)
  3.times.map { |i| ["jwk-#{%w[past present future][i]}.json", CanvasSecurity::RSAKeyPair.new.to_jwk.to_json] }.to_h
    .then { |keys| proxy.set_keys(keys, global: true) }
  DynamicSettings.reset_cache!
  puts "Keys regenerated!"
'
docker compose restart web
```

---

## 5. NATS Credentials Rotation

If NATS authentication is enabled for the message queue.

### Prerequisites
- NATS authentication must be configured

### Procedure

1. **Generate new credentials:**
   ```bash
   # If using NATS with user/password
   openssl rand -base64 24
   ```

2. **Update NATS configuration:**
   ```bash
   ssh labadmin@<INFRA_VM_IP>
   cd ~/kootenai/deploy

   # Edit .env
   nano .env
   # Update NATS_PASSWORD (if configured)
   ```

3. **Restart NATS and API:**
   ```bash
   docker compose restart nats api
   ```

4. **Verify NATS connectivity:**
   ```bash
   docker compose logs nats | tail -10
   docker compose logs api | grep -i nats | tail -10
   ```

---

## 6. Wazuh API Credentials Rotation

For rotating Wazuh SIEM integration credentials.

### Procedure

1. **Create new API user in Wazuh:**
   ```bash
   # On Wazuh manager
   /var/ossec/bin/wazuh-control stop
   /var/ossec/bin/wazuh-passwords-tool --user wazuh-api-new --password NEW_PASSWORD
   /var/ossec/bin/wazuh-control start
   ```

2. **Update Kootenai configuration:**
   ```bash
   ssh labadmin@<INFRA_VM_IP>
   cd ~/kootenai/deploy
   nano .env
   # Update:
   # WAZUH_API_USER=wazuh-api-new
   # WAZUH_API_PASSWORD=NEW_PASSWORD
   ```

3. **Restart API and verify:**
   ```bash
   docker compose restart api
   docker compose logs api | grep -i wazuh
   ```

---

## Emergency Rotation

In case of a suspected security breach:

1. **Immediately rotate all secrets** following the procedures above
2. **Review audit logs** for suspicious activity:
   ```bash
   docker compose logs api | grep -i "auth\|login\|failed"
   ```
3. **Check for unauthorized access** in the database:
   ```sql
   SELECT * FROM audit_logs
   WHERE event_type IN ('login_failed', 'unauthorized_access')
   AND created_at > NOW() - INTERVAL '24 hours'
   ORDER BY created_at DESC;
   ```
4. **Invalidate all sessions** by rotating JWT_SECRET
5. **Document the incident** for compliance purposes

---

## Automation

For automated rotation, consider:

1. **HashiCorp Vault** for centralized secret management
2. **AWS Secrets Manager** or **GCP Secret Manager** for cloud deployments
3. **GitHub Actions secrets** for CI/CD pipelines

Example Vault integration (future enhancement):
```bash
# Fetch secret from Vault
export JWT_SECRET=$(vault kv get -field=value secret/kootenai/jwt)
```

---

## Verification Checklist

After any secret rotation:

- [ ] Service restarts successfully
- [ ] Health endpoint returns healthy: `curl http://localhost:8080/health`
- [ ] Readiness check passes: `curl http://localhost:8080/ready`
- [ ] Users can log in (if JWT rotated, they need to re-login)
- [ ] Pod operations work (if Proxmox token rotated)
- [ ] LTI launch works (if Canvas keys rotated)
- [ ] Old secrets are securely deleted or archived
- [ ] Backup of .env is stored securely

---

## Related Documentation

- [Proxmox Setup Guide](proxmox-setup.md)
- [Canvas LTI Integration](canvas-lms-integration.md)
- [Wazuh Integration](wazuh-integration.md)
