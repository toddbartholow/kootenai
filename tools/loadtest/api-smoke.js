// api-smoke.js - Quick smoke test for Kootenai API
// Run: k6 run api-smoke.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const healthCheckDuration = new Trend('health_check_duration');
const dashboardDuration = new Trend('dashboard_duration');

// Test configuration
export const options = {
  vus: 10,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
    errors: ['rate<0.01'],
  },
};

// Configuration from environment
const BASE_URL = __ENV.API_URL || 'http://192.0.2.10:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';

// Common headers
function getHeaders() {
  const headers = {
    'Content-Type': 'application/json',
  };
  if (AUTH_TOKEN) {
    headers['Authorization'] = `Bearer ${AUTH_TOKEN}`;
  }
  return headers;
}

export default function () {
  // Test 1: Health check endpoint
  const healthRes = http.get(`${BASE_URL}/health`, {
    tags: { name: 'health' },
  });
  healthCheckDuration.add(healthRes.timings.duration);

  check(healthRes, {
    'health: status is 200': (r) => r.status === 200,
    'health: response time < 100ms': (r) => r.timings.duration < 100,
  }) || errorRate.add(1);

  sleep(0.5);

  // Test 2: Ready check endpoint
  const readyRes = http.get(`${BASE_URL}/ready`, {
    tags: { name: 'ready' },
  });

  check(readyRes, {
    'ready: status is 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(0.5);

  // Test 3: Version endpoint
  const versionRes = http.get(`${BASE_URL}/api/v1/version`, {
    tags: { name: 'version' },
  });

  check(versionRes, {
    'version: status is 200': (r) => r.status === 200,
    'version: has version field': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.version !== undefined;
      } catch {
        return false;
      }
    },
  }) || errorRate.add(1);

  sleep(0.5);

  // Test 4: List lab templates (public endpoint)
  const templatesRes = http.get(`${BASE_URL}/api/v1/templates`, {
    headers: getHeaders(),
    tags: { name: 'templates' },
  });

  check(templatesRes, {
    'templates: status is 200 or 401': (r) => r.status === 200 || r.status === 401,
  }) || errorRate.add(1);

  sleep(0.5);

  // Test 5: List pathways (if auth available)
  if (AUTH_TOKEN) {
    const pathwaysRes = http.get(`${BASE_URL}/api/v1/pathways`, {
      headers: getHeaders(),
      tags: { name: 'pathways' },
    });

    check(pathwaysRes, {
      'pathways: status is 200': (r) => r.status === 200,
    }) || errorRate.add(1);

    sleep(0.5);

    // Test 6: Dashboard endpoint
    const dashboardRes = http.get(`${BASE_URL}/api/v1/dashboard`, {
      headers: getHeaders(),
      tags: { name: 'dashboard' },
    });
    dashboardDuration.add(dashboardRes.timings.duration);

    check(dashboardRes, {
      'dashboard: status is 200': (r) => r.status === 200,
      'dashboard: response time < 500ms': (r) => r.timings.duration < 500,
    }) || errorRate.add(1);
  }

  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'smoke-results.json': JSON.stringify(data, null, 2),
  };
}

// Text summary helper
function textSummary(data, options) {
  const { metrics, root_group } = data;
  let output = '\n======== SMOKE TEST RESULTS ========\n\n';

  output += `Duration: ${Math.round(metrics.iteration_duration.values.avg)}ms avg\n`;
  output += `Iterations: ${metrics.iterations.values.count}\n`;
  output += `HTTP Requests: ${metrics.http_reqs.values.count}\n\n`;

  output += 'Response Times:\n';
  output += `  - avg: ${Math.round(metrics.http_req_duration.values.avg)}ms\n`;
  output += `  - p90: ${Math.round(metrics.http_req_duration.values['p(90)'])}ms\n`;
  output += `  - p95: ${Math.round(metrics.http_req_duration.values['p(95)'])}ms\n`;
  output += `  - p99: ${Math.round(metrics.http_req_duration.values['p(99)'])}ms\n\n`;

  output += `Error Rate: ${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%\n`;

  return output;
}
