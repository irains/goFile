import { describe, expect, it } from 'vitest';
import type { FileEntry } from '../api/client';
import { filterAndSortEntries, listingSelectionState } from './workspaceListing';

const entries: FileEntry[] = [
  { name: 'Zulu.txt', path: 'Zulu.txt', kind: 'file', sizeBytes: 12, modifiedAt: '2026-01-02T00:00:00Z', mode: '', extension: 'txt', isArchive: false, previewable: true, editable: true, version: '1' },
  { name: 'alpha', path: 'alpha', kind: 'directory', sizeBytes: 0, modifiedAt: '2026-01-03T00:00:00Z', mode: '', extension: '', isArchive: false, previewable: false, editable: false, version: '2' },
  { name: 'notes 10.txt', path: 'notes 10.txt', kind: 'file', sizeBytes: 30, modifiedAt: '2026-01-01T00:00:00Z', mode: '', extension: 'txt', isArchive: false, previewable: true, editable: true, version: '3' },
  { name: 'archive.zip', path: 'archive.zip', kind: 'file', sizeBytes: 50, modifiedAt: '2026-01-04T00:00:00Z', mode: '', extension: 'zip', isArchive: true, previewable: false, editable: false, version: '5' },
  { name: 'duplicate.txt', path: 'duplicate-b.txt', kind: 'file', sizeBytes: 1, modifiedAt: '2026-01-05T00:00:00Z', mode: '', extension: 'txt', isArchive: false, previewable: true, editable: true, version: '6' },
  { name: 'duplicate.txt', path: 'duplicate-a.txt', kind: 'file', sizeBytes: 1, modifiedAt: '2026-01-05T00:00:00Z', mode: '', extension: 'txt', isArchive: false, previewable: true, editable: true, version: '7' }
];

describe('filterAndSortEntries', () => {
  it('filters names case-insensitively without mutating the loaded listing', () => {
    const result = filterAndSortEntries(entries, { query: 'NOTES', kind: 'all', sort: 'name-asc' });

    expect(result.map(({ name }) => name)).toEqual(['notes 10.txt']);
    expect(entries.map(({ name }) => name)).toEqual(['Zulu.txt', 'alpha', 'notes 10.txt', 'archive.zip', 'duplicate.txt', 'duplicate.txt']);
  });

  it('filters by entry kind before sorting the display list', () => {
    expect(filterAndSortEntries(entries, { query: '', kind: 'directory', sort: 'name-desc' }).map(({ name }) => name)).toEqual(['alpha']);
  });

  it('sorts deterministically by modified time and then path', () => {
    expect(filterAndSortEntries(entries, { query: '', kind: 'all', sort: 'modified-asc' }).map(({ name }) => name)).toEqual(['notes 10.txt', 'Zulu.txt', 'alpha', 'archive.zip', 'duplicate.txt', 'duplicate.txt']);
    expect(filterAndSortEntries(entries, { query: '', kind: 'all', sort: 'modified-desc' }).filter(({ name }) => name === 'duplicate.txt').map(({ path }) => path)).toEqual(['duplicate-a.txt', 'duplicate-b.txt']);
  });

  it('filters archive entries independently of their file kind', () => {
    expect(filterAndSortEntries(entries, { query: '', kind: 'archive', sort: 'name-asc' }).map(({ name }) => name)).toEqual(['archive.zip']);
  });

  it('uses paths as deterministic final tie-breakers', () => {
    expect(filterAndSortEntries(entries, { query: 'duplicate', kind: 'file', sort: 'name-asc' }).map(({ path }) => path)).toEqual(['duplicate-a.txt', 'duplicate-b.txt']);
  });

  it('limits batch candidates and select-all state to displayed selected items', () => {
    const displayed = filterAndSortEntries(entries, { query: '', kind: 'archive', sort: 'name-asc' });
    const selection = listingSelectionState(displayed, new Set(['archive.zip', 'Zulu.txt']));

    expect(selection.selectedEntries.map(({ path }) => path)).toEqual(['archive.zip']);
    expect(selection.allSelected).toBe(true);
    expect(selection.partiallySelected).toBe(false);
  });

  it('sorts files by size in descending order', () => {
    expect(filterAndSortEntries(entries, { query: '', kind: 'file', sort: 'size-desc' }).map(({ name }) => name)).toEqual(['archive.zip', 'notes 10.txt', 'Zulu.txt', 'duplicate.txt', 'duplicate.txt']);
  });
});
