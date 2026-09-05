const units = ['B', 'kB', 'MB', 'GB', 'TB', 'PB'] as const;

function fractionDigits(value: number) {
  if (value >= 100) return 0;
  if (value >= 10) return 1;
  return 2;
}

function rounded(value: number) {
  const digits = fractionDigits(value);
  const factor = 10 ** digits;
  return Math.round(value * factor) / factor;
}

export function formatBytes(input: number, locale?: string) {
  let value = Number.isFinite(input) ? Math.max(0, input) : 0;
  let unitIndex = 0;

  while (value >= 1_000 && unitIndex < units.length - 1) {
    value /= 1_000;
    unitIndex += 1;
  }

  if (rounded(value) >= 1_000 && unitIndex < units.length - 1) {
    value /= 1_000;
    unitIndex += 1;
  }

  const digits = fractionDigits(value);
  const amount = new Intl.NumberFormat(locale, {
    maximumFractionDigits: digits,
    minimumFractionDigits: 0,
    useGrouping: false
  }).format(value);

  return `${amount} ${units[unitIndex]}`;
}
