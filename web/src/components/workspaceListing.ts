import type { FileEntry } from '../api/client';

export type ListingKindFilter = 'all' | FileEntry['kind'] | 'archive';
export type ListingSort = 'name-asc' | 'name-desc' | 'modified-desc' | 'modified-asc' | 'size-desc' | 'size-asc';

export type ListingControls = {
  query: string;
  kind: ListingKindFilter;
  sort: ListingSort;
};

export const EMPTY_ENTRIES: FileEntry[] = [];

const nameCollator = new Intl.Collator('en', { numeric: true, sensitivity: 'base' });

function compareModified(left: FileEntry, right: FileEntry): number {
  return new Date(left.modifiedAt).getTime() - new Date(right.modifiedAt).getTime();
}

function compareEntries(left: FileEntry, right: FileEntry, sort: ListingSort): number {
  switch (sort) {
    case 'name-desc':
      return nameCollator.compare(right.name, left.name);
    case 'modified-desc':
      return compareModified(right, left);
    case 'modified-asc':
      return compareModified(left, right);
    case 'size-desc':
      return right.sizeBytes - left.sizeBytes;
    case 'size-asc':
      return left.sizeBytes - right.sizeBytes;
    default:
      return nameCollator.compare(left.name, right.name);
  }
}

export type ListingSelectionState = {
  selectedEntries: FileEntry[];
  allSelected: boolean;
  partiallySelected: boolean;
};

export function listingSelectionState(entries: FileEntry[], selected: ReadonlySet<string>): ListingSelectionState {
  const selectedEntries = entries.filter((entry) => selected.has(entry.path));
  return {
    selectedEntries,
    allSelected: entries.length > 0 && selectedEntries.length === entries.length,
    partiallySelected: selectedEntries.length > 0 && selectedEntries.length < entries.length
  };
}

export function filterAndSortEntries(entries: FileEntry[], controls: ListingControls): FileEntry[] {
  const query = controls.query.trim().toLocaleLowerCase();

  return entries
    .filter((entry) => {
      if (controls.kind === 'all') return true;
      if (controls.kind === 'archive') return entry.isArchive;
      return entry.kind === controls.kind;
    })
    .filter((entry) => !query || entry.name.toLocaleLowerCase().includes(query))
    .slice()
    .sort((left, right) => compareEntries(left, right, controls.sort) || nameCollator.compare(left.name, right.name) || nameCollator.compare(left.path, right.path));
}
