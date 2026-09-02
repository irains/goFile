import { beforeEach, describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { I18nProvider } from '../i18n';
import type { FileEntry } from '../api/client';
import { EntryMenu, editorPathFromLocation } from './Workspace';
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
  version: 'v1'
};

beforeEach(() => {
  document.head.innerHTML = '<meta name="fileharbor-base" content="/fileharbor">';
});

describe('editor route parsing', () => {
  it('opens a root-relative editor path below the configured base path', () => {
    expect(editorPathFromLocation({ pathname: '/fileharbor/edit/docs/June%20report.txt' })).toBe('docs/June report.txt');
  });

  it('does not treat a directory route or malformed escape as an editor path', () => {
    expect(editorPathFromLocation({ pathname: '/fileharbor/d/docs/report.txt' })).toBeNull();
    expect(editorPathFromLocation({ pathname: '/fileharbor/edit/%E0%A4%A' })).toBeNull();
  });
});

describe('entry menu actions', () => {
  it('retains only safe actions in restricted workspaces', () => {
    expect(entryMenuActions(file, false, false).map(({ name }) => name)).toEqual(['download', 'preview', 'properties']);
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