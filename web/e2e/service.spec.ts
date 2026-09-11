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

    await page.getByRole('button', { name: 'Settings' }).click();
    await page.getByRole('radio', { name: 'Graphite' }).click();
    await expect(page.locator('html')).toHaveAttribute('data-fileharbor-accent', 'graphite');
    await page.getByRole('button', { name: 'Close' }).click();

    await page.reload();
    await page.getByRole('button', { name: 'Settings' }).click();
    await expect(page.getByRole('radio', { name: 'Graphite' })).toBeChecked();
    await page.getByRole('button', { name: 'Close' }).click();
    await expect(page.getByRole('link', { name: folderName })).toBeVisible();
  });

  test('moves a service file to the recycle bin, survives reload, and restores it', async ({ page }) => {
    const recycleBin = page.getByRole('dialog').filter({ has: page.getByRole('heading', { name: 'Recycle bin' }) });
    const restoredName = `playwright-recycle-${Date.now()}.txt`;
    const recycleBinEntry = recycleBin.getByText(restoredName, { exact: true });
    await page.goto('/d/service-fixture');
    await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible();
    await page.getByLabel('Username').fill(username!);
    await page.getByLabel('Password').fill(password!);
    await page.getByRole('button', { name: 'Sign in' }).click();

    await page.getByRole('button', { name: 'New file' }).click();
    const newFileDialog = page.getByRole('dialog', { name: 'Create file' });
    await newFileDialog.getByLabel('File name').fill(restoredName);
    const createResponse = page.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname === '/do/newfile');
    await newFileDialog.getByRole('button', { name: 'Confirm' }).click();
    const created = await createResponse;
    expect(created.status()).toBe(200);
    expect(await created.json()).toMatchObject({ ok: true });
    await expect(page.getByRole('button', { name: restoredName, exact: true })).toBeVisible();
    await page.getByRole('button', { name: `Actions ${restoredName}` }).click();
    await page.getByRole('menuitem', { name: 'Move to recycle bin' }).click();
    const confirmation = page.getByRole('dialog', { name: `Move ${restoredName} to the recycle bin?` });
    const moveResponse = page.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname === '/do/rm');
    await confirmation.getByRole('button', { name: 'Move to recycle bin' }).click();
    const moved = await moveResponse;
    expect(moved.status()).toBe(200);
    expect(await moved.json()).toMatchObject({ ok: true });
    await expect(confirmation).toHaveCount(0);
    // An exiting modal hides the workspace from role locators before deletion finishes.
    await expect(page.getByRole('button', { name: restoredName, exact: true, includeHidden: true })).toHaveCount(0);

    await page.getByRole('button', { name: 'Recycle bin' }).click();
    await expect(recycleBin).toBeVisible();
    await expect(recycleBinEntry).toBeVisible();
    await page.reload();
    await page.getByRole('button', { name: 'Recycle bin' }).click();
    await expect(recycleBin).toBeVisible();
    await expect(recycleBinEntry).toBeVisible();
    const recycledRow = recycleBin.getByRole('listitem').filter({ has: page.getByText(restoredName, { exact: true }) });
    const restoreResponse = page.waitForResponse((response) => response.request().method() === 'POST' && /\/api\/trash\/[^/]+\/restore$/.test(new URL(response.url()).pathname));
    await recycledRow.getByRole('button', { name: 'Restore', exact: true }).click();
    const restored = await restoreResponse;
    expect(restored.status()).toBe(200);
    expect(await restored.json()).toMatchObject({ ok: true, path: `service-fixture/${restoredName}` });
    await expect(recycleBinEntry).toHaveCount(0);
    await recycleBin.getByRole('button', { name: 'Close', exact: true }).click();
    await expect(recycleBin).toHaveCount(0);
    await expect(page.getByRole('button', { name: restoredName, exact: true })).toBeVisible();
    await page.reload();
    await expect(page.getByRole('button', { name: restoredName, exact: true })).toBeVisible();
  });
});
