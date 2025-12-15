import { test, expect } from '../fixtures/auth';

test.describe('Login', () => {
  test('shows login page', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('text=Sign in to BlazeLog')).toBeVisible();
    await expect(page.locator('input[name="username"]')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
  });

  test('redirects to dashboard after login', async ({ page, loginAs }) => {
    await loginAs('admin');
    await expect(page).toHaveURL('/dashboard');
  });

  test('shows error for invalid credentials', async ({ page }) => {
    await page.goto('/login');
    await page.fill('input[name="username"]', 'invalid');
    await page.fill('input[name="password"]', 'invalid');
    await page.click('button[type="submit"]');

    await expect(page.locator('text=Invalid credentials')).toBeVisible();
  });

  test('logout redirects to login', async ({ page, loginAs }) => {
    await loginAs('admin');
    await page.click('text=Logout');
    await expect(page).toHaveURL('/login');
  });
});
