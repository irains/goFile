import { expect, test } from '@playwright/test';

const username = process.env.FILEHARBOR_E2E_USERNAME;
const password = process.env.FILEHARBOR_E2E_PASSWORD;
const hasServiceConfiguration = Boolean(process.env.PLAYWRIGHT_BASE_URL && username && password);

// Service login submits a CI-only password. Never retain Playwright media because it
// could reproduce the interaction or serialize request data.
test.use({ trace: 'off', video: 'off', screenshot: 'off' });

test.describe('Go service integration', () => {
  test.skip(!hasServiceConfiguration, 'requires PLAYWRIGHT_BASE_URL and ephemeral service test credentials');

  test('authenticates a protected deep link, navigates folders, and creates a folder', async ({ page }) => {
    const folderName = `playwright-created-${Date.now()}`;

    await page.goto('/d/service-fixture');
    await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible();
    await page.getByLabel('Username').fill(username!);
    await page.getByLabel('Password').fill(password!);
    await page.getByRole('button', { name: 'Sign in' }).click();

    await expect(page).toHaveURL(/\/d\/service-fixture$/);
    await expect(page.getByRole('button', { name: 'seed.txt', exact: true })).toBeVisible();

    await page.getByRole('link', { name: 'Root' }).click();
    await expect(page).toHaveURL(/\/$/);
    await page.getByRole('link', { name: 'service-fixture' }).click();
    await expect(page).toHaveURL(/\/d\/service-fixture$/);

    await page.getByRole('button', { name: 'New folder' }).click();
    const dialog = page.getByRole('dialog', { name: 'Create folder' });
    await dialog.getByLabel('Folder name').fill(folderName);
    await dialog.getByRole('button', { name: 'Confirm' }).click();
    await expect(page.getByRole('link', { name: folderName })).toBeVisible();

    await page.reload();
    await expect(page.getByRole('link', { name: folderName })).toBeVisible();
  });
});
