'use client';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { timeAgo, type ArticleSummary } from '@/lib/reader-api';
import { BookmarkIcon } from 'lucide-react';

type Props = {
  articles: ArticleSummary[];
  selectedId?: number;
  loading: boolean;
  error: boolean;
  hasMore: boolean;
  loadingMore: boolean;
  emptyHint: string;
  onLoadMore: () => void;
  onSelect: (id: number) => void;
};

export function ArticleList({
  articles,
  selectedId,
  loading,
  error,
  hasMore,
  loadingMore,
  emptyHint,
  onLoadMore,
  onSelect,
}: Props) {
  if (loading)
    return (
      <div aria-busy className='divide-y'>
        {Array.from({ length: 7 }, (_, i) => (
          <div key={`sk-${i}`} className='space-y-2 p-4'>
            <Skeleton className='h-3 w-1/3' />
            <Skeleton className='h-4 w-5/6' />
            <Skeleton className='h-3 w-2/3' />
          </div>
        ))}
      </div>
    );

  if (error)
    return (
      <p className='p-6 text-sm text-destructive'>
        Couldn&apos;t load articles. Try refreshing.
      </p>
    );

  if (!articles.length)
    return <p className='p-6 text-sm text-muted-foreground'>{emptyHint}</p>;

  return (
    <ScrollArea className='min-h-0 flex-1'>
      <ul>
        {articles.map((a) => (
          <li key={a.id} className='border-b last:border-b-0'>
            <button
              type='button'
              onClick={() => onSelect(a.id)}
              aria-current={a.id === selectedId}
              className={cn(
                'block w-full px-4 py-3.5 text-left transition-colors outline-none hover:bg-accent/50 focus-visible:bg-accent/50',
                a.id === selectedId && 'bg-accent'
              )}
            >
              <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                {!a.read && (
                  <span className='size-1.5 shrink-0 rounded-full bg-brand'>
                    <span className='sr-only'>Unread</span>
                  </span>
                )}
                <span className='truncate'>{a.feedTitle}</span>
                <span aria-hidden>·</span>
                <time dateTime={a.publishedAt} className='shrink-0'>
                  {timeAgo(a.publishedAt)}
                </time>
                {a.saved && (
                  <Badge
                    variant='secondary'
                    className='ml-auto gap-1 px-1.5 py-0 text-[10px]'
                  >
                    <BookmarkIcon className='size-3 fill-current' /> Saved
                  </Badge>
                )}
              </div>
              <h2
                className={cn(
                  'mt-1 line-clamp-2 text-sm leading-snug',
                  a.read ? 'text-muted-foreground' : 'font-semibold'
                )}
              >
                {a.title}
              </h2>
              {a.excerpt && (
                <p className='mt-1 line-clamp-2 text-xs text-muted-foreground'>
                  {a.excerpt}
                </p>
              )}
            </button>
          </li>
        ))}
        {hasMore && (
          <li className='p-3 text-center'>
            <Button
              variant='ghost'
              size='sm'
              disabled={loadingMore}
              onClick={onLoadMore}
              className='text-muted-foreground'
            >
              {loadingMore ? 'Loading…' : 'Load more'}
            </Button>
          </li>
        )}
      </ul>
    </ScrollArea>
  );
}
