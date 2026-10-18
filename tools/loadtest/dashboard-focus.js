// dashboard-focus.js - Dashboard-focused load test
// Run: k6 run dashboard-focus.js
// Tests dashboard caching performance specifically

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const dashboardCacheHit = new Rate('dashboard_cache_hit');
const dashboardDuration = new Trend('dashboard_duration');
const leaderboardDuration = new Trend('leaderboard_duration');
const firstCallDuration = new Trend('first_call_duration');
const cachedCallDuration = new Trend('cached_call_duration');

// Test configuration - focus on dashboard performance
export const options = {
  scenarios: {
    // Scenario 1: Repeated dashboard requests (should hit cache)
    dashboard_cached: {
      executor: 'constant-vus',
      vus: 20,
      duration: '2m',
      tags: { scenario: 'cached' },
      exec: 'dashboardCachedTest',
    },
    // Scenario 2: Different users (each needs fresh cache)
    dashboard_unique: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 30 },
        { duration: '1m', target: 30 },
        { duration: '30s', target: 0 },
      ],
      startTime: '2m',
      tags: { scenario: 'unique' },
      exec: 'dashboardUniqueTest',
    },
    // Scenario 3: Leaderboard hammering
    leaderboard_stress: {
      executor: 'constant-arrival-rate',
      rate: 50,  // 50 requests per second
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 50,
      startTime: '4m',
      tags: { scenario: 'leaderboard' },
      exec: 'leaderboardTest',
    },
  },
  thresholds: {
    dashboard_duration: ['p(95)<400', 'p(99)<800'],
    leaderboard_duration: ['p(95)<300', 'p(99)<600'],
    cached_call_duration: ['p(95)<100'],  // Cached calls should be very fast
    http_req_failed: ['rate<0.01'],
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

// Test function: Repeated dashboard requests (tests caching)
export function dashboardCachedTest() {
  if (!AUTH_TOKEN) {
    // Without auth, just test health endpoint
    const res = http.get(`${BASE_URL}/health`);
    check(res, { 'health: ok': (r) => r.status === 200 });
    sleep(0.5);
    return;
  }

  // First call (may be cache miss)
  const firstRes = http.get(`${BASE_URL}/api/v1/dashboard`, {
    headers: getHeaders(),
    tags: { name: 'dashboard_first' },
  });
  firstCallDuration.add(firstRes.timings.duration);
  dashboardDuration.add(firstRes.timings.duration);

  check(firstRes, {
    'dashboard first: status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(0.5);

  // Second call (should be cached)
  const cachedRes = http.get(`${BASE_URL}/api/v1/dashboard`, {
    headers: getHeaders(),
    tags: { name: 'dashboard_cached' },
  });
  cachedCallDuration.add(cachedRes.timings.duration);
  dashboardDuration.add(cachedRes.timings.duration);

  // Check if cached response is faster
  const isFaster = cachedRes.timings.duration < firstRes.timings.duration * 0.8;
  dashboardCacheHit.add(isFaster ? 1 : 0);

  check(cachedRes, {
    'dashboard cached: status 200': (r) => r.status === 200,
    'dashboard cached: faster than first': () => isFaster,
  }) || errorRate.add(1);

  sleep(1);
}

// Test function: Different "users" requesting dashboard
export function dashboardUniqueTest() {
  if (!AUTH_TOKEN) {
    const res = http.get(`${BASE_URL}/health`);
    check(res, { 'health: ok': (r) => r.status === 200 });
    sleep(0.5);
    return;
  }

  // Each VU simulates a different user - cache won't help
  const res = http.get(`${BASE_URL}/api/v1/dashboard`, {
    headers: getHeaders(),
    tags: { name: 'dashboard_unique' },
  });
  dashboardDuration.add(res.timings.duration);

  check(res, {
    'dashboard unique: status 200': (r) => r.status === 200,
    'dashboard unique: response < 500ms': (r) => r.timings.duration < 500,
  }) || errorRate.add(1);

  sleep(Math.random() * 2 + 0.5);
}

// Test function: Leaderboard stress test
export function leaderboardTest() {
  const limits = [10, 20, 50, 100];
  const limit = limits[Math.floor(Math.random() * limits.length)];

  const res = http.get(`${BASE_URL}/api/v1/leaderboard?limit=${limit}`, {
    headers: getHeaders(),
    tags: { name: 'leaderboard' },
  });
  leaderboardDuration.add(res.timings.duration);

  check(res, {
    'leaderboard: status 200 or 401': (r) => r.status === 200 || r.status === 401,
    'leaderboard: response < 300ms': (r) => r.timings.duration < 300,
  }) || errorRate.add(1);
}

export function handleSummary(data) {
  return {
    'stdout': dashboardSummary(data),
    'dashboard-results.json': JSON.stringify(data, null, 2),
  };
}

function dashboardSummary(data) {
  const { metrics } = data;
  let output = '\n======== DASHBOARD FOCUS TEST RESULTS ========\n\n';

  output += 'Dashboard Performance:\n';
  if (metrics.dashboard_duration) {
    output += `  - avg: ${Math.round(metrics.dashboard_duration.values.avg)}ms\n`;
    output += `  - p90: ${Math.round(metrics.dashboard_duration.values['p(90)'])}ms\n`;
    output += `  - p95: ${Math.round(metrics.dashboard_duration.values['p(95)'])}ms\n`;
    output += `  - p99: ${Math.round(metrics.dashboard_duration.values['p(99)'])}ms\n\n`;
  }

  output += 'Caching Effectiveness:\n';
  if (metrics.first_call_duration && metrics.cached_call_duration) {
    const firstAvg = Math.round(metrics.first_call_duration.values.avg);
    const cachedAvg = Math.round(metrics.cached_call_duration.values.avg);
    const improvement = ((firstAvg - cachedAvg) / firstAvg * 100).toFixed(1);
    output += `  - First call avg: ${firstAvg}ms\n`;
    output += `  - Cached call avg: ${cachedAvg}ms\n`;
    output += `  - Cache improvement: ${improvement}%\n`;
  }
  if (metrics.dashboard_cache_hit) {
    output += `  - Cache hit rate: ${(metrics.dashboard_cache_hit.values.rate * 100).toFixed(1)}%\n`;
  }
  output += '\n';

  output += 'Leaderboard Performance:\n';
  if (metrics.leaderboard_duration) {
    output += `  - avg: ${Math.round(metrics.leaderboard_duration.values.avg)}ms\n`;
    output += `  - p95: ${Math.round(metrics.leaderboard_duration.values['p(95)'])}ms\n`;
  }
  output += '\n';

  output += `Error Rate: ${(metrics.http_req_failed.values.rate * 100).toFixed(2)}%\n`;

  return output;
}
