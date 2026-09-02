import { describe, expect, it, vi } from 'vitest';
import { aceThemeForMode, isThemeMode, nextThemeMode, readStoredThemeMode, resolveColorScheme, systemPrefersDark } from './theme';

describe('appearance helpers', () => {
  it('uses the light Ace theme only for the light scheme', () => {
    expect(aceThemeForMode('light')).toBe('github_light_default');
    expect(aceThemeForMode('dark')).toBe('github_dark');
    expect(aceThemeForMode(undefined)).toBe('github_dark');
  });

  it('defaults the first interaction to light and reverses explicit modes', () => {
    expect(nextThemeMode(undefined)).toBe('light');
    expect(nextThemeMode('light')).toBe('dark');
    expect(nextThemeMode('dark')).toBe('system');
    expect(nextThemeMode('system')).toBe('light');
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

  it('recognises valid stored values', () => {
    expect(isThemeMode('light')).toBe(true);
    expect(isThemeMode('dark')).toBe(true);
    expect(isThemeMode('system')).toBe(true);
    expect(isThemeMode('')).toBe(false);
    expect(isThemeMode(null)).toBe(false);
    expect(isThemeMode('auto')).toBe(false);
  });
});

describe('system preference detection', () => {
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
});