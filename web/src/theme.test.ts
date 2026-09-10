import { describe, expect, it, vi } from 'vitest';
import { accentIds, aceThemeForMode, defaultAccentId, isAccentId, isThemeMode, persistAccentId, readStoredAccentId, readStoredThemeMode, resolveColorScheme, systemPrefersDark, themeAccentStorageKey } from './theme';

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

  it('allows only curated accent IDs', () => {
    expect(accentIds).toEqual(['forest', 'harbor', 'slate', 'orchid', 'cedar']);
    for (const accentId of accentIds) expect(isAccentId(accentId)).toBe(true);
    expect(isAccentId('#236676')).toBe(false);
    expect(isAccentId('red')).toBe(false);
    expect(isAccentId('')).toBe(false);
    expect(isAccentId(null)).toBe(false);
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

  it('restores only a valid accent and defaults otherwise', () => {
    const storage = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => { storage.set(key, value); }
    });
    expect(readStoredAccentId()).toBe(defaultAccentId);
    storage.set(themeAccentStorageKey, 'orchid');
    expect(readStoredAccentId()).toBe('orchid');
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
