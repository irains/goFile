import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import createCache from '@emotion/cache';
import { CacheProvider } from '@emotion/react';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useColorScheme } from '@mui/material/styles';
import { getRuntime } from './runtime';
import { readStoredThemeMode, resolveColorScheme, systemPrefersDark, themeModeStorageKey, themeSchemeAttribute, themeSchemeStorageKey } from './theme';
import { createAppTheme } from './tokens';
import { I18nProvider } from './i18n';
import { LoginPage, isLoginRoute } from './components/LoginPage';
import { Workspace } from './components/Workspace';
import { SessionProvider } from './session/SessionProvider';
import './styles.css';

const runtime = getRuntime();
const loginPage = isLoginRoute();
const emotionCache = createCache({ key: 'fileharbor', nonce: runtime.csrfNonce || undefined });
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false, refetchOnWindowFocus: false },
    mutations: { retry: false }
  }
});
const theme = createAppTheme();
const initialMode = readStoredThemeMode();

// Resolve color scheme from intent, listening to system changes for the `system` intent.
function useSystemColorSync() {
  const { mode, setMode } = useColorScheme();
  const [systemDark, setSystemDark] = useState<boolean>(() => systemPrefersDark());
  useEffect(() => {
    if (mode !== 'system') return;
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = (event: MediaQueryListEvent) => {
      const next = resolveColorScheme('system');
      setSystemDark(event.matches);
      const root = document.documentElement;
      root.setAttribute(themeSchemeAttribute, next);
      root.style.colorScheme = next;
    };
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, [mode, setMode]);
  // First-render sync so a stored 'system' intent reflects the current media query.
  useEffect(() => {
    if (mode !== 'system') return;
    const resolved = resolveColorScheme('system');
    const root = document.documentElement;
    root.setAttribute(themeSchemeAttribute, resolved);
    root.style.colorScheme = resolved;
    setMode('system');
  }, [mode, systemDark, setMode]);
}

function ThemeSync() {
  useSystemColorSync();
  return null;
}

document.documentElement.lang = runtime.language;
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <CacheProvider value={emotionCache}>
      <BrowserRouter basename={runtime.basePath || undefined}>
        <ThemeProvider theme={theme} defaultMode={initialMode} modeStorageKey={themeModeStorageKey} colorSchemeStorageKey={themeSchemeStorageKey} noSsr>
          <ThemeSync />
          <CssBaseline enableColorScheme />
          <QueryClientProvider client={queryClient}>
            <I18nProvider>
              <SessionProvider loginPage={loginPage}>{loginPage ? <LoginPage /> : <Workspace />}</SessionProvider>
            </I18nProvider>
          </QueryClientProvider>
        </ThemeProvider>
      </BrowserRouter>
    </CacheProvider>
  </StrictMode>
);