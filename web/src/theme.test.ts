import { describe, expect, it, vi } from 'vitest';
import { accentIds, accentPalettes, aceThemeForMode, defaultAccentId, isAccentId, isThemeMode, persistAccentId, readStoredAccentId, readStoredThemeMode, resolveColorScheme, systemPrefersDark, themeAccentStorageKey } from './theme';

describe('appearance helpers', () => {
  it('uses the light Ace theme only for the light scheme', () => {
    expect(aceThemeForMode('light')).toBe('github_light_default');
    expect(aceThemeForMode('dark')).toBe('github_dark');
    expect(aceThemeForMode(undefined)).toBe('github_dark');
  });

  it('resolves explicit intents to themselves', () => {
    expect(resolveColorScheme('light')).toBe('light');
    expect(resolveColorScheme('dark')).toBe('dark');
  });

  it('resolves system intent using prefers-color-scheme', () => {
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: query.includes('dark') }));
    expect(resolveColorScheme('system')).toBe('dark');
    vi.stubGlobal('matchMedia', () => ({ matches: false }));
    expect(resolveColorScheme('system')).toBe('light');
    expect(resolveColorScheme(undefined)).toBe('light');
  });

  it('recognises valid stored mode values', () => {
    expect(isThemeMode('light')).toBe(true);
    expect(isThemeMode('dark')).toBe(true);
    expect(isThemeMode('system')).toBe(true);
    expect(isThemeMode('')).toBe(false);
    expect(isThemeMode(null)).toBe(false);
    expect(isThemeMode('auto')).toBe(false);
  });

  it('allows only the twelve curated page palette IDs', () => {
    expect(accentIds).toEqual(['forest', 'harbor', 'slate', 'orchid', 'cedar', 'fjord', 'juniper', 'ember', 'dune', 'saffron', 'mulberry', 'graphite']);
    for (const accentId of accentIds) expect(isAccentId(accentId)).toBe(true);
    expect(isAccentId('#236676')).toBe(false);
    expect(isAccentId('red')).toBe(false);
    expect(isAccentId('')).toBe(false);
    expect(isAccentId(null)).toBe(false);
  });

  it('gives every palette complete light and dark surface roles', () => {
    for (const accentId of accentIds) {
      for (const scheme of ['light', 'dark'] as const) {
        const palette = accentPalettes[accentId][scheme];
        expect(palette.canvas).toMatch(/^#/);
        expect(palette.paper).toMatch(/^#/);
        expect(palette.appBar).toMatch(/^#/);
        expect(palette.overlay).toMatch(/^#/);
        expect(palette.tableHeader).toMatch(/^#/);
        expect(palette.text.primary).toMatch(/^#/);
        expect(palette.divider).toMatch(/^#/);
        expect(palette.focus).toMatch(/^#/);
        expect(palette.texture.dot).toMatch(/^\d+ \d+ \d+$/);
      }
    }
  });
});

describe('stored preferences', () => {
  it('falls back to light when matchMedia is unavailable', () => {
    vi.stubGlobal('matchMedia', undefined);
    expect(systemPrefersDark()).toBe(false);
  });

  it('reads the stored mode from localStorage when available', () => {
    const storage = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => { storage.set(key, value); }
    });
    storage.set('fileharbor-mode', 'dark');
    expect(readStoredThemeMode()).toBe('dark');
    storage.set('fileharbor-mode', 'system');
    expect(readStoredThemeMode()).toBe('system');
    storage.set('fileharbor-mode', 'invalid');
    expect(readStoredThemeMode()).toBe('system');
  });

  it('restores legacy and new palette IDs, but defaults invalid values', () => {
    const storage = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => { storage.set(key, value); }
    });
    expect(readStoredAccentId()).toBe(defaultAccentId);
    storage.set(themeAccentStorageKey, 'orchid');
    expect(readStoredAccentId()).toBe('orchid');
    storage.set(themeAccentStorageKey, 'graphite');
    expect(readStoredAccentId()).toBe('graphite');
    storage.set(themeAccentStorageKey, '#abcdef');
    expect(readStoredAccentId()).toBe(defaultAccentId);
    persistAccentId('cedar');
    expect(storage.get(themeAccentStorageKey)).toBe('cedar');
  });

  it('tolerates unavailable browser storage', () => {
    vi.stubGlobal('localStorage', { getItem: () => { throw new Error('blocked'); }, setItem: () => { throw new Error('blocked'); } });
    expect(readStoredAccentId()).toBe(defaultAccentId);
    expect(() => persistAccentId('harbor')).not.toThrow();
  });
});
