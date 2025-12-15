// BlazeLog Auth Stress Test
// Tests rate limiting and lockout behavior
// Run: k6 run scripts/k6/auth-stress.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';

// Custom metrics
const rateLimited = new Counter('rate_limited');
const loginSuccess = new Counter('login_success');
const loginFailed = new Counter('login_failed');
const errorRate = new Rate('errors');

export const options = {
  scenarios: {
    // Test rate limiting on login endpoint
    rate_limit_test: {
      executor: 'constant-arrival-rate',
      rate: 20,              // 20 requests per second
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 10,
      maxVUs: 50,
      tags: { test: 'rate_limit' },
    },
    // Test account lockout
    lockout_test: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 10,       // 10 failed attempts
      startTime: '35s',
      tags: { test: 'lockout' },
    },
    // Test recovery after lockout
    recovery_test: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      startTime: '50s',
      tags: { test: 'recovery' },
    },
  },
  thresholds: {
    'rate_limited': ['count>5'],   // Expect some rate limiting
    'http_req_duration{test:rate_limit}': ['p(95)<500'],
  },
};

const BASE_URL = __ENV.BLAZELOG_URL || 'http://localhost:8080';

// Rate limit test - rapid login attempts
export default function() {
  const scenario = __ENV.scenario || 'rate_limit_test';

  if (scenario === 'rate_limit_test' || __ITER < 100) {
    // Valid credentials - test rate limiting
    testRateLimit();
  } else if (scenario === 'lockout_test') {
    // Invalid credentials - test lockout
    testLockout();
  } else {
    // Recovery test
    testRecovery();
  }
}

function testRateLimit() {
  const res = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({
    username: __ENV.BLAZELOG_USER || 'admin',
    password: __ENV.BLAZELOG_PASS || 'admin123',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status === 429) {
    rateLimited.add(1);
    check(res, {
      'rate limited response': (r) => r.status === 429,
      'has retry-after header': (r) => r.headers['Retry-After'] !== undefined,
    });
  } else if (res.status === 200) {
    loginSuccess.add(1);
    check(res, {
      'login success': (r) => r.status === 200,
      'has token': (r) => r.json('token') !== undefined,
    }) || errorRate.add(1);
  } else {
    loginFailed.add(1);
    check(res, {
      'expected status': (r) => [200, 401, 429].includes(r.status),
    }) || errorRate.add(1);
  }
}

function testLockout() {
  // Use invalid credentials to trigger lockout
  const res = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({
    username: 'lockout-test-user',
    password: 'wrong-password-' + __ITER,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, {
    'failed login': (r) => r.status === 401 || r.status === 423,
  });

  if (res.status === 423) {
    console.log('Account locked out after ' + __ITER + ' attempts');
    check(res, {
      'lockout response': (r) => r.status === 423,
    });
  }

  sleep(0.5);
}

function testRecovery() {
  // Test that valid login works after lockout period
  const res = http.post(`${BASE_URL}/api/auth/login`, JSON.stringify({
    username: __ENV.BLAZELOG_USER || 'admin',
    password: __ENV.BLAZELOG_PASS || 'admin123',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, {
    'login recovers': (r) => r.status === 200 || r.status === 429,
  });

  sleep(1);
}

// Lifecycle hooks
export function setup() {
  console.log('Starting auth stress test against ' + BASE_URL);
}

export function teardown(data) {
  console.log('Auth stress test completed');
}
