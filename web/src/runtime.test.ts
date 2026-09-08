import { beforeEach, describe, expect, it } from 'vitest';
import { directoryRoute, editorRoute, encodePathSegments, itemUrl, routeUrl } from './runtime';

beforeEach(() => {
  document.head.innerHTML = '';
  document.documentElement.dataset.basePath = '';
  document.documentElement.lang = 'en';
});

describe('runtime URLs', () => {
  it('preserves a configured base path while keeping SPA routes basename-independent', () => {
    document.head.innerHTML = '<meta name="fileharbor-base" content="/fileharbor/">';
    expect(routeUrl('api/directories', { path: 'ops/logs' })).toBe('/fileharbor/api/directories?path=ops%2Flogs');
    expect(directoryRoute('')).toBe('/');
    expect(directoryRoute('ops/June report')).toBe('/d/ops/June%20report');
    expect(editorRoute('ops/June report.txt')).toBe('/edit/ops/June%20report.txt');
    expect(itemUrl('d', '')).toBe('/fileharbor/');
    expect(itemUrl('d', 'ops/June report')).toBe('/fileharbor/d/ops/June%20report');
  });

  it.each([
    ['', ''],
    ['docs/reports', 'docs/reports'],
    ['资料/报告 一.txt', '%E8%B5%84%E6%96%99/%E6%8A%A5%E5%91%8A%20%E4%B8%80.txt'],
    ['100%/#notes', '100%25/%23notes'],
    ['literal%2Fname/file', 'literal%252Fname/file']
  ])('encodes each path segment exactly once for %s', (input, encoded) => {
    expect(encodePathSegments(input)).toBe(encoded);
    expect(directoryRoute(input)).toBe(encoded ? `/d/${encoded}` : '/');
    expect(editorRoute(input)).toBe(encoded ? `/edit/${encoded}` : '/edit');
  });
});
