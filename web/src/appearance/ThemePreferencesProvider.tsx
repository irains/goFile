import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { createAppTheme } from '../tokens';
import { type AccentId, isAccentId, persistAccentId, readStoredAccentId, themeAccentAttribute, themeAccentStorageKey, readStoredThemeMode, themeModeStorageKey, themeSchemeStorageKey } from '../theme';

type ThemePreferencesContextValue = {
  accentId: AccentId;
  setAccentId: (accentId: AccentId) => void;
};

const ThemePreferencesContext = createContext<ThemePreferencesContextValue | null>(null);

export function ThemePreferencesProvider({ children }: { children: ReactNode }) {
  const [accentId, setAccentIdState] = useState<AccentId>(readStoredAccentId);
  const theme = useMemo(() => createAppTheme(accentId), [accentId]);
  const initialMode = useMemo(readStoredThemeMode, []);

  useEffect(() => {
    document.documentElement.setAttribute(themeAccentAttribute, accentId);
  }, [accentId]);

  useEffect(() => {
    const synchronize = (event: StorageEvent) => {
      if (event.key === themeAccentStorageKey) setAccentIdState(readStoredAccentId());
    };
    window.addEventListener('storage', synchronize);
    return () => window.removeEventListener('storage', synchronize);
  }, []);

  const value = useMemo<ThemePreferencesContextValue>(() => ({
    accentId,
    setAccentId(nextAccentId) {
      if (!isAccentId(nextAccentId)) return;
      setAccentIdState(nextAccentId);
      persistAccentId(nextAccentId);
    }
  }), [accentId]);

  return (
    <ThemePreferencesContext.Provider value={value}>
      <ThemeProvider theme={theme} defaultMode={initialMode} modeStorageKey={themeModeStorageKey} colorSchemeStorageKey={themeSchemeStorageKey} noSsr>
        <CssBaseline enableColorScheme />
        {children}
      </ThemeProvider>
    </ThemePreferencesContext.Provider>
  );
}

export function useThemePreferences() {
  const value = useContext(ThemePreferencesContext);
  if (!value) throw new Error('useThemePreferences must be used within ThemePreferencesProvider');
  return value;
}
