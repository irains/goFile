import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';
import { Mark } from './Mark';

describe('Mark', () => {
  it('renders an SVG with stroke=currentColor', () => {
    const { container } = render(<Mark />);
    const svg = container.querySelector('svg');
    expect(svg).toBeTruthy();
    expect(svg?.getAttribute('stroke')).toBe('currentColor');
  });

  it('inherits color by default and accepts valid CSS colors', () => {
    const { container, rerender } = render(<Mark size={32} />);
    const svg = container.querySelector('svg') as SVGElement;
    expect(svg.style.fontSize).toBe('32px');
    expect(svg.style.color).toBe('currentcolor');

    rerender(<Mark size={32} color="var(--mui-palette-primary-main)" />);
    expect(svg.style.color).toBe('var(--mui-palette-primary-main)');
  });

  it('renders a title when provided', () => {
    const { container } = render(<Mark title="FileHarbor mark" />);
    const svg = container.querySelector('svg');
    expect(container.querySelector('title')?.textContent).toBe('FileHarbor mark');
    expect(svg?.getAttribute('role')).toBe('img');
    expect(svg?.getAttribute('aria-label')).toBe('FileHarbor mark');
  });

  it('is decorative when no title is provided', () => {
    const { container } = render(<Mark />);
    expect(container.querySelector('svg')?.getAttribute('aria-hidden')).toBe('true');
  });
});
