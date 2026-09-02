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

  it('renders with an explicit color and size', () => {
    const { container } = render(<Mark size={32} color="#6252ba" />);
    const svg = container.querySelector('svg') as SVGElement;
    expect(svg.style.fontSize).toBe('32px');
    expect(svg.style.color).toBe('rgb(98, 82, 186)');
  });

  it('renders a title when provided', () => {
    const { container } = render(<Mark title="FileHarbor mark" />);
    expect(container.querySelector('title')?.textContent).toBe('FileHarbor mark');
  });
});