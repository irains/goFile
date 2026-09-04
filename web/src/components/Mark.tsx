import type { CSSProperties } from 'react';

export type MarkProps = {
  size?: number;
  color?: CSSProperties['color'];
  className?: string;
  title?: string;
};

// Single-stroke harbor mark: two pilings under a horizontal cap.
// The SVG inherits a valid CSS color from its parent by default.
export function Mark({ size = 24, color = 'currentColor', className, title }: MarkProps) {
  const style: CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    color,
    fontSize: `${size}px`,
    lineHeight: 0,
    verticalAlign: 'middle'
  };
  return (
    <svg
      viewBox="0 0 24 24"
      width="1em"
      height="1em"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      style={style}
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      {title && <title>{title}</title>}
      <path d="M3 9 H21" />
      <path d="M6 9 V19" />
      <path d="M11 9 V19" />
      <path d="M16 9 V19" />
      <path d="M3 21 H21" />
    </svg>
  );
}
