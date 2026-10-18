// api-spike.js - Spike test for Kootenai API
// Run: k6 run api-spike.js
// Tests system behavior under sudden traffic spikes

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const apiCalls = new Counter('api_calls');

// Spike test configuration
export const options = {
  stages: [
    { duration: '1m', target: 20 },   // Normal load
    { duration: '30s', target: 20 },  // Stay at normal
    { duration: '10s', target: 500 }, // SPIKE! Sudden jump to 500 VUs
    { duration: '1m', target: 500 },  // Stay at spike level
    { duration: '10s', target: 20 },  // Drop back to normal
    { duration: '1m', target: 20 },   // Recovery period
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'],  // More lenient during spike
    http_req_failed: ['rate<0.10'],     // Allow up to 10% errors during spike
  },
};

// Configuration
const BASE_URL = __ENV.API_URL || 'http://192.0.2.10:8080';
const AUTH_TOKEN = __ENV.AUTH_TOKEN || '';

function getHeaders() {
  const headers = { 'Content-Type': 'application/json' };
  if (AUTH_TOKEN) headers['Authorization'] = `Bearer ${AUTH_TOKEN}`;
  return headers;
}

export default function () {
  // Simulate realistic request distribution during spike

  const rand = Math.random();

  // 50% health checks (lightweight)
  if (rand < 0.5) {
    const res = http.get(`${BASE_URL}/health`, { tags: { name: 'health' } });
    apiCalls.add(1);
    check(res, { 'health: ok': (r) => r.status === 200 }) || errorRate.add(1);
    sleep(0.05);
    return;
  }

  // 30% templates
  if (rand < 0.8) {
    const res = http.get(`${BASE_URL}/api/v1/templates`, {
      headers: getHeaders(),
      tags: { name: 'templates' },
    });
    apiCalls.add(1);
    check(res, { 'templates: ok': (r) => r.status === 200 || r.status === 401 }) || errorRate.add(1);
    sleep(0.1);
    return;
  }

  // 20% dashboard (if authenticated)
  if (AUTH_TOKEN) {
    const res = http.get(`${BASE_URL}/api/v1/dashboard`, {
      headers: getHeaders(),
      tags: { name: 'dashboard' },
    });
    apiCalls.add(1);
    check(res, { 'dashboard: ok': (r) => r.status === 200 }) || errorRate.add(1);
    sleep(0.2);
  } else {
    const res = http.get(`${BASE_URL}/api/v1/version`, { tags: { name: 'version' } });
    apiCalls.add(1);
    check(res, { 'version: ok': (r) => r.status === 200 }) || errorRate.add(1);
    sleep(0.1);
  }
}

export function handleSummary(data) {
  return {
    'stdout': spikeSummary(data),
    'spike-results.json': JSON.stringify(data, null, 2),
  };
}

function spikeSummary(data) {
  const { metrics } = data;
  let output = '\n======== SPIKE TEST RESULTS ========\n\n';

  output += `Spike Level: 500 VUs (from baseline of 20)\n`;
  output += `Spike Duration: 1 minute\n\n`;

  output += `Total Requests: ${metrics.http_reqs.values.count}\n`;
  output += `Peak Request Rate: ${Math.round(metrics.http_reqs.values.rate)}/s\n\n`;

  output += 'Response Times:\n';
  output += `  - avg: ${Math.round(metrics.http_req_duration.values.avg)}ms\n`;
  output += `  - p50: ${Math.round(metrics.http_req_duration.values.med)}ms\n`;
  output += `  - p90: ${Math.round(metrics.http_req_duration.values['p(90)'])}ms\n`;
  output += `  - p95: ${Math.round(metrics.http_req_duration.values['p(95)'])}ms\n`;
  output += `  - p99: ${Math.round(metrics.http_req_duration.values['p(99)'])}ms\n`;
  output += `  - max: ${Math.round(metrics.http_req_duration.values.max)}ms\n\n`;

  output += `Error Rate: ${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%\n\n`;

  // Analysis
  const errorPct = metrics.http_req_failed.values.rate * 100;
  const p95 = metrics.http_req_duration.values['p(95)'];

  output += 'Spike Handling Analysis:\n';
  if (errorPct < 1 && p95 < 500) {
    output += '  Excellent - System absorbed spike with minimal impact\n';
  } else if (errorPct < 5 && p95 < 1000) {
    output += '  Good - System handled spike with graceful degradation\n';
  } else if (errorPct < 10 && p95 < 2000) {
    output += '  Acceptable - System struggled but recovered\n';
  } else {
    output += '  Poor - System failed to handle spike, consider rate limiting or scaling\n';
  }

  return output;
}
