import { test, expect } from '../fixtures/auth';

test.describe('Projects Settings', () => {
  test.beforeEach(async ({ loginAs }) => {
    await loginAs('admin');
  });

  test('navigates to projects page', async ({ page }) => {
    await page.click('text=Settings');
    await page.click('text=Projects');
    await expect(page).toHaveURL('/settings/projects');
    await expect(page.locator('h1')).toContainText('Projects');
  });

  test('shows create button', async ({ page }) => {
    await page.goto('/settings/projects');
    await expect(page.locator('button:has-text("Create Project")')).toBeVisible();
  });

  test('opens create project modal', async ({ page }) => {
    await page.goto('/settings/projects');
    await page.click('button:has-text("Create Project")');
    await expect(page.locator('[role="dialog"]')).toBeVisible();
    await expect(page.locator('text=Create Project')).toBeVisible();
  });

  test('creates new project', async ({ page }) => {
    await page.goto('/settings/projects');
    await page.click('button:has-text("Create Project")');

    await page.fill('input[name="name"]', 'Test Project');
    await page.fill('textarea[name="description"]', 'Test project description');

    await page.click('button:has-text("Save")');
    await expect(page.locator('[role="dialog"]')).not.toBeVisible();
  });

  test('opens members modal', async ({ page }) => {
    await page.goto('/settings/projects');

    // Assuming there's at least one project, click members button
    const membersBtn = page.locator('button[title="Manage Members"]').first();
    if (await membersBtn.isVisible()) {
      await membersBtn.click();
      await expect(page.locator('text=Project Members')).toBeVisible();
    }
  });
});
