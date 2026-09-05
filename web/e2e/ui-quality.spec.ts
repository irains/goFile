import { expect, test } from '@playwright/test';

const session = {
  ok: true,
  session: { username: 'admin', csrf_token: 'csrf', expires_at: '2030-01-01T00:00:00Z' },
  base_path: '',
  locale: 'en',
  capabilities: { browse: true, upload: true, mutate: true, editor_save: true }
};

async function mockWorkspaceApi(page: import('@playwright/test').Page) {
  await page.route('**/api/session', (route) => route.fulfill({ json: session }));
  await page.route(/\/api\/listing/, (route) => route.fulfill({
    json: {
      ok: true,
      directory: {
        path: '',
        parent_path: null,
        listing_token: 'listing-token',
        truncated: false,
        entries: [{
          name: 'sample.txt', path: 'sample.txt', kind: 'file', size_bytes: 6_370_000,
          modified_at: '2026-09-05T10:54:28Z', mode: '-rw-r--r--',
          is_archive: false, previewable: true, editable: true, version: 'v1'
        }]
      }
    }
  }));
  await page.route(/\/api\/directories/, (route) => {
    const path = new URL(route.request().url()).searchParams.get('path') ?? '';
    const dirs = path === '' ? [{ name: 'Documents', path: 'Documents' }] : [{ name: 'Reports', path: `${path}/Reports` }];
    return route.fulfill({ json: { ok: true, path, dirs } });
  });
}

test('login labels stay within the outlined controls after focus', async ({ page }) => {
  await page.goto('/login');
  const username = page.locator('input[name="username"]');
  const label = page.locator('label').filter({ hasText: 'Username' });
  await username.focus();

  const [inputBox, labelBox] = await Promise.all([username.boundingBox(), label.boundingBox()]);
  expect(inputBox).not.toBeNull();
  expect(labelBox).not.toBeNull();
  expect(labelBox!.y).toBeGreaterThan(inputBox!.y - 16);
  expect(labelBox!.y).toBeLessThan(inputBox!.y + inputBox!.height);
});

test('move destination browsing stays in one responsive dialog', async ({ page }) => {
  await mockWorkspaceApi(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  await expect(page.getByRole('cell', { name: '6.37 MB' })).toBeVisible();
  await page.getByRole('button', { name: 'Actions sample.txt' }).click();
  await page.getByRole('menuitem', { name: 'Move' }).click();

  await expect(page.getByRole('dialog')).toHaveCount(1);
  await expect(page.locator('.MuiDialog-paperFullScreen')).toHaveCount(1);
  await expect(page.getByText('Destination', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Documents' }).click();
  await expect(page.getByLabel('Destination: Documents')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Move' })).toBeEnabled();

  const metrics = await page.locator('body').evaluate((body) => ({ scrollWidth: body.scrollWidth, clientWidth: body.clientWidth }));
  expect(metrics.scrollWidth).toBe(metrics.clientWidth);
});
