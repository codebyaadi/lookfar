// Types and a small fetch helper for the Go API (proxied at /api/v1).

export type SourceKind = 'page' | 'rss' | 'github';

export type SourceConfig = {
  selector?: string;
  ignoreSelectors?: string[];
  ignorePatterns?: string[];
  keepTimestamps?: boolean;
  minChangeChars?: number;
  maxItems?: number;
  includePrereleases?: boolean;
};

export type Source = {
  id: number;
  kind: SourceKind;
  url: string;
  title: string;
  config: SourceConfig;
  intervalSecs: number;
  paused: boolean;
  nextCheckAt: string;
  lastCheckedAt: string | null;
  lastChangedAt: string | null;
  errorCount: number;
  lastError: string | null;
  createdAt: string;
  snapshotCount?: number;
};

export type CheckResult = {
  ok: boolean;
  error?: string;
  baseline?: boolean;
  changed?: boolean;
  notModified?: boolean;
  belowThreshold?: boolean;
  snapshotId?: number;
};

export type SnapshotSummary = {
  id: number;
  fetchedAt: string;
  addedLines: number;
  removedLines: number;
  sizeBytes: number;
  baseline: boolean;
};

export type Hunk = {
  op: 'eq' | 'add' | 'del' | 'skip';
  lines?: string[];
  skipped?: number;
};

export type Diff = { added: number; removed: number; hunks: Hunk[] };

export type SnapshotDetail = SnapshotSummary & {
  content: string;
  diff: Diff | null;
};

export type ApiResult<T> =
  | { ok: true; data: T }
  | { ok: false; status: number; message: string };

/** Calls the API through the app's own origin. Never throws on HTTP errors. */
export async function api<T>(
  fetchFn: typeof fetch,
  path: string,
  init?: { method?: string; body?: unknown }
): Promise<ApiResult<T>> {
  let res: Response;
  try {
    res = await fetchFn(`/api/v1${path}`, {
      method: init?.method ?? 'GET',
      headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
      body: init?.body ? JSON.stringify(init.body) : undefined,
    });
  } catch {
    return {
      ok: false,
      status: 503,
      message: 'The Lookfar API is unreachable',
    };
  }
  if (res.status >= 500) {
    return {
      ok: false,
      status: 503,
      message: 'The Lookfar API is unreachable',
    };
  }
  if (res.status === 204) return { ok: true, data: undefined as T };
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return {
      ok: false,
      status: res.status,
      message: body?.error?.message ?? `Request failed (${res.status})`,
    };
  }
  return { ok: true, data: body as T };
}
