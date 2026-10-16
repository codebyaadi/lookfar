'use client';

import {
  api,
  type ArticleDetail,
  type ArticleFilter,
  type ArticlePage,
  type ArticleSummary,
} from '@/lib/reader-api';
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
} from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { MenuIcon } from 'lucide-react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import type { User } from '@/lib/api';
import { ArticleList } from './article-list';
import { ArticleView } from './article-view';
import { Sidebar } from './sidebar';

const FILTERS = new Set<string>(['all', 'unread', 'saved']);

export function Reader({ user }: { user: User }) {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const qc = useQueryClient();
  const [menuOpen, setMenuOpen] = useState(false);

  const filter = FILTERS.has(params.get('filter') ?? '')
    ? (params.get('filter') as ArticleFilter)
    : 'all';
  const feedId = Number(params.get('feed')) || undefined;
  const articleId = Number(params.get('a')) || undefined;

  const navigate = useCallback(
    (next: {
      filter?: ArticleFilter;
      feed?: number | null;
      a?: number | null;
    }) => {
      const q = new URLSearchParams(params.toString());
      if (next.filter !== undefined) q.set('filter', next.filter);
      if (next.feed !== undefined) {
        if (next.feed) q.set('feed', String(next.feed));
        else q.delete('feed');
      }
      if (next.a !== undefined) {
        if (next.a) q.set('a', String(next.a));
        else q.delete('a');
      }
      router.replace(`${pathname}?${q}`, { scroll: false });
    },
    [params, pathname, router]
  );

  const feeds = useQuery({ queryKey: ['feeds'], queryFn: api.feeds });

  const list = useInfiniteQuery({
    queryKey: ['articles', { filter, feedId }],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam }) =>
      api.articles({ filter, feed: feedId, cursor: pageParam }),
    getNextPageParam: (last) => last.nextCursor,
  });
  const articles = useMemo(
    () => list.data?.pages.flatMap((p) => p.articles) ?? [],
    [list.data]
  );

  const detail = useQuery({
    queryKey: ['article', articleId],
    queryFn: () => api.article(articleId!),
    enabled: !!articleId,
  });

  // Patch read/saved flags in every cached list and the open article so the UI
  // updates instantly, without refetching (which would drop a just-read item
  // out of the Unread list while the user is still reading it).
  const patchArticle = useCallback(
    (id: number, patch: Partial<Pick<ArticleSummary, 'read' | 'saved'>>) => {
      qc.setQueriesData<InfiniteData<ArticlePage>>(
        { queryKey: ['articles'] },
        (data) =>
          data && {
            ...data,
            pages: data.pages.map((p) => ({
              ...p,
              articles: p.articles.map((a) =>
                a.id === id ? { ...a, ...patch } : a
              ),
            })),
          }
      );
      qc.setQueryData<ArticleDetail>(
        ['article', id],
        (a) => a && { ...a, ...patch }
      );
    },
    [qc]
  );

  const setState = useMutation({
    mutationFn: ({
      id,
      ...state
    }: {
      id: number;
      read?: boolean;
      saved?: boolean;
    }) => api.setState(id, state),
    onMutate: ({ id, ...patch }) => patchArticle(id, patch),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['feeds'] }),
    onError: (err: Error) => {
      toast.error(err.message);
      qc.invalidateQueries({ queryKey: ['articles'] });
    },
  });

  const markAllRead = useMutation({
    mutationFn: () => api.markAllRead(feedId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['articles'] });
      qc.invalidateQueries({ queryKey: ['feeds'] });
      qc.invalidateQueries({ queryKey: ['article'] });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  // Opening an article marks it read.
  const openedId = detail.data?.id;
  const openedRead = detail.data?.read;
  useEffect(() => {
    if (openedId && openedRead === false)
      setState.mutate({ id: openedId, read: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openedId, openedRead]);

  const select = useCallback(
    (delta: number) => {
      if (!articles.length) return;
      const i = articles.findIndex((a) => a.id === articleId);
      const next =
        articles[Math.min(Math.max(i + delta, 0), articles.length - 1)];
      if (next) navigate({ a: next.id });
      if (
        i + delta >= articles.length - 3 &&
        list.hasNextPage &&
        !list.isFetchingNextPage
      )
        list.fetchNextPage();
    },
    [articles, articleId, navigate, list]
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (
        e.metaKey ||
        e.ctrlKey ||
        e.altKey ||
        t.closest('input, textarea, [contenteditable], [role="dialog"]')
      )
        return;
      const current = detail.data;
      switch (e.key) {
        case 'j':
          select(1);
          break;
        case 'k':
          select(-1);
          break;
        case 's':
          if (current)
            setState.mutate({ id: current.id, saved: !current.saved });
          break;
        case 'm':
          if (current) setState.mutate({ id: current.id, read: !current.read });
          break;
        case 'o':
          if (current)
            window.open(current.url, '_blank', 'noopener,noreferrer');
          break;
        case 'Escape':
          navigate({ a: null });
          break;
        default:
          return;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [select, detail.data, setState, navigate]);

  const activeFeed = feeds.data?.find((f) => f.id === feedId);
  const heading =
    activeFeed?.title ??
    { all: 'All articles', unread: 'Unread', saved: 'Saved' }[filter];

  return (
    <div className='ambient flex h-dvh gap-2 overflow-hidden p-0 md:p-2'>
      <Sidebar
        user={user}
        feeds={feeds.data ?? []}
        loading={feeds.isLoading}
        filter={filter}
        feedId={feedId}
        mobileOpen={menuOpen}
        onMobileOpenChange={setMenuOpen}
        onSelect={(next) => navigate({ ...next, a: null })}
      />

      <section
        className={`${articleId ? 'hidden lg:flex' : 'flex'} w-full shrink-0 flex-col overflow-hidden border bg-card/70 shadow-[var(--shadow-panel)] backdrop-blur-xl md:rounded-xl lg:w-96`}
      >
        <header className='flex h-14 items-center gap-2 border-b px-3'>
          <Button
            variant='ghost'
            size='icon'
            aria-label='Open menu'
            className='md:hidden'
            onClick={() => setMenuOpen(true)}
          >
            <MenuIcon className='size-5' />
          </Button>
          <h1 className='min-w-0 flex-1 truncate text-sm font-semibold'>
            {heading}
          </h1>
          <Button
            variant='ghost'
            size='sm'
            disabled={markAllRead.isPending}
            onClick={() => markAllRead.mutate()}
            className='text-muted-foreground'
          >
            Mark all read
          </Button>
        </header>
        <ArticleList
          articles={articles}
          selectedId={articleId}
          loading={list.isLoading}
          error={list.isError}
          hasMore={!!list.hasNextPage}
          loadingMore={list.isFetchingNextPage}
          onLoadMore={() => list.fetchNextPage()}
          onSelect={(id) => navigate({ a: id })}
          emptyHint={
            feeds.data?.length === 0
              ? 'Add your first feed from the sidebar to get started.'
              : filter === 'unread'
                ? "You're all caught up."
                : filter === 'saved'
                  ? 'Press S while reading to save an article.'
                  : 'Nothing here yet. New articles show up within a few minutes.'
          }
        />
      </section>

      <main
        className={`${articleId ? 'flex' : 'hidden lg:flex'} min-w-0 flex-1 flex-col overflow-hidden border bg-card/70 shadow-[var(--shadow-panel)] backdrop-blur-xl md:rounded-xl`}
      >
        <ArticleView
          article={detail.data}
          loading={!!articleId && detail.isLoading}
          error={!!articleId && detail.isError}
          onBack={() => navigate({ a: null })}
          onToggleSaved={(a) => setState.mutate({ id: a.id, saved: !a.saved })}
          onToggleRead={(a) => setState.mutate({ id: a.id, read: !a.read })}
        />
      </main>
    </div>
  );
}
