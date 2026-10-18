# Load Testing

This directory contains load testing scripts for the Kootenai API using [k6](https://k6.io/).

## Prerequisites

Install k6:

```bash
# macOS
brew install k6

# Linux (Debian/Ubuntu)
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6

# Docker
docker pull grafana/k6
```

## Configuration

Set environment variables before running tests:

```bash
export API_URL=http://<INFRA_IP>:8080
export AUTH_TOKEN=your-jwt-token  # Optional, for authenticated tests
```

## Test Scripts

| Script | Description |
|--------|-------------|
| `api-smoke.js` | Quick smoke test (10 VUs, 30s) |
| `api-load.js` | Standard load test (50 VUs, 5min) |
| `api-stress.js` | Stress test with ramping (up to 200 VUs) |
| `api-spike.js` | Spike test (sudden burst to 500 VUs) |
| `dashboard-focus.js` | Dashboard-specific load test |

## Running Tests

### Smoke Test (Quick Verification)
```bash
k6 run api-smoke.js
```

### Load Test (Standard)
```bash
k6 run api-load.js
```

### Stress Test
```bash
k6 run api-stress.js
```

### Spike Test
```bash
k6 run api-spike.js
```

### Custom Options
```bash
# Override VUs and duration
k6 run --vus 100 --duration 10m api-load.js

# Output to JSON
k6 run --out json=results.json api-load.js

# Output to InfluxDB (for Grafana dashboards)
k6 run --out influxdb=http://localhost:8086/k6 api-load.js
```

## Performance Thresholds

The tests are configured with the following thresholds:

| Metric | Threshold | Description |
|--------|-----------|-------------|
| `http_req_duration` | p(95) < 500ms | 95% of requests under 500ms |
| `http_req_duration` | p(99) < 1000ms | 99% of requests under 1s |
| `http_req_failed` | rate < 0.01 | Less than 1% error rate |
| `http_reqs` | rate > 100 | At least 100 req/sec |

## Test Scenarios

### 1. Smoke Test
- 10 virtual users
- 30 second duration
- Verifies basic functionality under minimal load

### 2. Load Test
- 50 virtual users
- 5 minute duration
- Simulates normal production load

### 3. Stress Test
- Ramps from 0 to 200 VUs over 20 minutes
- Identifies breaking points
- Stages: ramp-up, sustain, ramp-down

### 4. Spike Test
- Normal load with sudden spike to 500 VUs
- Tests auto-scaling and recovery
- Measures time to recover

## Interpreting Results

After running a test, k6 outputs:

```
✓ status is 200
✓ response time < 500ms

checks.........................: 100.00% ✓ 15000 ✗ 0
http_req_duration..............: avg=45.2ms  min=12ms  med=38ms  max=890ms  p(90)=78ms  p(95)=120ms
http_reqs......................: 15000   250.00/s
```

Key metrics to monitor:
- **http_req_duration**: Response times (lower is better)
- **http_req_failed**: Error rate (lower is better)
- **http_reqs**: Throughput (higher is better)
- **checks**: Assertion pass rate (100% is ideal)

## Integration with CI/CD

Add to your GitHub Actions workflow:

```yaml
- name: Run k6 load test
  uses: grafana/k6-action@v0.3.1
  with:
    filename: tools/loadtest/api-smoke.js
    flags: --out json=results.json
```
