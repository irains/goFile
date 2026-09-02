import { type SxProps, type Theme, createTheme } from '@mui/material/styles';
import type { CSSProperties } from 'react';

// ──────────────────────────────────────────────────────────────────────────────
// Module augmentation: extend MUI's Typography variants to include the design
// system roles (`display`, `title`, `body`, `bodyStrong`, `caption`, `overline`).
// ──────────────────────────────────────────────────────────────────────────────
declare module '@mui/material/Typography' {
  interface TypographyPropsVariantOverrides {
    display: true;
    title: true;
    body: true;
    bodyStrong: true;
    caption: true;
    overline: true;
  }
}

declare module '@mui/material/styles' {
  interface Theme {
    custom: {
      radii: typeof radii;
      spacing: typeof spacing;
      motion: typeof motion;
      semantic: typeof semantic;
    };
  }
  interface ThemeOptions {
    custom?: Theme['custom'];
  }
  interface TypographyVariants {
    display?: CSSProperties;
    title?: CSSProperties;
    body?: CSSProperties;
    bodyStrong?: CSSProperties;
    caption?: CSSProperties;
    overline?: CSSProperties;
  }
  interface TypographyVariantsOptions {
    display?: CSSProperties;
    title?: CSSProperties;
    body?: CSSProperties;
    bodyStrong?: CSSProperties;
    caption?: CSSProperties;
    overline?: CSSProperties;
  }
}

// ──────────────────────────────────────────────────────────────────────────────
// Spacing — 4 px grid + semantic aliases.
// ──────────────────────────────────────────────────────────────────────────────
export const spacing = {
  xxs: 4,
  xs: 8,
  sm: 12,
  md: 16,
  lg: 20,
  xl: 24,
  xxl: 32,
  gutter: 16,
  page: 24,
  section: 32
} as const;

// ──────────────────────────────────────────────────────────────────────────────
// Radii — drive shape.borderRadius + per-component overrides.
// ──────────────────────────────────────────────────────────────────────────────
export const radii = {
  none: 0,
  sm: 6,
  md: 10,
  lg: 14,
  pill: 999
} as const;

// ──────────────────────────────────────────────────────────────────────────────
// Motion — wire into theme.transitions.duration.* for layout transitions, plus
// raw milliseconds for component-level animation.
// ──────────────────────────────────────────────────────────────────────────────
export const motion = {
  instant: 80,
  fast: 120,
  base: 180,
  slow: 240
} as const;

// ──────────────────────────────────────────────────────────────────────────────
// Surface — the canonical "card / panel" look. elevation=0 + 1 px border.
// ──────────────────────────────────────────────────────────────────────────────
export const surface: SxProps<Theme> = {
  bgcolor: 'background.paper',
  backgroundImage: 'none',
  border: '1px solid',
  borderColor: 'divider',
  borderRadius: radii.md,
  boxShadow: 'none'
};

// ──────────────────────────────────────────────────────────────────────────────
// Semantic colors — entry-kind / status / destructive affordances.
// ──────────────────────────────────────────────────────────────────────────────
export const semantic = {
  folder: 'primary.main',
  folderMuted: 'primary.light',
  file: 'text.secondary',
  archive: 'warning.main',
  readonly: 'text.disabled',
  unsaved: 'warning.main',
  destructive: 'error.main'
} as const;

// ──────────────────────────────────────────────────────────────────────────────
// Typography — system font stack (no webfonts due to strict CSP).
// ──────────────────────────────────────────────────────────────────────────────
export const fontFamily = 'system-ui, -apple-system, "Segoe UI Variable", "Segoe UI", sans-serif';
export const fontFamilyMono = 'ui-monospace, "JetBrains Mono", "Cascadia Mono", Menlo, Consolas, monospace';

// ──────────────────────────────────────────────────────────────────────────────
// App theme factory.
// ──────────────────────────────────────────────────────────────────────────────
export function createAppTheme() {
  return createTheme({
    cssVariables: { colorSchemeSelector: 'data-mui-color-scheme' },
    defaultColorScheme: 'dark',
    colorSchemes: {
      light: {
        palette: {
          primary: { main: '#6252ba', light: '#8272da', dark: '#44388b', contrastText: '#fbfaff' },
          background: { default: '#f8f7fb', paper: '#ffffff' },
          text: { primary: '#211f2a', secondary: '#45415a' },
          divider: '#e6e3ee',
          AppBar: { defaultBg: '#f8f7fb', darkBg: '#f8f7fb' },
          success: { main: '#2f7a56' },
          warning: { main: '#a46112' },
          error: { main: '#b13e4a' },
          info: { main: '#3c62a8' }
        }
      },
      dark: {
        palette: {
          primary: { main: '#9b8cff', light: '#c9c1ff', dark: '#7062cf', contrastText: '#17151f' },
          background: { default: '#17171c', paper: '#23232b' },
          text: { primary: '#eeeef3', secondary: '#aaa9b5' },
          divider: '#3a3a45',
          AppBar: { defaultBg: '#17171c', darkBg: '#17171c' },
          success: { main: '#66b78d' },
          warning: { main: '#d9a450' },
          error: { main: '#e87d86' },
          info: { main: '#7c9eea' }
        }
      }
    },
    shape: { borderRadius: radii.md },
    spacing: [4, 8, 12, 16, 20, 24, 32],
    typography: {
      fontFamily,
      display: { fontSize: 28, lineHeight: '36px', fontWeight: 800, letterSpacing: '-0.01em' },
      title: { fontSize: 20, lineHeight: '28px', fontWeight: 700 },
      body: { fontSize: 14, lineHeight: '22px', fontWeight: 400 },
      bodyStrong: { fontSize: 14, lineHeight: '22px', fontWeight: 600 },
      caption: { fontSize: 12, lineHeight: '18px', fontWeight: 500 },
      overline: { fontSize: 11, lineHeight: '16px', fontWeight: 800, letterSpacing: '0.13em', textTransform: 'uppercase' },
      button: { textTransform: 'none', fontWeight: 600 }
    },
    transitions: {
      duration: {
        shortest: motion.instant,
        shorter: motion.fast,
        short: motion.base,
        standard: motion.base,
        complex: motion.slow,
        enteringScreen: motion.fast,
        leavingScreen: motion.instant
      }
    },
    custom: { radii, spacing, motion, semantic },
    components: {
      MuiButton: {
        styleOverrides: {
          root: { minHeight: 48, borderRadius: radii.md },
          outlined: ({ theme }) => ({
            borderColor: theme.palette.mode === 'light' ? theme.palette.primary.dark : undefined
          })
        }
      },
      MuiIconButton: {
        styleOverrides: { root: { minWidth: 48, minHeight: 48, borderRadius: radii.md } }
      },
      MuiChip: {
        styleOverrides: {
          root: { borderRadius: radii.pill, fontWeight: 600 }
        }
      },
      MuiCssBaseline: {
        styleOverrides: {
          'html, body, #root': { minHeight: '100%' },
          a: { color: 'inherit' },
          ':focus-visible': { outline: '2px solid', outlineColor: 'primary.main', outlineOffset: '2px' },
          '*::-webkit-scrollbar': { width: '10px', height: '10px' },
          '*::-webkit-scrollbar-thumb': { background: 'rgba(127,127,127,.35)', borderRadius: radii.pill }
        }
      }
    }
  });
}