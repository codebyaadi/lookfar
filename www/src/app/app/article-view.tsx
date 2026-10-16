'use client';

import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { timeAgo, type ArticleDetail } from '@/lib/reader-api';
import { cn } from '@/lib/utils';
import {
  ArrowLeftIcon,
  BookmarkIcon,
  CircleDotIcon,
  ExternalLinkIcon,
} from 'lucide-react';

type Props = {
  article?: ArticleDetail;
  loading: boolean;
  error: boolean;
  onBack: () => void;
  onToggleSaved: (a: ArticleDetail) => void;
  onToggleRead: (a: ArticleDetail) => void;
};

function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className='rounded border bg-muted px-1.5 py-0.5 font-mono text-[11px]'>
      {children}
    </kbd>
  );
}

function IconAction({
  label,
  shortcut,
  children,
  ...props
}: React.ComponentProps<typeof Button> & { label: string; shortcut?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant='ghost'
          size='icon'
          aria-label={label}
          className='text-muted-foreground'
          {...props}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        {label} {shortcut && <Kbd>{shortcut}</Kbd>}
      </TooltipContent>
    </Tooltip>
  );
}

export function ArticleView({
  article,
  loading,
  error,
  onBack,
  onToggleSaved,
  onToggleRead,
}: Props) {
  if (loading)
    return (
      <div className='mx-auto w-full max-w-2xl space-y-4 p-8' aria-busy>
        <Skeleton className='h-8 w-3/4' />
        <Skeleton className='h-4 w-1/3' />
        <Skeleton className='h-64 w-full' />
      </div>
    );

  if (error)
    return (
      <p className='p-8 text-sm text-destructive'>
        Couldn&apos;t load this article.
      </p>
    );

  if (!article)
    return (
      <div className='flex flex-1 items-center justify-center p-8 text-center text-sm text-muted-foreground'>
        <div className='space-y-3'>
          <p>Select an article to read.</p>
          <p className='flex flex-wrap items-center justify-center gap-x-3 gap-y-1.5 text-xs'>
            <span>
              <Kbd>j</Kbd> <Kbd>k</Kbd> navigate
            </span>
            <span>
              <Kbd>s</Kbd> save
            </span>
            <span>
              <Kbd>m</Kbd> read / unread
            </span>
            <span>
              <Kbd>o</Kbd> open original
            </span>
          </p>
        </div>
      </div>
    );

  return (
    <>
      <header className='flex h-14 shrink-0 items-center gap-1 border-b px-3'>
        <Button
          variant='ghost'
          size='icon'
          aria-label='Back to list'
          className='lg:hidden'
          onClick={onBack}
        >
          <ArrowLeftIcon className='size-4' />
        </Button>
        <div className='flex-1' />
        <IconAction
          label={article.read ? 'Mark unread' : 'Mark read'}
          shortcut='m'
          onClick={() => onToggleRead(article)}
        >
          <CircleDotIcon
            className={cn('size-4', !article.read && 'text-brand')}
          />
        </IconAction>
        <IconAction
          label={article.saved ? 'Unsave' : 'Save'}
          shortcut='s'
          aria-pressed={article.saved}
          onClick={() => onToggleSaved(article)}
        >
          <BookmarkIcon
            className={cn(
              'size-4',
              article.saved && 'fill-current text-foreground'
            )}
          />
        </IconAction>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              asChild
              variant='ghost'
              size='icon'
              className='text-muted-foreground'
            >
              <a
                href={article.url}
                target='_blank'
                rel='noopener noreferrer'
                aria-label='Open original'
              >
                <ExternalLinkIcon className='size-4' />
              </a>
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            Open original <Kbd>o</Kbd>
          </TooltipContent>
        </Tooltip>
      </header>
      <ScrollArea className='min-h-0 flex-1'>
        <article className='mx-auto max-w-2xl px-5 py-8 sm:px-8'>
          <p className='text-sm text-muted-foreground'>
            {article.feedTitle}
            {article.author && ` · ${article.author}`} ·{' '}
            <time dateTime={article.publishedAt}>
              {new Date(article.publishedAt).toLocaleDateString(undefined, {
                dateStyle: 'medium',
              })}{' '}
              ({timeAgo(article.publishedAt)} ago)
            </time>
          </p>
          <h1 className='mt-2 text-2xl leading-tight font-semibold tracking-tight sm:text-3xl'>
            {article.title}
          </h1>
          {/* contentHtml is sanitised server-side (bluemonday) before storage. */}
          <div
            className='article-body mt-6'
            dangerouslySetInnerHTML={{
              __html:
                article.contentHtml || `<p>${escapeHtml(article.summary)}</p>`,
            }}
          />
          <Button
            asChild
            variant='link'
            className='mt-8 h-auto p-0 text-muted-foreground'
          >
            <a href={article.url} target='_blank' rel='noopener noreferrer'>
              Read on the original site{' '}
              <ExternalLinkIcon className='size-3.5' />
            </a>
          </Button>
        </article>
      </ScrollArea>
    </>
  );
}

function escapeHtml(s: string) {
  return s.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`);
}
