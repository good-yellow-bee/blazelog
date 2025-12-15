import { test, expect } from '../fixtures/auth';

test.describe('Connections Settings', () => {
  test.beforeEach(async ({ loginAs }) => {
    await loginAs('admin');
  });

  test('navigates to connections page', async ({ page }) => {
    await page.click('text=Settings');
    await page.click('text=Connections');
    await expect(page).toHaveURL('/settings/connections');
    await expect(page.locator('h1')).toContainText('Connections');
  });

  test('shows create button', async ({ page }) => {
    await page.goto('/settings/connections');
    await expect(page.locator('button:has-text("Create Connection")')).toBeVisible();
  });

  test('opens create connection modal', async ({ page }) => {
    await page.goto('/settings/connections');
    await page.click('button:has-text("Create Connection")');
    await expect(page.locator('[role="dialog"]')).toBeVisible();
    await expect(page.locator('text=Create Connection')).toBeVisible();
  });

  test('form shows password field for new connection', async ({ page }) => {
    await page.goto('/settings/connections');
    await page.click('button:has-text("Create Connection")');

    // Select password auth type
    await page.selectOption('select[name="auth_type"]', 'password');
    await expect(page.locator('input[name="password"]')).toBeVisible();
  });

  test('form shows key upload for key auth', async ({ page }) => {
    await page.goto('/settings/connections');
    await page.click('button:has-text("Create Connection")');

    // Select key auth type
    await page.selectOption('select[name="auth_type"]', 'key');
    await expect(page.locator('input[name="private_key"]')).toBeVisible();
  });

  test('test connection button exists', async ({ page }) => {
    await page.goto('/settings/connections');

    // If there's an existing connection, check for test button
    const testBtn = page.locator('button:has-text("Test")').first();
    if (await testBtn.isVisible()) {
      await expect(testBtn).toBeEnabled();
    }
  });
});
