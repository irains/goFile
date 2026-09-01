import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import createCache from '@emotion/cache';
import { CacheProvider } from '@emotion/react';
import { CssBaseline, ThemeProvider, createTheme } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { getRuntime } from './runtime';
import { themeModeStorageKey, themeSchemeAttribute, themeSchemeStorageKey } from './theme';
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
const theme = createTheme({
  cssVariables: { colorSchemeSelector: themeSchemeAttribute },
  defaultColorScheme: 'dark',
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#6252ba', light: '#8272da', dark: '#44388b', contrastText: '#fbfaff' },
        background: { default: '#f8f7fb', paper: '#ffffff' },
        text: { primary: '#211f2a', secondary: '#5f5a6c' },
        divider: '#dfdce8',
        success: { main: '#2f7a56' },
        warning: { main: '#a46112' },
        error: { main: '#b13e4a' },
        info: { main: '#3c62a8' }
      }
    },
    dark: {
      palette: {
        primary: { main: '#9b8cff', light: '#c9c1ff', dark: '#7062cf', contrastText: '#17151f' },
        background: { default: '#17171c', paper: '#202027' },
        text: { primary: '#eeeef3', secondary: '#aaa9b5' },
        divider: '#34333d',
        success: { main: '#66b78d' },
        warning: { main: '#d9a450' },
        error: { main: '#e87d86' },
        info: { main: '#7c9eea' }
      }
    }
  },
  shape: { borderRadius: 10 },
  typography: {
    fontFamily: 'ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    button: { textTransform: 'none', fontWeight: 700 }
  },
  components: {
    MuiButton: { styleOverrides: { root: { minHeight: 42 } } },
    MuiIconButton: { styleOverrides: { root: { minWidth: 44, minHeight: 44 } } },
    MuiCssBaseline: { styleOverrides: { 'html, body, #root': { minHeight: '100%' }, a: { color: 'inherit' } } }
  }
});

document.documentElement.lang = runtime.language;
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <CacheProvider value={emotionCache}>
      <BrowserRouter basename={runtime.basePath || undefined}>
        <ThemeProvider theme={theme} defaultMode="dark" modeStorageKey={themeModeStorageKey} colorSchemeStorageKey={themeSchemeStorageKey} noSsr>
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
