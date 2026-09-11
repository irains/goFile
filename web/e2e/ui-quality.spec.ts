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

test('operation dialog labels stay inside the content area', async ({ page }) => {
  await mockWorkspaceApi(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByRole('radio', { name: '简体中文' }).click();
  await page.getByRole('button', { name: '关闭' }).click();
  await page.getByRole('button', { name: '新建文件夹' }).click();

  const dialog = page.getByRole('dialog');
  const input = dialog.getByRole('textbox', { name: '文件夹名称' });
  const label = dialog.locator(`label[for="${await input.getAttribute('id')}"]`);
  const content = dialog.locator('.MuiDialogContent-root');

  await input.focus();
  await expect(input).toBeFocused();
  await expect(label).toHaveClass(/MuiInputLabel-shrink/);

  const [inputBox, labelBox, contentBox] = await Promise.all([
    input.boundingBox(),
    label.boundingBox(),
    content.boundingBox()
  ]);
  expect(inputBox).not.toBeNull();
  expect(labelBox).not.toBeNull();
  expect(contentBox).not.toBeNull();
  expect(labelBox!.y).toBeGreaterThanOrEqual(contentBox!.y);
  expect(labelBox!.y).toBeLessThan(inputBox!.y + inputBox!.height);
});

test('folder navigation is SPA-based and supports browser history', async ({ page }) => {
  let sessionRequests = 0;
  const requestedPaths: string[] = [];
  await page.route('**/api/session', (route) => { sessionRequests += 1; return route.fulfill({ json: session }); });
  await page.route(/\/api\/listing/, (route) => {
    const path = new URL(route.request().url()).searchParams.get('path') ?? '';
    requestedPaths.push(path);
    const entries = path === ''
      ? [{ name: 'Docs #1', path: 'Docs #1', kind: 'directory', size_bytes: 0, modified_at: '2026-09-05T10:54:28Z', mode: 'drwxr-xr-x', is_archive: false, previewable: false, editable: false, version: 'd1' }]
      : [{ name: 'Nested folder', path: `${path}/Nested folder`, kind: 'directory', size_bytes: 0, modified_at: '2026-09-05T10:54:28Z', mode: 'drwxr-xr-x', is_archive: false, previewable: false, editable: false, version: 'd2' }];
    return route.fulfill({ json: { ok: true, directory: { path, parent_path: path ? '' : null, listing_token: `token-${path}`, truncated: false, entries } } });
  });

  await page.goto('/');
  const appBar = page.locator('header');
  await expect(appBar).toBeVisible();
  const appBarElement = await appBar.elementHandle();

  await page.getByRole('link', { name: 'Docs #1' }).click();
  await expect(page).toHaveURL(/\/d\/Docs%20%231$/);
  await expect(page.getByRole('link', { name: 'Nested folder' })).toBeVisible();
  expect(await page.locator('header').evaluate((node, original) => node === original, appBarElement)).toBe(true);
  expect(sessionRequests).toBeGreaterThan(0);
  expect(requestedPaths).toEqual(['', 'Docs #1']);

  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('link', { name: 'Docs #1' })).toBeVisible();
  await page.goForward();
  await expect(page).toHaveURL(/\/d\/Docs%20%231$/);
  await page.getByRole('link', { name: 'Root' }).click();
  await expect(page).toHaveURL(/\/$/);
});

test('editor route blocks dirty navigation and closes to its origin', async ({ page }) => {
  await page.route('**/api/session', (route) => route.fulfill({ json: session }));
  await page.route(/\/api\/listing/, (route) => {
    const path = new URL(route.request().url()).searchParams.get('path') ?? '';
    return route.fulfill({ json: { ok: true, directory: { path, parent_path: null, listing_token: `token-${path}`, truncated: false, entries: [{ name: 'sample.txt', path: 'sample.txt', kind: 'file', size_bytes: 6, modified_at: '2026-09-05T10:54:28Z', mode: '-rw-r--r--', is_archive: false, previewable: true, editable: true, version: 'v1' }] } } });
  });
  await page.route(/\/api\/editor\/content/, (route) => route.fulfill({ json: { ok: true, editor: { path: 'sample.txt', name: 'sample.txt', content: 'original', size_bytes: 8, modified_at: '2026-09-05T10:54:28Z', extension: 'txt', version: 'v1' } } }));

  await page.goto('/');
  await page.getByRole('button', { name: 'Actions sample.txt' }).click();
  await page.getByRole('menuitem', { name: 'Edit' }).click();
  await expect(page).toHaveURL(/\/edit\/sample\.txt$/);
  await expect(page.getByText('sample.txt', { exact: true }).first()).toBeVisible();

  await expect(page.locator('.ace_text-input')).toBeVisible();
  await page.locator('.ace_text-input').pressSequentially(' changed');
  await expect(page.getByText('Unsaved changes')).toBeVisible();
  await page.goBack();
  await expect(page.getByRole('dialog', { name: 'Discard unsaved changes?' })).toBeVisible();
  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page).toHaveURL(/\/edit\/sample\.txt$/);

  await page.goBack();
  await page.getByRole('button', { name: 'Discard' }).click();
  await expect(page).toHaveURL(/\/$/);
});

