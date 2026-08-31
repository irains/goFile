import { beforeEach, describe, expect, it } from 'vitest';
import { editorPathFromLocation } from './Workspace';

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
