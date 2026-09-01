import { describe, expect, it } from 'vitest';
import { aceThemeForMode, nextThemeMode } from './theme';

describe('appearance helpers', () => {
  it('uses the light Ace theme only for the light scheme', () => {
    expect(aceThemeForMode('light')).toBe('github_light_default');
    expect(aceThemeForMode('dark')).toBe('github_dark');
    expect(aceThemeForMode(undefined)).toBe('github_dark');
  });

  it('defaults the first interaction to light and reverses explicit modes', () => {
    expect(nextThemeMode(undefined)).toBe('light');
    expect(nextThemeMode('light')).toBe('dark');
    expect(nextThemeMode('dark')).toBe('light');
  });
});
