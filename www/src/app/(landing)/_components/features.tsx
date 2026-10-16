import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import {
  CompassIcon,
  KeyboardIcon,
  MessagesSquareIcon,
  NewspaperIcon,
  SparklesIcon,
  SunriseIcon,
  type LucideIcon,
} from 'lucide-react';

type Feature = {
  icon: LucideIcon;
  title: string;
  body: string;
  live: boolean;
  className?: string;
};

const features: Feature[] = [
  {
    icon: CompassIcon,
    title: 'Find any feed',
    body: 'Paste a website address and Feedoku finds its feed for you. Coming from another reader? Import your OPML file in one click.',
    live: true,
    className: 'lg:col-span-2',
  },
  {
    icon: KeyboardIcon,
    title: 'Keyboard-first',
    body: 'J and K to move, S to save, M to toggle read, O to open the original. Never touch the mouse.',
    live: true,
  },
  {
    icon: NewspaperIcon,
    title: 'A reader that gets out of the way',
    body: 'Clean serif typography, dark mode, and full-text extraction when a feed only sends a teaser.',
    live: true,
  },
  {
    icon: SparklesIcon,
    title: 'AI summaries & topics',
    body: 'A TL;DR and key points on every article, automatically tagged so you can decide in seconds what deserves your time.',
    live: false,
    className: 'lg:col-span-2',
  },
  {
    icon: SunriseIcon,
    title: 'Your daily briefing',
    body: 'A morning digest of the stories that matter across your feeds, grouped by theme, with links back to every source.',
    live: false,
  },
  {
    icon: MessagesSquareIcon,
    title: 'Ask your feeds',
    body: 'Ask “what did people say about the new GPUs this week?” and get a streamed answer with citations. Search that understands meaning, too.',
    live: false,
    className: 'lg:col-span-2',
  },
];

export function Features() {
  return (
    <section id='features' className='scroll-mt-24 px-4 py-24'>
      <div className='mx-auto max-w-5xl'>
        <p className='text-sm font-semibold text-brand'>Features</p>
        <h2 className='mt-2 max-w-2xl text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl'>
          Everything you follow, finally manageable.
        </h2>
        <div className='mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
          {features.map(({ icon: Icon, title, body, live, className }) => (
            <article
              key={title}
              className={cn(
                'rounded-3xl border bg-card/70 p-7 backdrop-blur transition-shadow hover:shadow-[var(--shadow-panel)]',
                className
              )}
            >
              <div className='flex items-start justify-between'>
                <span className='grid size-11 place-items-center rounded-2xl bg-accent text-accent-foreground'>
                  <Icon className='size-5' />
                </span>
                <Badge
                  variant={live ? 'default' : 'secondary'}
                  className='rounded-full px-2.5'
                >
                  {live ? 'Available' : 'Coming soon'}
                </Badge>
              </div>
              <h3 className='mt-6 text-xl font-bold tracking-tight'>{title}</h3>
              <p className='mt-2 leading-relaxed text-muted-foreground'>
                {body}
              </p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
