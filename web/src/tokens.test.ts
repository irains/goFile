import { describe, expect, it } from 'vitest';
import { fontFamilyMono, radii, semantic, spacing, surface, motion } from './tokens';

describe('design tokens', () => {
  it('exposes a 4 px spacing grid', () => {
    expect(spacing.xxs).toBe(4);
    expect(spacing.md).toBe(16);
    expect(spacing.xxl).toBe(32);
  });

  it('exposes a coherent radii scale', () => {
    expect(radii.none).toBe(0);
    expect(radii.md).toBeGreaterThan(radii.sm);
    expect(radii.lg).toBeGreaterThan(radii.md);
    expect(radii.pill).toBeGreaterThan(radii.lg);
  });

  it('exposes motion durations ordered fastest to slowest', () => {
    expect(motion.instant).toBeLessThan(motion.fast);
    expect(motion.fast).toBeLessThan(motion.base);
    expect(motion.base).toBeLessThanOrEqual(motion.slow);
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
    expect(surface).toMatchObject({ borderRadius: radii.md, bgcolor: 'background.paper' });
  });
});