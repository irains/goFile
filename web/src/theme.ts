export const themeModeStorageKey = 'fileharbor-mode';
export const themeSchemeStorageKey = 'fileharbor-color-scheme';
export const themeSchemeAttribute = 'data-mui-color-scheme';
export const themeAccentStorageKey = 'fileharbor-accent';
export const themeAccentAttribute = 'data-fileharbor-accent';

export type ThemeMode = 'light' | 'dark' | 'system';
export type AccentId = 'forest' | 'harbor' | 'slate' | 'orchid' | 'cedar';
type ColorScheme = 'light' | 'dark';

type AccentScheme = {
  main: string;
  light: string;
  dark: string;
  contrastText: string;
  prepaintDot: string;
  prepaintWash: string;
};

export type AccentPalette = Record<ColorScheme, AccentScheme>;

export const accentIds: readonly AccentId[] = ['forest', 'harbor', 'slate', 'orchid', 'cedar'];
export const defaultAccentId: AccentId = 'forest';

export const accentPalettes: Record<AccentId, AccentPalette> = {
  forest: {
    light: { main: '#3C6A4D', light: '#70967A', dark: '#2D533B', contrastText: '#FBF9F2', prepaintDot: '67 84 54', prepaintWash: '89 111 74' },
    dark: { main: '#9EC6A0', light: '#C4DEC4', dark: '#729D77', contrastText: '#171D18', prepaintDot: '219 231 207', prepaintWash: '128 159 113' }
  },
  harbor: {
    light: { main: '#236676', light: '#5A92A0', dark: '#174D5A', contrastText: '#FBF9F2', prepaintDot: '45 90 101', prepaintWash: '55 116 132' },
    dark: { main: '#8EC8D3', light: '#B8E1E7', dark: '#5A9EAA', contrastText: '#171D18', prepaintDot: '212 235 238', prepaintWash: '105 168 181' }
  },
  slate: {
    light: { main: '#4C617C', light: '#7B91AD', dark: '#36485F', contrastText: '#FBF9F2', prepaintDot: '70 84 106', prepaintWash: '83 107 139' },
    dark: { main: '#B5C6E1', light: '#D7E1F1', dark: '#8398B7', contrastText: '#171D18', prepaintDot: '224 231 244', prepaintWash: '141 164 200' }
  },
  orchid: {
    light: { main: '#74557F', light: '#A17DAA', dark: '#553B60', contrastText: '#FBF9F2', prepaintDot: '104 78 111', prepaintWash: '127 91 137' },
    dark: { main: '#D2B7DE', light: '#E7D5ED', dark: '#A48AB2', contrastText: '#171D18', prepaintDot: '239 226 243', prepaintWash: '178 139 190' }
  },
  cedar: {
    light: { main: '#8A572E', light: '#B78359', dark: '#69401E', contrastText: '#FBF9F2', prepaintDot: '118 77 44', prepaintWash: '151 96 55' },
    dark: { main: '#E7BF84', light: '#F3DBAF', dark: '#BB8D52', contrastText: '#171D18', prepaintDot: '244 226 194', prepaintWash: '200 151 85' }
  }
};

const MEDIA_DARK = '(prefers-color-scheme: dark)';
const SCHEME_LIGHT: ColorScheme = 'light';
const SCHEME_DARK: ColorScheme = 'dark';

export function isThemeMode(value: unknown): value is ThemeMode {
  return value === 'light' || value === 'dark' || value === 'system';
}

export function isAccentId(value: unknown): value is AccentId {
  return typeof value === 'string' && accentIds.includes(value as AccentId);
}

export function readStoredThemeMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(themeModeStorageKey);
    return isThemeMode(raw) ? raw : 'system';
  } catch {
    return 'system';
  }
}

export function readStoredAccentId(): AccentId {
  try {
    const raw = localStorage.getItem(themeAccentStorageKey);
    return isAccentId(raw) ? raw : defaultAccentId;
  } catch {
    return defaultAccentId;
  }
}

export function persistAccentId(accentId: AccentId) {
  try {
    localStorage.setItem(themeAccentStorageKey, accentId);
  } catch {
    // Browser privacy settings may block storage. The current-tab state remains usable.
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

// Ace editor theme depends on the *resolved* color scheme (not the stored intent).
export function aceThemeForMode(mode: ColorScheme | undefined) {
  return mode === 'light' ? 'github_light_default' : 'github_dark';
}
