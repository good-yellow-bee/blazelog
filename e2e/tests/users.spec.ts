import { test, expect } from '../fixtures/auth';

test.describe('Users Settings', () => {
  test.beforeEach(async ({ loginAs }) => {
    await loginAs('admin');
  });

  test('navigates to users page', async ({ page }) => {
    await page.click('text=Settings');
    await page.click('text=Users');
    await expect(page).toHaveURL('/settings/users');
    await expect(page.locator('h1')).toContainText('Users');
  });

  test('shows create button', async ({ page }) => {
    await page.goto('/settings/users');
    await expect(page.locator('button:has-text("Create User")')).toBeVisible();
  });

  test('opens create user modal', async ({ page }) => {
    await page.goto('/settings/users');
    await page.click('button:has-text("Create User")');
    await expect(page.locator('[role="dialog"]')).toBeVisible();
    await expect(page.locator('text=Create User')).toBeVisible();
  });

  test('create user form has required fields', async ({ page }) => {
    await page.goto('/settings/users');
    await page.click('button:has-text("Create User")');

    await expect(page.locator('input[name="username"]')).toBeVisible();
    await expect(page.locator('input[name="email"]')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
    await expect(page.locator('select[name="role"]')).toBeVisible();
  });

  test('displays role badges', async ({ page }) => {
    await page.goto('/settings/users');

    // Should see at least one role badge in the table
    const roleBadge = page.locator('span:has-text("Admin"), span:has-text("Operator"), span:has-text("Viewer")').first();
    await expect(roleBadge).toBeVisible();
  });

  test('reset password modal available', async ({ page }) => {
    await page.goto('/settings/users');

    // Click reset password on first user (if exists)
    const resetBtn = page.locator('button[title="Reset Password"]').first();
    if (await resetBtn.isVisible()) {
      await resetBtn.click();
      await expect(page.locator('text=Reset Password')).toBeVisible();
    }
  });
});
