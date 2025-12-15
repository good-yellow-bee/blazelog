import { test, expect } from '../fixtures/auth';

test.describe('Alerts Settings', () => {
  test.beforeEach(async ({ loginAs }) => {
    await loginAs('admin');
  });

  test('navigates to alerts page', async ({ page }) => {
    await page.click('text=Settings');
    await page.click('text=Alerts');
    await expect(page).toHaveURL('/settings/alerts');
    await expect(page.locator('h1')).toContainText('Alerts');
  });

  test('viewer can access alerts page', async ({ page, loginAs }) => {
    await page.context().clearCookies();
    await loginAs('viewer');
    await page.goto('/settings/alerts');
    await expect(page.locator('h1')).toContainText('Alerts');
  });

  test('shows create button for operator', async ({ page, loginAs }) => {
    await page.context().clearCookies();
    await loginAs('operator');
    await page.goto('/settings/alerts');
    await expect(page.locator('button:has-text("Create Alert")')).toBeVisible();
  });

  test('viewer cannot see create button', async ({ page, loginAs }) => {
    await page.context().clearCookies();
    await loginAs('viewer');
    await page.goto('/settings/alerts');
    await expect(page.locator('button:has-text("Create Alert")')).not.toBeVisible();
  });

  test('opens create alert modal', async ({ page }) => {
    await page.goto('/settings/alerts');
    await page.click('button:has-text("Create Alert")');
    await expect(page.locator('[role="dialog"]')).toBeVisible();
    await expect(page.locator('text=Create Alert')).toBeVisible();
  });

  test('creates new alert', async ({ page }) => {
    await page.goto('/settings/alerts');
    await page.click('button:has-text("Create Alert")');

    await page.fill('input[name="name"]', 'Test Alert');
    await page.fill('textarea[name="description"]', 'Test description');
    await page.selectOption('select[name="type"]', 'threshold');
    await page.selectOption('select[name="severity"]', 'high');

    await page.click('button:has-text("Save")');

    // Wait for modal to close and table to update
    await expect(page.locator('[role="dialog"]')).not.toBeVisible();
  });
});
