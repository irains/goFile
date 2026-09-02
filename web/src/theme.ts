export const themeModeStorageKey = 'fileharbor-mode';
export const themeSchemeStorageKey = 'fileharbor-color-scheme';
export const themeSchemeAttribute = 'data-mui-color-scheme';

export type ThemeMode = 'light' | 'dark' | 'system';

const MEDIA_DARK = '(prefers-color-scheme: dark)';
const SCHEME_LIGHT: ColorScheme = 'light';
const SCHEME_DARK: ColorScheme = 'dark';
type ColorScheme = 'light' | 'dark';

export function isThemeMode(value: unknown): value is ThemeMode {
  return value === 'light' || value === 'dark' || value === 'system';
}

export function readStoredThemeMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(themeModeStorageKey);
    return isThemeMode(raw) ? raw : 'system';
  } catch {
    return 'system';
  }
}

export function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;
  return window.matchMedia(MEDIA_DARK).matches;
}

// Resolves the effective `data-mui-color-scheme` value for a stored intent.
// `system` falls through to the OS preference.
export function resolveColorScheme(intent: ThemeMode | undefined): ColorScheme {
  if (intent === 'light') return SCHEME_LIGHT;
  if (intent === 'dark') return SCHEME_DARK;
  return systemPrefersDark() ? SCHEME_DARK : SCHEME_LIGHT;
}

export function nextThemeMode(mode: ThemeMode | undefined): ThemeMode {
  if (mode === 'light') return 'dark';
  if (mode === 'dark') return 'system';
  return 'light';
}

// Ace editor theme depends on the *resolved* color scheme (not the stored intent).
export function aceThemeForMode(mode: ColorScheme | undefined) {
  return mode === 'light' ? 'github_light_default' : 'github_dark';
}
