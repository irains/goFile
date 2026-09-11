import { type SxProps, type Theme, createTheme } from '@mui/material/styles';
import type { AccentId } from './theme';
import { accentPalettes } from './theme';
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
export function createAppTheme(accentId: AccentId = 'forest') {
  const palette = accentPalettes[accentId];
  return createTheme({
    cssVariables: { colorSchemeSelector: 'data-mui-color-scheme' },
    defaultColorScheme: 'dark',
    colorSchemes: {
      light: {
        palette: {
          primary: palette.light.primary,
          secondary: palette.light.secondary,
          background: { default: palette.light.canvas, paper: palette.light.paper },
          text: palette.light.text,
          divider: palette.light.divider,
          action: palette.light.action,
          AppBar: { defaultBg: palette.light.appBar, darkBg: palette.light.appBar },
          success: { main: '#3C6A4D', light: '#70967A', dark: '#2D533B', contrastText: '#FBF9F2' },
          warning: { main: '#9A651D', light: '#C39148', dark: '#754811', contrastText: '#242820' },
          error: { main: '#A2443C', light: '#C97971', dark: '#7C302B', contrastText: '#FBF9F2' },
          info: { main: '#286470', light: '#5C9299', dark: '#1D4C55', contrastText: '#FBF9F2' }
        }
      },
      dark: {
        palette: {
          primary: palette.dark.primary,
          secondary: palette.dark.secondary,
          background: { default: palette.dark.canvas, paper: palette.dark.paper },
          text: palette.dark.text,
          divider: palette.dark.divider,
          action: palette.dark.action,
          AppBar: { defaultBg: palette.dark.appBar, darkBg: palette.dark.appBar },
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
          root: () => ({
            minHeight: 40,
            borderRadius: radii.sm,
            transition: controlTransition,
            '&:hover': { boxShadow: '0 3px 10px color-mix(in srgb, var(--fileharbor-shadow) 22%, transparent)' },
            '&:active': { transform: 'translateY(1px)', boxShadow: 'none' }
          }),
          sizeSmall: { minHeight: 32 },
          sizeLarge: { minHeight: 44 },
          outlined: { borderColor: 'var(--mui-palette-divider)', '&:hover': { borderColor: 'var(--mui-palette-primary-main)' } }
        }
      },
      MuiIconButton: {
        styleOverrides: {
          root: () => ({
            minWidth: 40,
            minHeight: 40,
            borderRadius: radii.sm,
            transition: controlTransition,
            '&:hover': { backgroundColor: 'var(--mui-palette-action-hover)', boxShadow: '0 2px 8px color-mix(in srgb, var(--fileharbor-shadow) 18%, transparent)' },
            '&:active': { transform: 'translateY(1px)', boxShadow: 'none' }
          }),
          sizeSmall: { minWidth: 32, minHeight: 32, ...compactTouchTarget }
        }
      },
      MuiChip: { styleOverrides: { root: { borderRadius: radii.sm, fontWeight: 600 } } },
      MuiOutlinedInput: {
        styleOverrides: {
          // Component overrides are evaluated against the default scheme. Use live
          // CSS variables so every rendered scheme keeps its own foreground and paper.
          root: {
            backgroundColor: 'var(--mui-palette-background-paper)',
            color: 'var(--mui-palette-text-primary)',
            transition: controlTransition
          },
          input: {
            color: 'var(--mui-palette-text-primary)',
            WebkitTextFillColor: 'var(--mui-palette-text-primary)',
            '&:-webkit-autofill': {
              WebkitBoxShadow: '0 0 0 100px var(--mui-palette-background-paper) inset',
              WebkitTextFillColor: 'var(--mui-palette-text-primary)'
            }
          },
          notchedOutline: { borderColor: 'var(--mui-palette-divider)' }
        }
      },
      MuiTableCell: { styleOverrides: { head: { backgroundColor: 'var(--fileharbor-table-header)', color: 'var(--mui-palette-text-secondary)', fontWeight: 700 } } },
      MuiTableRow: {
        styleOverrides: {
          root: ({ theme }) => ({
            transition: theme.transitions.create(['background-color', 'box-shadow'], { duration: theme.transitions.duration.short }),
            '&.Mui-selected': { backgroundColor: 'var(--mui-palette-action-selected)' },
            '&.MuiTableRow-hover:hover': { backgroundColor: 'var(--mui-palette-action-hover)' }
          })
        }
      },
      MuiDialog: { styleOverrides: { paper: { backgroundColor: 'var(--fileharbor-overlay)', backgroundImage: 'none', border: '1px solid var(--mui-palette-divider)', boxShadow: '0 14px 36px color-mix(in srgb, var(--fileharbor-shadow) 30%, transparent)' } } },
      MuiDrawer: { styleOverrides: { paper: { backgroundColor: 'var(--fileharbor-overlay)', backgroundImage: 'none', borderLeft: '1px solid var(--mui-palette-divider)' } } },
      MuiMenu: { styleOverrides: { paper: { backgroundColor: 'var(--fileharbor-overlay)', backgroundImage: 'none', border: '1px solid var(--mui-palette-divider)', boxShadow: '0 8px 20px color-mix(in srgb, var(--fileharbor-shadow) 24%, transparent)' } } },
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
