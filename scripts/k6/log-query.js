// BlazeLog Log Query Performance Test
// Tests log query endpoints under various load conditions
// Run: k6 run scripts/k6/log-query.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

// Custom metrics
const queryLatency = new Trend('query_latency');
const exportLatency = new Trend('export_latency');
const errorRate = new Rate('errors');

export const options = {
  scenarios: {
    // Simple queries
    simple_queries: {
      executor: 'constant-vus',
      vus: 5,
      duration: '1m',
      tags: { query_type: 'simple' },
    },
    // Complex filtered queries
    filtered_queries: {
      executor: 'constant-vus',
      vus: 3,
      duration: '1m',
      startTime: '30s',
      tags: { query_type: 'filtered' },
    },
    // Time range queries
    time_range_queries: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '20s', target: 10 },
        { duration: '40s', target: 10 },
        { duration: '20s', target: 0 },
      ],
      startTime: '1m30s',
      tags: { query_type: 'time_range' },
    },
  },
  thresholds: {
    'query_latency': ['p(95)<100'],      // 95% under 100ms (target from PLAN.md)
    'export_latency': ['p(95)<500'],     // Export under 500ms
    http_req_failed: ['rate<0.01'],      // Error rate under 1%
  },
};

const BASE_URL = __ENV.BLAZELOG_URL || 'http://localhost:8080';
const WEB_URL = BASE_URL;

// Setup - login and get session
export function setup() {
  // Try API login first
  const apiRes = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({
    username: __ENV.BLAZELOG_USER || 'admin',
    password: __ENV.BLAZELOG_PASS || 'admin123',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  let token = null;
  if (apiRes.status === 200) {
    token = JSON.parse(apiRes.body).token;
  }

  return { token };
}

export default function(data) {
  const scenario = __ENV.K6_SCENARIO_NAME || 'simple_queries';

  switch (scenario) {
    case 'simple_queries':
      testSimpleQueries(data);
      break;
    case 'filtered_queries':
      testFilteredQueries(data);
      break;
    case 'time_range_queries':
      testTimeRangeQueries(data);
      break;
    default:
      testSimpleQueries(data);
  }
}

function testSimpleQueries(data) {
  const headers = data.token
    ? { 'Authorization': `Bearer ${data.token}` }
    : {};

  // Query logs data endpoint
  const start = Date.now();
  const res = http.get(`${WEB_URL}/logs/data?limit=100`, { headers });
  queryLatency.add(Date.now() - start);

  check(res, {
    'simple query status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(0.5);
}

function testFilteredQueries(data) {
  const headers = data.token
    ? { 'Authorization': `Bearer ${data.token}` }
    : {};

  // Various filter combinations
  const filters = [
    'level=error',
    'level=warn&limit=50',
    'source=nginx',
    'level=error&source=nginx',
  ];

  const filter = filters[Math.floor(Math.random() * filters.length)];

  const start = Date.now();
  const res = http.get(`${WEB_URL}/logs/data?${filter}`, { headers });
  queryLatency.add(Date.now() - start);

  check(res, {
    'filtered query status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(0.3);
}

function testTimeRangeQueries(data) {
  const headers = data.token
    ? { 'Authorization': `Bearer ${data.token}` }
    : {};

  // Query last 24 hours (target metric from PLAN.md)
  const now = new Date();
  const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000);

  const params = new URLSearchParams({
    from: yesterday.toISOString(),
    to: now.toISOString(),
    limit: '1000',
  });

  const start = Date.now();
  const res = http.get(`${WEB_URL}/logs/data?${params}`, { headers });
  const latency = Date.now() - start;
  queryLatency.add(latency);

  // Check against PLAN.md target: <100ms for last 24h queries
  check(res, {
    'time range query status 200': (r) => r.status === 200,
    'time range query under 100ms': () => latency < 100,
  }) || errorRate.add(1);

  sleep(0.2);
}

// Export test
export function testExport(data) {
  const headers = data.token
    ? { 'Authorization': `Bearer ${data.token}` }
    : {};

  const start = Date.now();
  const res = http.get(`${WEB_URL}/logs/export?format=csv&limit=1000`, { headers });
  exportLatency.add(Date.now() - start);

  check(res, {
    'export status 200': (r) => r.status === 200,
    'export has content': (r) => r.body.length > 0,
  }) || errorRate.add(1);
}
