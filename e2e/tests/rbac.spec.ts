import { test, expect } from '../fixtures/auth';

test.describe('RBAC - Role-Based Access Control', () => {
  test.describe('Viewer Role', () => {
    test.beforeEach(async ({ loginAs }) => {
      await loginAs('viewer');
    });

    test('can access dashboard', async ({ page }) => {
      await expect(page).toHaveURL('/dashboard');
    });

    test('can access logs', async ({ page }) => {
      await page.goto('/logs');
      await expect(page.locator('h1')).toContainText('Logs');
    });

    test('can access alerts settings', async ({ page }) => {
      await page.goto('/settings/alerts');
      await expect(page.locator('h1')).toContainText('Alerts');
    });

    test('cannot access projects settings', async ({ page }) => {
      const response = await page.goto('/settings/projects');
      // Should be forbidden or redirected
      expect(response?.status()).toBe(403);
    });

    test('cannot access connections settings', async ({ page }) => {
      const response = await page.goto('/settings/connections');
      expect(response?.status()).toBe(403);
    });

    test('cannot access users settings', async ({ page }) => {
      const response = await page.goto('/settings/users');
      expect(response?.status()).toBe(403);
    });

    test('sidebar hides admin-only links', async ({ page }) => {
      await page.goto('/dashboard');
      await page.click('text=Settings');

      await expect(page.locator('a[href="/settings/alerts"]')).toBeVisible();
      await expect(page.locator('a[href="/settings/projects"]')).not.toBeVisible();
      await expect(page.locator('a[href="/settings/connections"]')).not.toBeVisible();
      await expect(page.locator('a[href="/settings/users"]')).not.toBeVisible();
    });
  });

  test.describe('Operator Role', () => {
    test.beforeEach(async ({ loginAs }) => {
      await loginAs('operator');
    });

    test('can access alerts settings', async ({ page }) => {
      await page.goto('/settings/alerts');
      await expect(page.locator('h1')).toContainText('Alerts');
    });

    test('can create alerts', async ({ page }) => {
      await page.goto('/settings/alerts');
      await expect(page.locator('button:has-text("Create Alert")')).toBeVisible();
    });

    test('cannot access projects settings', async ({ page }) => {
      const response = await page.goto('/settings/projects');
      expect(response?.status()).toBe(403);
    });

    test('cannot access users settings', async ({ page }) => {
      const response = await page.goto('/settings/users');
      expect(response?.status()).toBe(403);
    });
  });

  test.describe('Admin Role', () => {
    test.beforeEach(async ({ loginAs }) => {
      await loginAs('admin');
    });

    test('can access all settings pages', async ({ page }) => {
      await page.goto('/settings/alerts');
      await expect(page.locator('h1')).toContainText('Alerts');

      await page.goto('/settings/projects');
      await expect(page.locator('h1')).toContainText('Projects');

      await page.goto('/settings/connections');
      await expect(page.locator('h1')).toContainText('Connections');

      await page.goto('/settings/users');
      await expect(page.locator('h1')).toContainText('Users');
    });

    test('sidebar shows all admin links', async ({ page }) => {
      await page.goto('/dashboard');
      await page.click('text=Settings');

      await expect(page.locator('a[href="/settings/alerts"]')).toBeVisible();
      await expect(page.locator('a[href="/settings/projects"]')).toBeVisible();
      await expect(page.locator('a[href="/settings/connections"]')).toBeVisible();
      await expect(page.locator('a[href="/settings/users"]')).toBeVisible();
    });

    test('can delete alerts', async ({ page }) => {
      await page.goto('/settings/alerts');
      // Admin should see delete buttons (if alerts exist)
      const deleteBtn = page.locator('button[title="Delete"]').first();
      if (await deleteBtn.isVisible()) {
        await expect(deleteBtn).toBeEnabled();
      }
    });
  });
});
