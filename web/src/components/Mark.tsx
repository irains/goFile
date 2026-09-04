import type { CSSProperties } from 'react';

export type MarkProps = {
  size?: number;
  color?: CSSProperties['color'];
  className?: string;
  title?: string;
};

// Two-piling harbor mark, optically centered for compact application rails.
// The SVG inherits a valid CSS color from its parent by default.
export function Mark({ size = 24, color = 'currentColor', className, title }: MarkProps) {
  const style: CSSProperties = {
    display: 'block',
    color,
    flex: '0 0 auto'
  };
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      style={style}
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      {title && <title>{title}</title>}
      <path d="M4 7 H20" />
      <path d="M7 7 V17" />
      <path d="M17 7 V17" />
      <path d="M4 19 H20" />
    </svg>
  );
}
