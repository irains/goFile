import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import createCache from '@emotion/cache';
import { CacheProvider } from '@emotion/react';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { getRuntime } from './runtime';
import { readStoredThemeMode, themeModeStorageKey, themeSchemeStorageKey } from './theme';
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

document.documentElement.lang = runtime.language;
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <CacheProvider value={emotionCache}>
      <BrowserRouter basename={runtime.basePath || undefined}>
        <ThemeProvider theme={theme} defaultMode={initialMode} modeStorageKey={themeModeStorageKey} colorSchemeStorageKey={themeSchemeStorageKey} noSsr>
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