// Browser-side client for the Go API. Requests go to /api/v1/* on the same
// origin (Next rewrites them), so the better-auth session cookie is sent.

export type Feed = {
  id: number;
  url: string;
  siteUrl: string | null;
  title: string;
  favicon: string | null;
  folder: string | null;
  unreadCount: number;
  error: string | null;
};

export type ArticleSummary = {
  id: number;
  feedId: number;
  feedTitle: string;
  feedFavicon: string | null;
  url: string;
  title: string;
  author: string | null;
  publishedAt: string;
  excerpt: string;
  imageUrl: string | null;
  read: boolean;
  saved: boolean;
  tldr: string;
  topics: string[];
};

export type ArticleAI = {
  status: 'pending' | 'done' | 'failed';
  tldr: string;
  keyPoints: string[];
  topics: string[];
  sentiment: string;
  readingMinutes: number;
};

export type Topic = { name: string; count: number };

export type ArticleDetail = {
  id: number;
  feedId: number;
  feedTitle: string;
  url: string;
  title: string;
  author: string | null;
  publishedAt: string;
  contentHtml: string;
  summary: string;
  imageUrl: string | null;
  read: boolean;
  saved: boolean;
  rating: number;
  ai: ArticleAI;
};

export type ArticlePage = {
  articles: ArticleSummary[];
  nextCursor: string | null;
};

export type ArticleFilter = 'all' | 'unread' | 'saved';

export class ApiRequestError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    ...init,
    headers: { 'content-type': 'application/json', ...init.headers },
  });
  if (res.status === 401) {
    window.location.href = '/signin?next=/app';
    throw new ApiRequestError(401, 'Signed out');
  }
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new ApiRequestError(
      res.status,
      body?.error?.message ?? 'Something went wrong'
    );
  }
  return res.status === 204 ? (undefined as T) : res.json();
}

export const api = {
  feeds: () => request<{ feeds: Feed[] }>('/feeds').then((r) => r.feeds),
  topics: () => request<{ topics: Topic[] }>('/topics').then((r) => r.topics),
  addFeed: (url: string) =>
    request<{ feed: Feed }>('/feeds', {
      method: 'POST',
      body: JSON.stringify({ url }),
    }).then((r) => r.feed),
  removeFeed: (id: number) =>
    request<void>(`/feeds/${id}`, { method: 'DELETE' }),
  importOpml: (xml: string) =>
    request<{ imported: number; failed: string[] }>('/feeds/opml', {
      method: 'POST',
      body: xml,
      headers: { 'content-type': 'text/xml' },
    }),
  articles: (p: {
    feed?: number;
    filter: ArticleFilter;
    topic?: string;
    cursor?: string | null;
  }) => {
    const q = new URLSearchParams({ filter: p.filter });
    if (p.feed) q.set('feed', String(p.feed));
    if (p.topic) q.set('topic', p.topic);
    if (p.cursor) q.set('cursor', p.cursor);
    return request<ArticlePage>(`/articles?${q}`);
  },
  article: (id: number) =>
    request<{ article: ArticleDetail }>(`/articles/${id}`).then(
      (r) => r.article
    ),
  setState: (id: number, state: { read?: boolean; saved?: boolean }) =>
    request<{ read: boolean; saved: boolean }>(`/articles/${id}/state`, {
      method: 'POST',
      body: JSON.stringify(state),
    }),
  markAllRead: (feed?: number) =>
    request<void>(`/articles/read-all${feed ? `?feed=${feed}` : ''}`, {
      method: 'POST',
    }),
};

export function timeAgo(iso: string): string {
  const s = (Date.now() - new Date(iso).getTime()) / 1000;
  if (s < 3600) return `${Math.max(1, Math.floor(s / 60))}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  if (s < 86400 * 30) return `${Math.floor(s / 86400)}d`;
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: s > 86400 * 365 ? 'numeric' : undefined,
  });
}
