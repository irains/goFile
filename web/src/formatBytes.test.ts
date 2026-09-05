import { describe, expect, it } from 'vitest';
import { formatBytes } from './formatBytes';

describe('formatBytes', () => {
  it('uses stable SI units instead of locale-specific compact byte notation', () => {
    expect(formatBytes(999, 'en-US')).toBe('999 B');
    expect(formatBytes(1_000, 'en-US')).toBe('1 kB');
    expect(formatBytes(1_024, 'en-US')).toBe('1.02 kB');
    expect(formatBytes(6_370_000, 'en-US')).toBe('6.37 MB');
    expect(formatBytes(6_370_000, 'zh-CN')).toBe('6.37 MB');
  });

  it('promotes values that round into the next unit', () => {
    expect(formatBytes(999_999, 'en-US')).toBe('1 MB');
    expect(formatBytes(1_000_000, 'en-US')).toBe('1 MB');
    expect(formatBytes(1_500_000_000, 'en-US')).toBe('1.5 GB');
  });

  it('handles invalid values as empty byte counts', () => {
    expect(formatBytes(-1, 'en-US')).toBe('0 B');
    expect(formatBytes(Number.NaN, 'en-US')).toBe('0 B');
  });
});
