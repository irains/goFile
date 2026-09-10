import { beforeEach, describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import type { FileEntry } from '../api/client';
import { EntryMenu, decodeRouteSplat, desktopTableColumnSx, directoryPathForEditor, entryKindLabel, fileNameButtonSx } from './Workspace';
import { listingSelectionState } from './workspaceListing';
import { entryMenuActions } from './entryActions';

const file: FileEntry = {
  name: 'notes.txt',
  path: 'notes.txt',
  kind: 'file',
  sizeBytes: 12,
  modifiedAt: '',
  mode: '',
  extension: 'txt',
  isArchive: false,
  previewable: true,
  editable: true,
  version: 'v1'
};

beforeEach(() => {
  document.head.innerHTML = '<meta name="fileharbor-base" content="/fileharbor">';
});

describe('editor route paths', () => {
  it('uses the already-decoded data-router splat without double decoding', () => {
    expect(decodeRouteSplat('docs/June report.txt')).toBe('docs/June report.txt');
    expect(decodeRouteSplat('literal%2Fname/report.txt')).toBe('literal%2Fname/report.txt');
  });

  it('returns the edited file directory when closing a deep link', () => {
    expect(directoryPathForEditor('docs/June report.txt')).toBe('docs');
    expect(directoryPathForEditor('notes.txt')).toBe('');
  });
});

describe('entry kind labels', () => {
  it('uses localised visible labels rather than raw API kinds', () => {
    const labels: Record<string, string> = { 'workspace.file': 'File', 'workspace.folder': 'Folder' };
    const t = (key: string) => labels[key];
    expect(entryKindLabel('file', t)).toBe('File');
    expect(entryKindLabel('directory', t)).toBe('Folder');
  });
});

describe('workspace table layout', () => {
  it('limits select-all and batch candidates to the displayed entries', () => {
    const archive = { ...file, name: 'archive.zip', path: 'archive.zip', isArchive: true };
    const state = listingSelectionState([archive], new Set([archive.path, file.path]));

    expect(state.selectedEntries).toEqual([archive]);
    expect(state.allSelected).toBe(true);
    expect(state.partiallySelected).toBe(false);
  });

  it('preserves wrapping names while keeping size and modified values on one line', () => {
    expect(desktopTableColumnSx.name).toEqual({ width: '100%' });
    expect(desktopTableColumnSx.size).toEqual({ width: '1%', whiteSpace: 'nowrap' });
    expect(desktopTableColumnSx.modified).toEqual({ width: '1%', whiteSpace: 'nowrap' });
    expect(fileNameButtonSx.overflowWrap).toBe('anywhere');
    expect(fileNameButtonSx).not.toHaveProperty('textOverflow');
    expect(fileNameButtonSx).not.toHaveProperty('whiteSpace');
  });
});

describe('entry menu actions', () => {
  it('retains only safe actions in restricted workspaces', () => {
    expect(entryMenuActions(file, false, false).map(({ name }) => name)).toEqual(['download', 'preview', 'properties']);
  });

  it('hides Edit for files that are not textual', () => {
    expect(entryMenuActions({ ...file, editable: false }, true, true).map(({ name }) => name)).not.toContain('edit');
  });

  it('marks Delete as the only destructive action', () => {
    expect(entryMenuActions(file, true, true).find(({ name }) => name === 'delete')?.destructive).toBe(true);
  });
});

describe('entry menu rendering', () => {
  it('renders direct icon-bearing action rows for a mutable file', () => {
    const anchor = document.createElement('button');
    document.body.append(anchor);
    render(<I18nProvider><EntryMenu entry={file} anchor={anchor} onClose={() => {}} onAction={() => {}} mutable editorAvailable /></I18nProvider>);
    const items = screen.getAllByRole('menuitem');
    expect(items).toHaveLength(9);
    expect(items.every((item) => item.querySelector('svg'))).toBe(true);
    expect(screen.getByRole('menuitem', { name: 'Delete' })).toHaveClass('MuiMenuItem-root');
  });
});
