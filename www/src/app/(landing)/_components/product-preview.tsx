import { Badge } from '@/components/ui/badge';
import {
  BookmarkIcon,
  InboxIcon,
  NewspaperIcon,
  SparklesIcon,
} from 'lucide-react';

const items = [
  {
    feed: 'The Go Blog',
    time: '2d',
    title: 'Better HTTP/1 connection reuse',
    unread: true,
  },
  {
    feed: 'Hacker News',
    time: '4h',
    title: 'New Withings users must pay $99/year for Apple Health syncing',
    unread: true,
  },
  {
    feed: 'Stratechery',
    time: '6h',
    title: 'The agent economy and the end of the browser',
    unread: false,
  },
  {
    feed: 'Quanta',
    time: '1d',
    title: 'A new proof reframes how we count prime gaps',
    unread: true,
  },
];

/** Static illustration of the reader. Not interactive. */
export function ProductPreview() {
  return (
    <div
      aria-hidden
      className='overflow-hidden rounded-3xl border bg-card/70 p-2 shadow-[var(--shadow-panel)] backdrop-blur-xl'
    >
      <div className='mb-2 flex items-center gap-1.5 px-3 pt-1.5'>
        <span className='size-2.5 rounded-full bg-border' />
        <span className='size-2.5 rounded-full bg-border' />
        <span className='size-2.5 rounded-full bg-border' />
      </div>
      <div className='grid gap-2 md:grid-cols-[13rem_18rem_1fr]'>
        <div className='hidden space-y-1 rounded-2xl bg-muted p-3 text-sm md:block'>
          <div className='flex items-center gap-2 rounded-full bg-accent px-3 py-1.5 font-medium text-accent-foreground'>
            <NewspaperIcon className='size-4' /> All articles
          </div>
          <div className='flex items-center gap-2 px-3 py-1.5 text-muted-foreground'>
            <InboxIcon className='size-4' /> Unread
            <span className='ml-auto text-xs'>24</span>
          </div>
          <div className='flex items-center gap-2 px-3 py-1.5 text-muted-foreground'>
            <BookmarkIcon className='size-4' /> Saved
          </div>
          <p className='px-3 pt-4 text-[10px] font-medium tracking-wider text-muted-foreground uppercase'>
            Feeds
          </p>
          {['The Go Blog', 'Hacker News', 'Stratechery', 'Quanta'].map((f) => (
            <div key={f} className='px-3 py-1 text-muted-foreground'>
              {f}
            </div>
          ))}
        </div>

        <div className='hidden rounded-2xl bg-muted/60 md:block'>
          {items.map((it, i) => (
            <div
              key={it.title}
              className={`border-b px-4 py-3 last:border-0 ${i === 0 ? 'rounded-t-2xl bg-accent' : ''}`}
            >
              <p className='flex items-center gap-1.5 text-[11px] text-muted-foreground'>
                {it.unread && (
                  <span className='size-1.5 rounded-full bg-brand' />
                )}
                {it.feed} · {it.time}
              </p>
              <p
                className={`mt-0.5 line-clamp-2 text-sm leading-snug ${it.unread ? 'font-semibold' : 'text-muted-foreground'}`}
              >
                {it.title}
              </p>
            </div>
          ))}
        </div>

        <div className='rounded-2xl bg-background/70 p-6 text-left sm:p-8'>
          <p className='text-xs text-muted-foreground'>
            The Go Blog · 2 days ago
          </p>
          <h3 className='mt-1.5 text-2xl leading-tight font-bold tracking-tight'>
            Better HTTP/1 connection reuse
          </h3>
          <div className='mt-4 rounded-2xl border bg-accent/60 p-4'>
            <p className='flex items-center gap-1.5 text-xs font-semibold text-accent-foreground'>
              <SparklesIcon className='size-3.5' /> TL;DR
              <Badge
                variant='secondary'
                className='ml-1 rounded-full px-2 py-0 text-[10px]'
              >
                Coming soon
              </Badge>
            </p>
            <p className='mt-2 text-sm leading-relaxed'>
              Go 1.27 reuses idle HTTP/1 connections more aggressively, cutting
              latency for programs that send many sequential requests to the
              same host.
            </p>
          </div>
          <div className='mt-4 space-y-2.5'>
            <div className='h-2.5 w-full rounded-full bg-muted' />
            <div className='h-2.5 w-[92%] rounded-full bg-muted' />
            <div className='h-2.5 w-[78%] rounded-full bg-muted' />
          </div>
        </div>
      </div>
    </div>
  );
}
