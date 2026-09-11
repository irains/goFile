import { describe, expect, it } from 'vitest';
import { createAppTheme, fontFamilyMono, radii, semantic, spacing, surface, motion } from './tokens';
import { accentIds, accentPalettes } from './theme';

const relativeLuminance = (hex: string) => {
  const channels = hex.slice(1).match(/.{2}/g)!.map((value) => Number.parseInt(value, 16) / 255);
  const linear = channels.map((value) => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4);
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
};

const contrast = (foreground: string, background: string) => {
  const [lighter, darker] = [relativeLuminance(foreground), relativeLuminance(background)].sort((a, b) => b - a);
  return (lighter + 0.05) / (darker + 0.05);
};

describe('design tokens', () => {
  it('exposes a 4 px spacing grid', () => {
    expect(spacing.xxs).toBe(4);
    expect(spacing.md).toBe(16);
    expect(spacing.xxl).toBe(32);
  });

  it('keeps task surfaces compact and corners restrained', () => {
    expect(radii.none).toBe(0);
    expect(radii.sm).toBe(4);
    expect(radii.md).toBe(6);
    expect(radii.lg).toBe(8);
    expect(radii.pill).toBeGreaterThan(radii.lg);
  });

  it('uses calm interaction durations in the approved range', () => {
    expect(motion.instant).toBeLessThan(motion.fast);
    expect(motion.fast).toBeGreaterThanOrEqual(180);
    expect(motion.base).toBeGreaterThanOrEqual(180);
    expect(motion.slow).toBeLessThanOrEqual(260);
  });

  it('exposes the semantic palette slots', () => {
    expect(semantic.destructive).toBe('error.main');
    expect(semantic.folder).toBe('primary.main');
    expect(semantic.unsaved).toBe('warning.main');
  });

  it('exposes a mono font stack with a non-empty fallback', () => {
    expect(fontFamilyMono).toContain('monospace');
    expect(fontFamilyMono).toContain('JetBrains Mono');
  });

  it('exposes a surface mixin that uses theme tokens', () => {
    expect(surface).toMatchObject({ borderRadius: `${radii.md}px`, bgcolor: 'background.paper' });
  });

  it('uses explicit warm material roles for the default Forest palette', () => {
    const theme = createAppTheme('forest');
    const colorSchemes = (theme as unknown as {
      colorSchemes: {
        light: { palette: { background: { default: string; paper: string }; primary: { main: string }; info: { main: string } } };
        dark: { palette: { background: { default: string; paper: string }; primary: { main: string }; info: { main: string } } };
      };
    }).colorSchemes;
    const light = colorSchemes.light.palette;
    const dark = colorSchemes.dark.palette;
    expect(light.background?.default).toBe('#F3F0E7');
    expect(light.background?.paper).toBe('#FBF9F2');
    expect(light.primary?.main).toBe('#3C6A4D');
    expect(light.info?.main).toBe('#286470');
    expect(dark.background?.default).toBe('#171D18');
    expect(dark.background?.paper).toBe('#222A22');
    expect(dark.primary?.main).toBe('#9EC6A0');
    expect(dark.info?.main).toBe('#84C5CC');
  });

  it('maps every palette to distinct page surfaces while preserving semantic status colors', () => {
    const lightCanvases = new Set<string>();
    const darkCanvases = new Set<string>();
    for (const accentId of accentIds) {
      const theme = createAppTheme(accentId);
      const colorSchemes = (theme as unknown as {
        colorSchemes: {
          light: { palette: { background: { default: string; paper: string }; primary: { main: string; contrastText: string }; text: { primary: string }; divider: string; AppBar: { defaultBg: string }; success: { main: string }; warning: { main: string }; error: { main: string }; info: { main: string } } };
          dark: { palette: { background: { default: string; paper: string }; primary: { main: string; contrastText: string }; text: { primary: string }; divider: string; AppBar: { defaultBg: string }; success: { main: string }; warning: { main: string }; error: { main: string }; info: { main: string } } };
        };
      }).colorSchemes;
      const light = colorSchemes.light.palette;
      const dark = colorSchemes.dark.palette;
      const lightPalette = accentPalettes[accentId].light;
      const darkPalette = accentPalettes[accentId].dark;
      lightCanvases.add(light.background.default);
      darkCanvases.add(dark.background.default);
      expect(light.background.default).toBe(lightPalette.canvas);
      expect(light.background.paper).toBe(lightPalette.paper);
      expect(light.primary.main).toBe(lightPalette.primary.main);
      expect(light.text.primary).toBe(lightPalette.text.primary);
      expect(light.divider).toBe(lightPalette.divider);
      expect(light.AppBar.defaultBg).toBe(lightPalette.appBar);
      expect(dark.background.default).toBe(darkPalette.canvas);
      expect(dark.background.paper).toBe(darkPalette.paper);
      expect(dark.primary.main).toBe(darkPalette.primary.main);
      expect(dark.text.primary).toBe(darkPalette.text.primary);
      expect(dark.divider).toBe(darkPalette.divider);
      expect(dark.AppBar.defaultBg).toBe(darkPalette.appBar);
      expect(contrast(light.text.primary, light.background.paper)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(light.primary.main, light.background.paper)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(light.primary.contrastText, light.primary.main)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(lightPalette.focus, light.background.paper)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(dark.text.primary, dark.background.paper)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(dark.primary.main, dark.background.default)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(dark.primary.contrastText, dark.primary.main)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(darkPalette.focus, dark.background.paper)).toBeGreaterThanOrEqual(4.5);
      expect(light.success.main).toBe('#3C6A4D');
      expect(light.warning.main).toBe('#9A651D');
      expect(light.error.main).toBe('#A2443C');
      expect(light.info.main).toBe('#286470');
      expect(dark.success.main).toBe('#9EC6A0');
      expect(dark.warning.main).toBe('#E6B76E');
      expect(dark.error.main).toBe('#F0A19A');
      expect(dark.info.main).toBe('#84C5CC');
    }
    expect(lightCanvases).toHaveLength(accentIds.length);
    expect(darkCanvases).toHaveLength(accentIds.length);
  });

  it('keeps primary text and material surfaces contrast-safe', () => {
    expect(contrast('#242820', '#FBF9F2')).toBeGreaterThanOrEqual(4.5);
    expect(contrast('#286470', '#FBF9F2')).toBeGreaterThanOrEqual(4.5);
    expect(contrast('#F0F1E7', '#222A22')).toBeGreaterThanOrEqual(4.5);
  });

  it('derives component shadows from the selected palette surface', () => {
    const harbor = createAppTheme('harbor');
    const button = harbor.components?.MuiButton?.styleOverrides;
    const root = button?.root as ((props: { theme: typeof harbor }) => Record<string, unknown>);
    const hover = root({ theme: harbor })['&:hover'] as { boxShadow: string };
    expect(hover.boxShadow).toContain('var(--fileharbor-shadow)');
  });
  it('supports fractional spacing used by compact stacks and rows', () => {
    const theme = createAppTheme();
    expect(theme.spacing(1.5)).toContain('1.5 * var(--mui-spacing, 4px)');
    expect(theme.spacing(2.5)).toContain('2.5 * var(--mui-spacing, 4px)');
  });

  it('uses CSS variables for the global focus outline', () => {
    const theme = createAppTheme();
    const baseline = theme.components?.MuiCssBaseline?.styleOverrides as Record<string, Record<string, string>>;
    expect(baseline[':focus-visible']).toMatchObject({ outline: '2px solid var(--mui-palette-primary-main)', outlineOffset: '2px' });
  });

  it('keeps outlined inputs bound to the active color-scheme variables', () => {
    const theme = createAppTheme();
    const outlinedInput = theme.components?.MuiOutlinedInput?.styleOverrides;
    expect(outlinedInput?.root).toMatchObject({
      backgroundColor: 'var(--mui-palette-background-paper)',
      color: 'var(--mui-palette-text-primary)'
    });
    expect(outlinedInput?.input).toMatchObject({
      color: 'var(--mui-palette-text-primary)',
      WebkitTextFillColor: 'var(--mui-palette-text-primary)',
      '&:-webkit-autofill': {
        WebkitBoxShadow: '0 0 0 100px var(--mui-palette-background-paper) inset',
        WebkitTextFillColor: 'var(--mui-palette-text-primary)'
      }
    });
    expect(outlinedInput?.notchedOutline).toMatchObject({ borderColor: 'var(--mui-palette-divider)' });
  });

  it('keeps table controls bound to the active color-scheme variables', () => {
    const theme = createAppTheme();
    const tableCell = theme.components?.MuiTableCell?.styleOverrides;
    const tableRow = theme.components?.MuiTableRow?.styleOverrides;
    const iconButton = theme.components?.MuiIconButton?.styleOverrides;
    const tableRowRoot = tableRow?.root as ((props: { theme: typeof theme }) => Record<string, unknown>);
    const iconButtonRoot = iconButton?.root as ((props: { theme: typeof theme }) => Record<string, unknown>);
    expect(tableCell?.head).toMatchObject({
      backgroundColor: 'var(--fileharbor-table-header)',
      color: 'var(--mui-palette-text-secondary)'
    });
    expect(tableRowRoot({ theme })).toMatchObject({
      '&.Mui-selected': { backgroundColor: 'var(--mui-palette-action-selected)' },
      '&.MuiTableRow-hover:hover': { backgroundColor: 'var(--mui-palette-action-hover)' }
    });
    expect(iconButtonRoot({ theme })).toMatchObject({
      '&:hover': { backgroundColor: 'var(--mui-palette-action-hover)' }
    });
  });

  it('keeps normal controls comfortable and table controls dense', () => {
    const theme = createAppTheme();
    const button = theme.components?.MuiButton?.styleOverrides;
    const iconButton = theme.components?.MuiIconButton?.styleOverrides;
    const chip = theme.components?.MuiChip?.styleOverrides;
    const buttonRoot = button?.root as ((props: unknown) => Record<string, unknown>);
    const iconButtonRoot = iconButton?.root as ((props: unknown) => Record<string, unknown>);
    expect(buttonRoot({ theme })).toMatchObject({ minHeight: 40, borderRadius: radii.sm });
    expect(button?.sizeSmall).toMatchObject({ minHeight: 32 });
    expect(button?.sizeLarge).toMatchObject({ minHeight: 44 });
    expect(iconButtonRoot({ theme })).toMatchObject({ minWidth: 40, minHeight: 40, borderRadius: radii.sm });
    expect(iconButton?.sizeSmall).toMatchObject({ minWidth: 32, minHeight: 32, '@media (pointer: coarse)': { minWidth: 44, minHeight: 44 } });
    expect(chip?.root).toMatchObject({ borderRadius: radii.sm });
  });
});
