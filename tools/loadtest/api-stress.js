// api-stress.js - Stress test for Kootenai API
// Run: k6 run api-stress.js
// This test identifies the breaking point of the system

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const apiCalls = new Counter('api_calls');

// Stress test configuration - ramp up to 200 VUs
export const options = {
  stages: [
    { duration: '2m', target: 50 },   // Ramp up to 50 VUs
    { duration: '3m', target: 50 },   // Stay at 50 VUs
    { duration: '2m', target: 100 },  // Ramp up to 100 VUs
    { duration: '3m', target: 100 },  // Stay at 100 VUs
    { duration: '2m', target: 150 },  // Ramp up to 150 VUs
    { duration: '3m', target: 150 },  // Stay at 150 VUs
    { duration: '2m', target: 200 },  // Ramp up to 200 VUs
    { duration: '3m', target: 200 },  // Stay at 200 VUs (stress!)
    { duration: '3m', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<2000'],
    http_req_failed: ['rate<0.05'],  // Allow up to 5% errors under stress
    errors: ['rate<0.05'],
  },
};

// Configuration from environment
const BASE_URL = __ENV.API_URL || 'http://192.0.2.10:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';

function getHeaders() {
  const headers = { 'Content-Type': 'application/json' };
  if (AUTH_TOKEN) headers['Authorization'] = `Bearer ${AUTH_TOKEN}`;
  return headers;
}

export default function () {
  // Mix of lightweight and heavyweight requests

  // 40% - Health checks (lightweight)
  if (Math.random() < 0.4) {
    const res = http.get(`${BASE_URL}/health`, { tags: { name: 'health' } });
    apiCalls.add(1);
    check(res, { 'health: status is 200': (r) => r.status === 200 }) || errorRate.add(1);
    sleep(0.1);
    return;
  }

  // 30% - Templates (medium weight)
  if (Math.random() < 0.5) {
    const res = http.get(`${BASE_URL}/api/v1/templates`, {
      headers: getHeaders(),
      tags: { name: 'templates' },
    });
    apiCalls.add(1);
    check(res, { 'templates: status is 200/401': (r) => r.status === 200 || r.status === 401 }) || errorRate.add(1);
    sleep(0.2);
    return;
  }

  // 30% - Authenticated endpoints (if available)
  if (AUTH_TOKEN) {
    const endpoint = randomAuthEndpoint();
    const res = http.get(`${BASE_URL}${endpoint.path}`, {
      headers: getHeaders(),
      tags: { name: endpoint.name },
    });
    apiCalls.add(1);
    check(res, { [`${endpoint.name}: status is 200`]: (r) => r.status === 200 }) || errorRate.add(1);
    sleep(0.3);
  } else {
    // Fall back to version endpoint
    const res = http.get(`${BASE_URL}/api/v1/version`, { tags: { name: 'version' } });
    apiCalls.add(1);
    check(res, { 'version: status is 200': (r) => r.status === 200 }) || errorRate.add(1);
    sleep(0.2);
  }

  // Think time
  sleep(Math.random() * 0.5);
}

function randomAuthEndpoint() {
  const endpoints = [
    { path: '/api/v1/dashboard', name: 'dashboard' },
    { path: '/api/v1/sessions?active=true', name: 'sessions' },
    { path: '/api/v1/pathways', name: 'pathways' },
    { path: '/api/v1/achievements', name: 'achievements' },
    { path: '/api/v1/leaderboard?limit=10', name: 'leaderboard' },
    { path: '/api/v1/activity', name: 'activity' },
  ];
  return endpoints[Math.floor(Math.random() * endpoints.length)];
}

export function handleSummary(data) {
  return {
    'stdout': stressSummary(data),
    'stress-results.json': JSON.stringify(data, null, 2),
  };
}

function stressSummary(data) {
  const { metrics } = data;
  let output = '\n======== STRESS TEST RESULTS ========\n\n';

  output += `Peak VUs: 200\n`;
  output += `Total Duration: ~23 minutes\n\n`;

  output += `Total Requests: ${metrics.http_reqs.values.count}\n`;
  output += `Peak Request Rate: ${Math.round(metrics.http_reqs.values.rate)}/s\n\n`;

  output += 'Response Times:\n';
  output += `  - avg: ${Math.round(metrics.http_req_duration.values.avg)}ms\n`;
  output += `  - p90: ${Math.round(metrics.http_req_duration.values['p(90)'])}ms\n`;
  output += `  - p95: ${Math.round(metrics.http_req_duration.values['p(95)'])}ms\n`;
  output += `  - p99: ${Math.round(metrics.http_req_duration.values['p(99)'])}ms\n`;
  output += `  - max: ${Math.round(metrics.http_req_duration.values.max)}ms\n\n`;

  output += `Error Rate: ${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%\n\n`;

  // Analysis
  const p95 = metrics.http_req_duration.values['p(95)'];
  const errorPct = metrics.http_req_failed.values.rate * 100;

  output += 'Analysis:\n';
  if (p95 < 500 && errorPct < 1) {
    output += '  System handled stress well - no significant degradation\n';
  } else if (p95 < 1000 && errorPct < 5) {
    output += '  System showed some degradation under peak load\n';
  } else {
    output += '  System struggled under stress - consider scaling\n';
  }

  return output;
}
