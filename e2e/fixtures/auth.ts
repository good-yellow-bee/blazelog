import { test as base, expect, Page } from '@playwright/test';

// Test user credentials (must exist in test database)
const TEST_USERS = {
  admin: { username: 'admin', password: 'admin123' },
  operator: { username: 'operator', password: 'operator123' },
  viewer: { username: 'viewer', password: 'viewer123' },
};

type TestUser = keyof typeof TEST_USERS;

// Custom fixture with login helpers
export const test = base.extend<{
  loginAs: (role: TestUser) => Promise<void>;
  loginAsAdmin: () => Promise<void>;
  loginAsOperator: () => Promise<void>;
  loginAsViewer: () => Promise<void>;
}>({
  loginAs: async ({ page }, use) => {
    const loginAs = async (role: TestUser) => {
      const user = TEST_USERS[role];
      await page.goto('/login');
      await page.fill('input[name="username"]', user.username);
      await page.fill('input[name="password"]', user.password);
      await page.click('button[type="submit"]');
      await page.waitForURL('/dashboard');
    };
    await use(loginAs);
  },

  loginAsAdmin: async ({ loginAs }, use) => {
    await use(async () => loginAs('admin'));
  },

  loginAsOperator: async ({ loginAs }, use) => {
    await use(async () => loginAs('operator'));
  },

  loginAsViewer: async ({ loginAs }, use) => {
    await use(async () => loginAs('viewer'));
  },
});

export { expect };

// Helper to check if element is visible
export async function isVisible(page: Page, selector: string): Promise<boolean> {
  const element = page.locator(selector);
  return element.isVisible();
}

// Helper to wait for toast/notification
export async function waitForToast(page: Page, text: string): Promise<void> {
  await expect(page.locator('[role="alert"]').filter({ hasText: text })).toBeVisible();
}

// Helper to fill form fields
export async function fillForm(
  page: Page,
  fields: Record<string, string>
): Promise<void> {
  for (const [name, value] of Object.entries(fields)) {
    const input = page.locator(`input[name="${name}"], textarea[name="${name}"], select[name="${name}"]`);
    const tagName = await input.evaluate((el) => el.tagName.toLowerCase());

    if (tagName === 'select') {
      await input.selectOption(value);
    } else {
      await input.fill(value);
    }
  }
}
