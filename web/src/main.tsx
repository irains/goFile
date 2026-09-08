import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import createCache from '@emotion/cache';
import { CacheProvider } from '@emotion/react';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { getRuntime } from './runtime';
import { readStoredThemeMode, themeModeStorageKey, themeSchemeStorageKey } from './theme';
import { createAppTheme } from './tokens';
import { I18nProvider } from './i18n';
import { createAppRouter } from './App';
import './styles.css';

const runtime = getRuntime();
const router = createAppRouter();
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
      <ThemeProvider theme={theme} defaultMode={initialMode} modeStorageKey={themeModeStorageKey} colorSchemeStorageKey={themeSchemeStorageKey} noSsr>
        <CssBaseline enableColorScheme />
        <QueryClientProvider client={queryClient}>
          <I18nProvider>
            <RouterProvider router={router} />
          </I18nProvider>
        </QueryClientProvider>
      </ThemeProvider>
    </CacheProvider>
  </StrictMode>
);