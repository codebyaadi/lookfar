'use client';

import { ModeToggle } from '@/components/mode-toggle';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import type { User } from '@/lib/api';
import { authClient } from '@/lib/auth-client';
import { cn } from '@/lib/utils';
import { api, type ArticleFilter, type Feed } from '@/lib/reader-api';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  BookmarkIcon,
  InboxIcon,
  LogOutIcon,
  NewspaperIcon,
  TrashIcon,
} from 'lucide-react';
import { useRouter } from 'next/navigation';
import { useMemo } from 'react';
import { toast } from 'sonner';
import { AddFeedDialog } from './add-feed-dialog';

type Selection = { filter?: ArticleFilter; feed?: number | null };

type Props = {
  user: User;
  feeds: Feed[];
  loading: boolean;
  filter: ArticleFilter;
  feedId?: number;
  mobileOpen: boolean;
  onMobileOpenChange: (open: boolean) => void;
  onSelect: (next: Selection) => void;
};

/** Desktop: static column. Mobile: the same content inside a Sheet. */
export function Sidebar(props: Props) {
  return (
    <>
      <aside className='hidden w-64 shrink-0 flex-col md:flex'>
        <SidebarContent {...props} />
      </aside>
      <Sheet open={props.mobileOpen} onOpenChange={props.onMobileOpenChange}>
        <SheetContent side='left' className='w-72 gap-0 p-0'>
          <SheetTitle className='sr-only'>Navigation</SheetTitle>
          <SheetDescription className='sr-only'>
            Feeds and filters
          </SheetDescription>
          <SidebarContent {...props} />
        </SheetContent>
      </Sheet>
    </>
  );
}

function NavItem({
  active,
  icon,
  count,
  children,
  onClick,
}: {
  active: boolean;
  icon?: React.ReactNode;
  count?: number;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Button
      variant='ghost'
      onClick={onClick}
      aria-current={active ? 'page' : undefined}
      className={cn(
        'h-9 w-full justify-start gap-2 px-2.5 font-normal text-muted-foreground',
        active && 'bg-accent font-medium text-accent-foreground'
      )}
    >
      {icon}
      <span className='min-w-0 flex-1 truncate text-left'>{children}</span>
      {!!count && <span className='text-xs tabular-nums'>{count}</span>}
    </Button>
  );
}

function SidebarContent({
  user,
  feeds,
  loading,
  filter,
  feedId,
  onMobileOpenChange,
  onSelect,
}: Props) {
  const router = useRouter();
  const qc = useQueryClient();

  const select = (next: Selection) => {
    onSelect(next);
    onMobileOpenChange(false);
  };

  const remove = useMutation({
    mutationFn: api.removeFeed,
    onSuccess: (_, id) => {
      toast.success('Unsubscribed');
      if (id === feedId) onSelect({ feed: null });
      qc.invalidateQueries({ queryKey: ['feeds'] });
      qc.invalidateQueries({ queryKey: ['articles'] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const groups = useMemo(() => {
    const map = new Map<string, Feed[]>();
    for (const f of feeds)
      map.set(f.folder ?? '', [...(map.get(f.folder ?? '') ?? []), f]);
    return [...map.entries()].toSorted(([a], [b]) =>
      a === '' ? 1 : b === '' ? -1 : a.localeCompare(b)
    );
  }, [feeds]);

  const totalUnread = feeds.reduce((n, f) => n + f.unreadCount, 0);

  return (
    <div className='flex h-full min-h-0 flex-col'>
      <div className='flex h-14 items-center justify-between px-4 md:px-2'>
        <span className='flex items-center gap-2 text-base font-semibold tracking-tight'>
          <span className='grid size-7 place-items-center rounded-lg bg-primary text-primary-foreground'>
            <NewspaperIcon className='size-4' />
          </span>
          Feedoku
        </span>
        <ModeToggle />
      </div>

      <ScrollArea className='min-h-0 flex-1'>
        <nav className='space-y-5 px-3 py-2 md:px-0' aria-label='Library'>
          <div className='space-y-0.5'>
            <NavItem
              active={!feedId && filter === 'all'}
              icon={<NewspaperIcon className='size-4' />}
              onClick={() => select({ filter: 'all', feed: null })}
            >
              All articles
            </NavItem>
            <NavItem
              active={!feedId && filter === 'unread'}
              icon={<InboxIcon className='size-4' />}
              count={totalUnread}
              onClick={() => select({ filter: 'unread', feed: null })}
            >
              Unread
            </NavItem>
            <NavItem
              active={!feedId && filter === 'saved'}
              icon={<BookmarkIcon className='size-4' />}
              onClick={() => select({ filter: 'saved', feed: null })}
            >
              Saved
            </NavItem>
          </div>

          <Separator />

          <div>
            <div className='mb-1 flex items-center justify-between pl-2.5'>
              <h2 className='text-xs font-medium tracking-wider text-muted-foreground uppercase'>
                Feeds
              </h2>
              <AddFeedDialog />
            </div>
            {loading && (
              <div className='space-y-2 px-2.5 pt-1'>
                <Skeleton className='h-4 w-28' />
                <Skeleton className='h-4 w-20' />
              </div>
            )}
            {groups.map(([folder, items]) => (
              <div key={folder} className='mt-3 first:mt-0'>
                {folder && (
                  <h3 className='px-2.5 py-1 text-xs text-muted-foreground'>
                    {folder}
                  </h3>
                )}
                <ul className='space-y-0.5'>
                  {items.map((f) => (
                    <li key={f.id} className='group relative'>
                      <NavItem
                        active={feedId === f.id}
                        count={f.unreadCount}
                        onClick={() => select({ feed: f.id, filter: 'all' })}
                      >
                        <span
                          className={cn(f.error && 'text-destructive')}
                          title={
                            f.error ? `Last fetch failed: ${f.error}` : f.title
                          }
                        >
                          {f.title}
                        </span>
                      </NavItem>
                      <Button
                        variant='ghost'
                        size='icon'
                        aria-label={`Unsubscribe from ${f.title}`}
                        className='absolute top-1/2 right-1 hidden size-7 -translate-y-1/2 bg-card text-muted-foreground group-hover:inline-flex hover:text-destructive focus-visible:inline-flex'
                        onClick={() => {
                          if (confirm(`Unsubscribe from "${f.title}"?`))
                            remove.mutate(f.id);
                        }}
                      >
                        <TrashIcon className='size-3.5' />
                      </Button>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </nav>
      </ScrollArea>

      <div className='flex items-center gap-2 border-t p-3 md:border-t-0 md:px-2'>
        <div className='min-w-0 flex-1'>
          <p className='truncate text-sm font-medium'>{user.name}</p>
          <p className='truncate text-xs text-muted-foreground'>{user.email}</p>
        </div>
        <Button
          variant='ghost'
          size='icon'
          aria-label='Sign out'
          className='text-muted-foreground'
          onClick={async () => {
            await authClient.signOut();
            router.push('/signin');
          }}
        >
          <LogOutIcon className='size-4' />
        </Button>
      </div>
    </div>
  );
}
