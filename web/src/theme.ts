export const themeModeStorageKey = 'fileharbor-mode';
export const themeSchemeStorageKey = 'fileharbor-color-scheme';
export const themeSchemeAttribute = 'data-mui-color-scheme';

export function aceThemeForMode(mode: string | undefined) {
  return mode === 'light' ? 'github_light_default' : 'github_dark';
}

export function nextThemeMode(mode: string | undefined): 'light' | 'dark' {
  return mode === 'light' ? 'dark' : 'light';
}
