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
  sm: 4,
  md: 6,
  lg: 8,
  pill: 999
} as const;

// ──────────────────────────────────────────────────────────────────────────────
// Motion — calm, stable state changes. No layout animation.
// ──────────────────────────────────────────────────────────────────────────────
export const motion = {
  instant: 120,
  fast: 180,
  base: 200,
  slow: 240
} as const;

// ──────────────────────────────────────────────────────────────────────────────
// Surface — the canonical opaque paper / panel look.
// ──────────────────────────────────────────────────────────────────────────────
export const surface: SxProps<Theme> = {
  bgcolor: 'background.paper',
  backgroundImage: 'none',
  border: '1px solid',
  borderColor: 'divider',
  borderRadius: `${radii.md}px`,
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

const controlTransition = 'box-shadow 200ms cubic-bezier(0.16, 1, 0.3, 1), background-color 200ms cubic-bezier(0.16, 1, 0.3, 1), border-color 200ms cubic-bezier(0.16, 1, 0.3, 1), transform 180ms cubic-bezier(0.16, 1, 0.3, 1)';
const compactTouchTarget = {
  '@media (pointer: coarse)': { minWidth: 44, minHeight: 44 }
};

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
          primary: { main: '#3C6A4D', light: '#70967A', dark: '#2D533B', contrastText: '#FBF9F2' },
          secondary: { main: '#286470', light: '#5C9299', dark: '#1D4C55', contrastText: '#FBF9F2' },
          background: { default: '#F3F0E7', paper: '#FBF9F2' },
          text: { primary: '#242820', secondary: '#596052', disabled: '#7A8074' },
          divider: '#D9D5C6',
          action: { hover: 'rgba(60, 106, 77, 0.055)', selected: 'rgba(60, 106, 77, 0.11)', disabled: 'rgba(36, 40, 32, 0.12)', disabledBackground: 'rgba(36, 40, 32, 0.08)' },
          AppBar: { defaultBg: '#F3F0E7', darkBg: '#F3F0E7' },
          success: { main: '#3C6A4D', light: '#70967A', dark: '#2D533B', contrastText: '#FBF9F2' },
          warning: { main: '#9A651D', light: '#C39148', dark: '#754811', contrastText: '#242820' },
          error: { main: '#A2443C', light: '#C97971', dark: '#7C302B', contrastText: '#FBF9F2' },
          info: { main: '#286470', light: '#5C9299', dark: '#1D4C55', contrastText: '#FBF9F2' }
        }
      },
      dark: {
        palette: {
          primary: { main: '#9EC6A0', light: '#C4DEC4', dark: '#729D77', contrastText: '#171D18' },
          secondary: { main: '#84C5CC', light: '#B4E0E2', dark: '#55939B', contrastText: '#171D18' },
          background: { default: '#171D18', paper: '#222A22' },
          text: { primary: '#F0F1E7', secondary: '#C1C7B9', disabled: '#899286' },
          divider: '#3B463C',
          action: { hover: 'rgba(196, 222, 196, 0.075)', selected: 'rgba(158, 198, 160, 0.16)', disabled: 'rgba(240, 241, 231, 0.13)', disabledBackground: 'rgba(240, 241, 231, 0.08)' },
          AppBar: { defaultBg: '#171D18', darkBg: '#171D18' },
          success: { main: '#9EC6A0', light: '#C4DEC4', dark: '#729D77', contrastText: '#171D18' },
          warning: { main: '#E6B76E', light: '#F2D39B', dark: '#B98235', contrastText: '#171D18' },
          error: { main: '#F0A19A', light: '#F7C3BD', dark: '#C9746C', contrastText: '#171D18' },
          info: { main: '#84C5CC', light: '#B4E0E2', dark: '#55939B', contrastText: '#171D18' }
        }
      }
    },
    shape: { borderRadius: radii.md },
    spacing: 4,
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
      },
      easing: { easeInOut: 'cubic-bezier(0.16, 1, 0.3, 1)', easeOut: 'cubic-bezier(0.16, 1, 0.3, 1)' }
    },
    custom: { radii, spacing, motion, semantic },
    components: {
      MuiButton: {
        styleOverrides: {
          root: ({ theme }) => ({
            minHeight: 40,
            borderRadius: radii.sm,
            transition: controlTransition,
            '&:hover': { boxShadow: `0 3px 10px ${theme.palette.mode === 'light' ? 'rgba(54, 63, 45, 0.12)' : 'rgba(0, 0, 0, 0.24)'}` },
            '&:active': { transform: 'translateY(1px)', boxShadow: 'none' }
          }),
          sizeSmall: { minHeight: 32 },
          sizeLarge: { minHeight: 44 },
          outlined: ({ theme }) => ({ borderColor: theme.palette.divider, '&:hover': { borderColor: theme.palette.primary.main } })
        }
      },
      MuiIconButton: {
        styleOverrides: {
          root: ({ theme }) => ({
            minWidth: 40,
            minHeight: 40,
            borderRadius: radii.sm,
            transition: controlTransition,
            '&:hover': { backgroundColor: theme.palette.action.hover, boxShadow: `0 2px 8px ${theme.palette.mode === 'light' ? 'rgba(54, 63, 45, 0.1)' : 'rgba(0, 0, 0, 0.22)'}` },
            '&:active': { transform: 'translateY(1px)', boxShadow: 'none' }
          }),
          sizeSmall: { minWidth: 32, minHeight: 32, ...compactTouchTarget }
        }
      },
      MuiChip: { styleOverrides: { root: { borderRadius: radii.sm, fontWeight: 600 } } },
      MuiOutlinedInput: {
        styleOverrides: {
          root: ({ theme }) => ({ backgroundColor: theme.palette.background.paper, transition: controlTransition }),
          notchedOutline: ({ theme }) => ({ borderColor: theme.palette.divider })
        }
      },
      MuiTableCell: { styleOverrides: { head: ({ theme }) => ({ backgroundColor: theme.palette.background.paper, color: theme.palette.text.secondary, fontWeight: 700 }) } },
      MuiTableRow: {
        styleOverrides: {
          root: ({ theme }) => ({
            transition: theme.transitions.create(['background-color', 'box-shadow'], { duration: theme.transitions.duration.short }),
            '&.Mui-selected': { backgroundColor: theme.palette.action.selected },
            '&.MuiTableRow-hover:hover': { backgroundColor: theme.palette.action.hover }
          })
        }
      },
      MuiDialog: { styleOverrides: { paper: ({ theme }) => ({ backgroundImage: 'none', border: `1px solid ${theme.palette.divider}`, boxShadow: theme.palette.mode === 'light' ? '0 14px 36px rgba(54, 63, 45, 0.16)' : '0 18px 42px rgba(0, 0, 0, 0.34)' }) } },
      MuiDrawer: { styleOverrides: { paper: ({ theme }) => ({ backgroundImage: 'none', borderLeft: `1px solid ${theme.palette.divider}` }) } },
      MuiMenu: { styleOverrides: { paper: ({ theme }) => ({ backgroundImage: 'none', border: `1px solid ${theme.palette.divider}`, boxShadow: theme.palette.mode === 'light' ? '0 8px 20px rgba(54, 63, 45, 0.13)' : '0 10px 28px rgba(0, 0, 0, 0.3)' }) } },
      MuiAlert: { styleOverrides: { root: { borderRadius: radii.sm } } },
      MuiLinearProgress: { styleOverrides: { root: { borderRadius: radii.sm, height: 6 } } },
      MuiCssBaseline: {
        styleOverrides: {
          'html, body, #root': { minHeight: '100%' },
          a: { color: 'inherit' },
          ':focus-visible': { outline: '2px solid var(--mui-palette-primary-main)', outlineOffset: '2px' }
        }
      }
    }
  });
}
