import { cn } from '@/lib/utils';

type LogoProps = {
  className?: string;
  /** `inverted` is for use on a `bg-primary` surface. */
  tone?: 'default' | 'inverted';
  /** Show only the mark. */
  markOnly?: boolean;
};

/** Heavy RSS-signal glyph with an AI spark. Uses currentColor. */
export function LogoGlyph({ className }: { className?: string }) {
  return (
    <svg
      viewBox='0 0 32 32'
      fill='none'
      aria-hidden
      className={className}
      strokeLinecap='round'
    >
      <circle cx='9.5' cy='22.5' r='3.6' fill='currentColor' />
      <path
        d='M6.5 12.2a13.3 13.3 0 0 1 13.3 13.3'
        stroke='currentColor'
        strokeWidth='4.6'
      />
      <path
        d='M6.5 4.6A20.9 20.9 0 0 1 27.4 25.5'
        stroke='currentColor'
        strokeWidth='4.6'
      />
    </svg>
  );
}

export function Logo({ className, tone = 'default', markOnly }: LogoProps) {
  const inverted = tone === 'inverted';
  return (
    <span className={cn('inline-flex items-center gap-2.5', className)}>
      <span
        className={cn(
          'grid size-9 shrink-0 place-items-center rounded-xl shadow-sm',
          inverted
            ? 'bg-primary-foreground text-primary'
            : 'bg-primary text-primary-foreground'
        )}
      >
        <LogoGlyph className='size-6' />
      </span>
      {!markOnly && (
        <span className='text-[1.4rem] leading-none font-extrabold tracking-tighter'>
          Feedoku
          <span className={inverted ? 'opacity-60' : 'text-brand'}>.</span>
        </span>
      )}
      {markOnly && <span className='sr-only'>Feedoku</span>}
    </span>
  );
}
