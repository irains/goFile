import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Button } from '@mui/material';
import { EmptyState } from './EmptyState';

describe('EmptyState', () => {
  it('renders icon, title, and caption', () => {
    render(<EmptyState icon={<span data-testid="icon" />} title="Empty" caption="Try uploading" />);
    expect(screen.getByTestId('icon')).toBeInTheDocument();
    expect(screen.getByText('Empty')).toBeInTheDocument();
    expect(screen.getByText('Try uploading')).toBeInTheDocument();
  });

  it('renders an optional CTA', () => {
    render(<EmptyState icon={<span />} title="x" action={<Button>Click me</Button>} />);
    expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument();
  });
});