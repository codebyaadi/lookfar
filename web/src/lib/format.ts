const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
];

const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

/** "5 minutes ago", "in 2 hours", "just now". */
export function timeAgo(iso: string | null, now = Date.now()): string {
  if (!iso) return 'never';
  const secs = Math.round((new Date(iso).getTime() - now) / 1000);
  const abs = Math.abs(secs);
  if (abs < 45) return secs > 0 ? 'in a few seconds' : 'just now';
  for (const [unit, size] of units) {
    if (abs >= size) return rtf.format(Math.round(secs / size), unit);
  }
  return rtf.format(Math.round(secs / 60), 'minute');
}

export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('en', {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

export const intervalOptions = [
  { secs: 900, label: 'Every 15 minutes' },
  { secs: 3600, label: 'Every hour' },
  { secs: 6 * 3600, label: 'Every 6 hours' },
  { secs: 24 * 3600, label: 'Daily' },
  { secs: 7 * 24 * 3600, label: 'Weekly' },
];

export function intervalLabel(secs: number): string {
  return (
    intervalOptions.find((o) => o.secs === secs)?.label ??
    `Every ${Math.round(secs / 60)} minutes`
  );
}

export const kindLabels = {
  page: 'Web page',
  rss: 'RSS / Atom feed',
  github: 'GitHub releases',
} as const;

export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}
