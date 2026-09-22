// api-load.js - Standard load test for Kootenai API
// Run: k6 run api-load.js

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const apiCalls = new Counter('api_calls');
const dashboardDuration = new Trend('dashboard_duration');
const templatesDuration = new Trend('templates_duration');
const sessionsDuration = new Trend('sessions_duration');

// Test configuration - 50 VUs for 5 minutes
export const options = {
  stages: [
    { duration: '30s', target: 25 },  // Ramp up to 25 VUs
    { duration: '1m', target: 50 },   // Ramp up to 50 VUs
    { duration: '3m', target: 50 },   // Stay at 50 VUs
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],
    errors: ['rate<0.01'],
    dashboard_duration: ['p(95)<400'],
    templates_duration: ['p(95)<300'],
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
  // Simulate realistic user behavior with mixed API calls

  group('Health & Status', function () {
    // Health check (lightweight)
    const healthRes = http.get(`${BASE_URL}/health`, {
      tags: { name: 'health' },
    });
    apiCalls.add(1);

    check(healthRes, {
      'health: status is 200': (r) => r.status === 200,
    }) || errorRate.add(1);

    sleep(0.1);
  });

  group('Public Endpoints', function () {
    // Version
    const versionRes = http.get(`${BASE_URL}/api/v1/version`, {
      tags: { name: 'version' },
    });
    apiCalls.add(1);

    check(versionRes, {
      'version: status is 200': (r) => r.status === 200,
    }) || errorRate.add(1);

    sleep(0.2);

    // Lab templates (most common public query)
    const templatesRes = http.get(`${BASE_URL}/api/v1/templates`, {
      headers: getHeaders(),
      tags: { name: 'templates' },
    });
    templatesDuration.add(templatesRes.timings.duration);
    apiCalls.add(1);

    check(templatesRes, {
      'templates: status is 200 or 401': (r) => r.status === 200 || r.status === 401,
      'templates: response time < 300ms': (r) => r.timings.duration < 300,
    }) || errorRate.add(1);

    sleep(0.3);
  });

  // Authenticated endpoints
  if (AUTH_TOKEN) {
    group('Dashboard', function () {
      // Dashboard - most frequently accessed authenticated endpoint
      const dashboardRes = http.get(`${BASE_URL}/api/v1/dashboard`, {
        headers: getHeaders(),
        tags: { name: 'dashboard' },
      });
      dashboardDuration.add(dashboardRes.timings.duration);
      apiCalls.add(1);

      check(dashboardRes, {
        'dashboard: status is 200': (r) => r.status === 200,
        'dashboard: response time < 400ms': (r) => r.timings.duration < 400,
        'dashboard: has user data': (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.user !== undefined;
          } catch {
            return false;
          }
        },
      }) || errorRate.add(1);

      sleep(0.5);
    });

    group('User Activity', function () {
      // Activity feed
      const activityRes = http.get(`${BASE_URL}/api/v1/activity`, {
        headers: getHeaders(),
        tags: { name: 'activity' },
      });
      apiCalls.add(1);

      check(activityRes, {
        'activity: status is 200': (r) => r.status === 200,
      }) || errorRate.add(1);

      sleep(0.3);
    });

    group('Sessions', function () {
      // List sessions
      const sessionsRes = http.get(`${BASE_URL}/api/v1/sessions?active=true`, {
        headers: getHeaders(),
        tags: { name: 'sessions' },
      });
      sessionsDuration.add(sessionsRes.timings.duration);
      apiCalls.add(1);

      check(sessionsRes, {
        'sessions: status is 200': (r) => r.status === 200,
        'sessions: response time < 400ms': (r) => r.timings.duration < 400,
      }) || errorRate.add(1);

      sleep(0.3);
    });

    group('Pathways', function () {
      // List pathways
      const pathwaysRes = http.get(`${BASE_URL}/api/v1/pathways`, {
        headers: getHeaders(),
        tags: { name: 'pathways' },
      });
      apiCalls.add(1);

      check(pathwaysRes, {
        'pathways: status is 200': (r) => r.status === 200,
      }) || errorRate.add(1);

      sleep(0.3);
    });

    group('Achievements', function () {
      // User achievements
      const achievementsRes = http.get(`${BASE_URL}/api/v1/achievements`, {
        headers: getHeaders(),
        tags: { name: 'achievements' },
      });
      apiCalls.add(1);

      check(achievementsRes, {
        'achievements: status is 200': (r) => r.status === 200,
      }) || errorRate.add(1);

      sleep(0.3);
    });

    group('Leaderboard', function () {
      // Leaderboard (cached, should be fast)
      const leaderboardRes = http.get(`${BASE_URL}/api/v1/leaderboard?limit=10`, {
        headers: getHeaders(),
        tags: { name: 'leaderboard' },
      });
      apiCalls.add(1);

      check(leaderboardRes, {
        'leaderboard: status is 200': (r) => r.status === 200,
        'leaderboard: response time < 300ms': (r) => r.timings.duration < 300,
      }) || errorRate.add(1);

      sleep(0.3);
    });
  }

  // Think time between iterations
  sleep(Math.random() * 2 + 1);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'load-results.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  const { metrics } = data;
  let output = '\n======== LOAD TEST RESULTS ========\n\n';

  output += `Total Requests: ${metrics.http_reqs.values.count}\n`;
  output += `Request Rate: ${Math.round(metrics.http_reqs.values.rate)}/s\n\n`;

  output += 'Response Times (http_req_duration):\n';
  output += `  - avg: ${Math.round(metrics.http_req_duration.values.avg)}ms\n`;
  output += `  - min: ${Math.round(metrics.http_req_duration.values.min)}ms\n`;
  output += `  - max: ${Math.round(metrics.http_req_duration.values.max)}ms\n`;
  output += `  - p90: ${Math.round(metrics.http_req_duration.values['p(90)'])}ms\n`;
  output += `  - p95: ${Math.round(metrics.http_req_duration.values['p(95)'])}ms\n`;
  output += `  - p99: ${Math.round(metrics.http_req_duration.values['p(99)'])}ms\n\n`;

  output += `Error Rate: ${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%\n\n`;

  if (metrics.dashboard_duration) {
    output += 'Dashboard Duration:\n';
    output += `  - avg: ${Math.round(metrics.dashboard_duration.values.avg)}ms\n`;
    output += `  - p95: ${Math.round(metrics.dashboard_duration.values['p(95)'])}ms\n\n`;
  }

  if (metrics.templates_duration) {
    output += 'Templates Duration:\n';
    output += `  - avg: ${Math.round(metrics.templates_duration.values.avg)}ms\n`;
    output += `  - p95: ${Math.round(metrics.templates_duration.values['p(95)'])}ms\n\n`;
  }

  // Check thresholds
  output += 'Threshold Results:\n';
  for (const [name, threshold] of Object.entries(data.thresholds || {})) {
    const status = threshold.ok ? 'PASS' : 'FAIL';
    output += `  ${status} ${name}\n`;
  }

  return output;
}
