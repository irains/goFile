import logoUrl from '../assets/fileharbor-logo.svg?no-inline';

export type MarkProps = {
  size?: number;
  className?: string;
  title?: string;
};

// The bundled mark keeps its authored colors across the application's schemes.
export function Mark({ size = 24, className, title }: MarkProps) {
  return (
    <img
      src={logoUrl}
      width={size}
      height={size}
      className={className}
      alt={title ?? ''}
      aria-hidden={title ? undefined : true}
      style={{ display: 'block', flex: '0 0 auto' }}
    />
  );
}
