import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Mark } from './Mark';

describe('Mark', () => {
  it('renders the bundled image at its default dimensions', () => {
    const { container } = render(<Mark />);
    const image = container.querySelector('img');
    expect(image).toBeTruthy();
    expect(image?.getAttribute('src')).toContain('fileharbor-logo');
    expect(image?.getAttribute('width')).toBe('24');
    expect(image?.getAttribute('height')).toBe('24');
  });

  it('forwards custom sizing and classes', () => {
    const { container } = render(<Mark size={32} className="brand-mark" />);
    const image = container.querySelector('img');
    expect(image?.getAttribute('width')).toBe('32');
    expect(image?.getAttribute('height')).toBe('32');
    expect(image).toHaveClass('brand-mark');
  });

  it('exposes an accessible image when titled', () => {
    render(<Mark title="FileHarbor logo" />);
    expect(screen.getByRole('img', { name: 'FileHarbor logo' })).toBeTruthy();
  });

  it('is decorative when no title is provided', () => {
    const { container } = render(<Mark />);
    const image = container.querySelector('img');
    expect(image?.getAttribute('alt')).toBe('');
    expect(image?.getAttribute('aria-hidden')).toBe('true');
    expect(screen.queryByRole('img')).toBeNull();
  });
});
