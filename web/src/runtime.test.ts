import { beforeEach, describe, expect, it } from 'vitest';
import { itemUrl, routeUrl } from './runtime';

beforeEach(() => {
  document.head.innerHTML = '';
  document.documentElement.dataset.basePath = '';
  document.documentElement.lang = 'en';
});

describe('runtime URLs', () => {
  it('preserves a configured base path and safely encodes entries', () => {
    document.head.innerHTML = '<meta name="fileharbor-base" content="/fileharbor/">';
    expect(routeUrl('api/directories', { path: 'ops/logs' })).toBe('/fileharbor/api/directories?path=ops%2Flogs');
    expect(itemUrl('d', '')).toBe('/fileharbor');
    expect(itemUrl('d', 'ops/June report')).toBe('/fileharbor/d/ops/June%20report');
  });
});