test('refresh folder is visible, responsive, and updates the listing', async ({ page }) => {
  let listingRequests = 0;
  let releaseRefresh: (() => void) | undefined;
  await page.route('**/api/session', (route) => route.fulfill({ json: session }));
  await page.route(/\/api\/listing/, async (route) => {
    listingRequests += 1;
    if (listingRequests === 2) await new Promise<void>((resolve) => { releaseRefresh = resolve; });
    const refreshed = listingRequests > 1;
    return route.fulfill({
      json: {
        ok: true,
        directory: {
          path: '', parent_path: null, listing_token: `listing-token-${listingRequests}`, truncated: false,
          entries: [{
            name: refreshed ? 'refreshed.txt' : 'sample.txt', path: refreshed ? 'refreshed.txt' : 'sample.txt', kind: 'file', size_bytes: 12,
            modified_at: '2026-09-05T10:54:28Z', mode: '-rw-r--r--', is_archive: false, previewable: true, editable: true, version: `v${listingRequests}`
          }]
        }
      }
    });
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  const refresh = page.getByRole('button', { name: 'Refresh' });
  await expect(refresh).toBeVisible();
  await expect(page.locator('header').getByRole('button', { name: 'Refresh' })).toHaveCount(0);
  await refresh.click();
  await expect(page.getByRole('button', { name: 'Refreshing…' })).toBeDisabled();
  await expect(page.getByText('sample.txt')).toBeVisible();
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(1);

  releaseRefresh?.();
  await expect(page.getByText('refreshed.txt')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toBeEnabled();
  expect(listingRequests).toBe(2);

  const metrics = await page.locator('body').evaluate((body) => ({ scrollWidth: body.scrollWidth, clientWidth: body.clientWidth }));
  expect(metrics.scrollWidth).toBe(metrics.clientWidth);
});

test('settings switches complete palettes, persists choices, and fits mobile two-column controls', async ({ page }) => {
  await mockWorkspaceApi(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  await page.getByRole('button', { name: 'Settings' }).click();
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
  const paletteGroup = page.getByRole('radiogroup', { name: 'Color palette' });
  await expect(paletteGroup.getByRole('radio')).toHaveCount(12);
  const forestCanvas = await page.locator('html').evaluate((root) => getComputedStyle(root).backgroundColor);
  await page.getByRole('radio', { name: 'Graphite' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-fileharbor-accent', 'graphite');
  await expect(page.getByRole('radio', { name: 'Graphite' })).toBeChecked();
  const graphiteCanvas = await page.locator('html').evaluate((root) => getComputedStyle(root).backgroundColor);
  expect(graphiteCanvas).not.toBe(forestCanvas);
  await expect(page.locator('header').filter({ hasText: 'FileHarbor' })).toHaveCSS('background-color', 'rgb(229, 233, 233)');

  await page.reload();
  await page.getByRole('button', { name: 'Settings' }).click();
  await expect(page.getByRole('radio', { name: 'Graphite' })).toBeChecked();
  await page.getByRole('radio', { name: 'Light' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-fileharbor-accent', 'graphite');

  await page.getByRole('radio', { name: 'Forest' }).focus();
  await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('radio', { name: 'Harbor' })).toBeChecked();
  await page.getByRole('radio', { name: '简体中文' }).click();
  await expect(page.getByRole('radiogroup', { name: '配色方案' }).getByRole('radio')).toHaveCount(12);
  await expect(page.getByRole('radio', { name: '石墨' })).toBeVisible();

  const metrics = await page.locator('body').evaluate((body) => ({ scrollWidth: body.scrollWidth, clientWidth: body.clientWidth }));
  expect(metrics.scrollWidth).toBe(metrics.clientWidth);
});

test('move destination browsing stays in one responsive dialog', async ({ page }) => {
  await mockWorkspaceApi(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  await expect(page.getByText('6.37 MB')).toBeVisible();
  await page.getByRole('button', { name: 'Actions sample.txt' }).click();
  await page.getByRole('menuitem', { name: 'Move', exact: true }).click();

  await expect(page.getByRole('dialog')).toHaveCount(1);
  await expect(page.locator('.MuiDialog-paperFullScreen')).toHaveCount(1);
  await expect(page.getByText('Destination', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Documents' }).click();
  await expect(page.getByLabel('Destination: Documents')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Move' })).toBeEnabled();

  const metrics = await page.locator('body').evaluate((body) => ({ scrollWidth: body.scrollWidth, clientWidth: body.clientWidth }));
  expect(metrics.scrollWidth).toBe(metrics.clientWidth);
});

test('recycle bin moves, confirms permanent actions, restores, and fits mobile', async ({ page }) => {
  let inWorkspace = true;
  const recycled = {
    id: '0123456789abcdef0123456789abcdef',
    name: 'sample.txt', original_path: 'sample.txt', kind: 'file', size_bytes: 6_370_000,
    deleted_at: '2026-09-11T10:54:28Z'
  };
  let trashEntries: typeof recycled[] = [];
  const longName = 'this-is-an-intentionally-long-mobile-filename-for-line-clamp-validation.txt';
  await page.route('**/api/session', (route) => route.fulfill({ json: session }));
  await page.route(/\/api\/listing/, (route) => route.fulfill({
    json: {
      ok: true,
      directory: {
        path: '', parent_path: null, listing_token: 'listing-token', truncated: false,
        entries: inWorkspace ? [{
          name: 'sample.txt', path: 'sample.txt', kind: 'file', size_bytes: 6_370_000,
          modified_at: '2026-09-05T10:54:28Z', mode: '-rw-r--r--',
          is_archive: false, previewable: true, editable: true, version: 'v1'
        }, {
          name: longName, path: longName, kind: 'file', size_bytes: 12,
          modified_at: '2026-09-05T10:54:28Z', mode: '-rw-r--r--',
          is_archive: false, previewable: true, editable: true, version: 'v2'
        }] : []
      }
    }
  }));
  await page.route(/\/api\/trash(?:\?.*)?$/, async (route) => {
    await route.fulfill({ json: { ok: true, entries: trashEntries } });
  });
  await page.route('**/do/rm', (route) => {
    inWorkspace = false;
    trashEntries = [recycled];
    return route.fulfill({ json: { ok: true, entry: recycled } });
  });
  await page.route(/\/api\/trash\/[^/]+\/restore$/, (route) => {
    inWorkspace = true;
    trashEntries = [];
    return route.fulfill({ json: { ok: true, path: 'sample.txt' } });
  });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  const longNameButton = page.getByRole('button', { name: longName, exact: true });
  await expect(longNameButton).toBeVisible();
  await expect(longNameButton).toHaveAttribute('title', longName);
  const longNameBox = await longNameButton.boundingBox();
  expect(longNameBox).not.toBeNull();
  expect(longNameBox!.height).toBeLessThanOrEqual(48);
  const workspaceActions = page.getByRole('button', { name: 'Refresh', exact: true }).locator('xpath=..');
  await expect(workspaceActions).toHaveCSS('display', 'grid');
  await expect(workspaceActions).toHaveCSS('grid-template-columns', /px.*px/);
  const sampleSelection = page.locator('input[type="checkbox"]').nth(1);
  await sampleSelection.check();
  await expect(page.getByText('1 selected')).toBeVisible();
  const batchActions = page.getByRole('button', { name: 'Move', exact: true }).locator('xpath=..');
  await expect(batchActions).toHaveCSS('display', 'grid');
  await sampleSelection.uncheck();
  await page.getByRole('button', { name: 'Actions sample.txt' }).click();
  await page.getByRole('menuitem', { name: 'Move to recycle bin' }).click();
  await page.getByRole('dialog', { name: 'Move sample.txt to the recycle bin?' }).getByRole('button', { name: 'Move to recycle bin' }).click();
  await expect(page.getByText('Nothing here yet')).toBeVisible();

  await page.getByRole('button', { name: 'Recycle bin' }).click();
  await expect(page.getByText('sample.txt', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Permanently delete sample.txt' }).click();
  const permanentDialog = page.getByRole('dialog', { name: 'Permanently delete sample.txt?' });
  const permanentConfirm = permanentDialog.getByRole('button', { name: 'Permanently delete' });
  await expect(permanentConfirm).toBeDisabled();
  await permanentDialog.getByRole('textbox', { name: 'Type DELETE to confirm.' }).fill('DELETE');
  await expect(permanentConfirm).toBeEnabled();
  await permanentDialog.getByRole('button', { name: 'Cancel' }).click();

  await page.getByRole('button', { name: 'Restore' }).click();
  await expect(page.getByText('sample.txt', { exact: true })).toBeVisible();
  await expect(page.getByText('Nothing here yet')).not.toBeVisible();
  const metrics = await page.locator('body').evaluate((body) => ({ scrollWidth: body.scrollWidth, clientWidth: body.clientWidth }));
  expect(metrics.scrollWidth).toBe(metrics.clientWidth);
});
