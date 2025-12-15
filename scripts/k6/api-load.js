// BlazeLog API Load Test
// Run: k6 run --vus 10 --duration 30s scripts/k6/api-load.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const healthLatency = new Trend('health_latency');
const projectsLatency = new Trend('projects_latency');
const usersLatency = new Trend('users_latency');

// Test configuration
export const options = {
  scenarios: {
    // Smoke test - basic functionality
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      startTime: '0s',
      tags: { scenario: 'smoke' },
    },
    // Load test - normal load
    load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 10 },  // Ramp up
        { duration: '1m', target: 10 },   // Stay at 10
        { duration: '30s', target: 50 },  // Ramp up
        { duration: '2m', target: 50 },   // Stay at 50
        { duration: '30s', target: 0 },   // Ramp down
      ],
      startTime: '15s',
      tags: { scenario: 'load' },
    },
    // Spike test - sudden traffic spike
    spike: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 100 }, // Quick ramp to 100
        { duration: '1m', target: 100 },  // Stay at peak
        { duration: '10s', target: 0 },   // Quick ramp down
      ],
      startTime: '5m',
      tags: { scenario: 'spike' },
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200'],  // 95% of requests under 200ms
    http_req_failed: ['rate<0.01'],    // Error rate under 1%
    errors: ['rate<0.01'],             // Custom error rate under 1%
  },
};

// Configuration
const BASE_URL = __ENV.BLAZELOG_URL || 'http://localhost:8080';
let authToken = null;

// Setup - run once before tests
export function setup() {
  // Login to get auth token
  const loginRes = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({
    username: __ENV.BLAZELOG_USER || 'admin',
    password: __ENV.BLAZELOG_PASS || 'admin123',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (loginRes.status === 200) {
    const body = JSON.parse(loginRes.body);
    return { token: body.token };
  }

  console.warn('Login failed, tests will run without auth');
  return { token: null };
}

// Main test function
export default function(data) {
  const headers = data.token
    ? { 'Authorization': `Bearer ${data.token}`, 'Content-Type': 'application/json' }
    : { 'Content-Type': 'application/json' };

  // Test health endpoint (public)
  const healthRes = http.get(`${BASE_URL}/health`);
  healthLatency.add(healthRes.timings.duration);
  check(healthRes, {
    'health status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  // Test health/ready endpoint
  const readyRes = http.get(`${BASE_URL}/health/ready`);
  check(readyRes, {
    'ready status 200': (r) => r.status === 200,
    'ready has status': (r) => r.json('status') !== undefined,
  }) || errorRate.add(1);

  // Test authenticated endpoints (if token available)
  if (data.token) {
    // Projects list
    const projectsRes = http.get(`${BASE_URL}/api/projects`, { headers });
    projectsLatency.add(projectsRes.timings.duration);
    check(projectsRes, {
      'projects status 200': (r) => r.status === 200,
      'projects is array': (r) => Array.isArray(r.json()),
    }) || errorRate.add(1);

    // Users list
    const usersRes = http.get(`${BASE_URL}/api/users`, { headers });
    usersLatency.add(usersRes.timings.duration);
    check(usersRes, {
      'users status 200': (r) => r.status === 200,
      'users is array': (r) => Array.isArray(r.json()),
    }) || errorRate.add(1);

    // Connections list
    const connRes = http.get(`${BASE_URL}/api/connections`, { headers });
    check(connRes, {
      'connections status 200': (r) => r.status === 200,
    }) || errorRate.add(1);

    // Alerts list
    const alertsRes = http.get(`${BASE_URL}/api/alerts`, { headers });
    check(alertsRes, {
      'alerts status 200': (r) => r.status === 200,
    }) || errorRate.add(1);
  }

  sleep(0.1); // Brief pause between iterations
}

// Teardown - cleanup
export function teardown(data) {
  // Nothing to clean up
}
