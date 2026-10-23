import { error, fail, redirect } from '@sveltejs/kit';
import {
  api,
  type CheckResult,
  type SnapshotDetail,
  type SnapshotSummary,
  type Source,
  type SourceConfig,
} from '#lib/api.ts';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params, url }) => {
  const src = await api<{ source: Source }>(fetch, `/sources/${params.id}`);
  if (!src.ok)
    error(src.status, src.status === 404 ? 'Source not found' : src.message);

  const list = await api<{ snapshots: SnapshotSummary[] }>(
    fetch,
    `/sources/${params.id}/snapshots`
  );
  if (!list.ok) error(list.status, list.message);
  const snapshots = list.data.snapshots;

  // Show the snapshot asked for, else the latest real change, else the baseline.
  const wanted = Number(url.searchParams.get('snapshot'));
  const pick =
    snapshots.find((s) => s.id === wanted) ??
    snapshots.find((s) => !s.baseline) ??
    snapshots[0];

  let snapshot: SnapshotDetail | null = null;
  if (pick) {
    const detail = await api<{ snapshot: SnapshotDetail }>(
      fetch,
      `/sources/${params.id}/snapshots/${pick.id}`
    );
    if (detail.ok) snapshot = detail.data.snapshot;
  }

  return {
    source: src.data.source,
    snapshots,
    snapshot,
    justCreated: url.searchParams.get('created') === '1',
  };
};

const lines = (v: FormDataEntryValue | null) =>
  String(v ?? '')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);

export const actions: Actions = {
  check: async ({ fetch, params }) => {
    const res = await api<{ source: Source; check: CheckResult }>(
      fetch,
      `/sources/${params.id}/check`,
      { method: 'POST' }
    );
    if (!res.ok) return fail(res.status, { checkMessage: res.message });
    return { check: res.data.check };
  },

  pause: async ({ fetch, params, request }) => {
    const form = await request.formData();
    const paused = form.get('paused') === 'true';
    const res = await api(fetch, `/sources/${params.id}`, {
      method: 'PATCH',
      body: { paused },
    });
    if (!res.ok) return fail(res.status, { message: res.message });
    return { saved: true };
  },

  save: async ({ fetch, params, request }) => {
    const form = await request.formData();
    const current = await api<{ source: Source }>(
      fetch,
      `/sources/${params.id}`
    );
    if (!current.ok) return fail(current.status, { message: current.message });

    const config: SourceConfig = { ...current.data.source.config };
    const kind = current.data.source.kind;
    if (kind === 'page') {
      config.selector = String(form.get('selector') ?? '').trim() || undefined;
      config.ignoreSelectors = lines(form.get('ignoreSelectors'));
      config.ignorePatterns = lines(form.get('ignorePatterns'));
      config.keepTimestamps = form.get('keepTimestamps') === 'on';
    }
    if (kind === 'github') {
      config.includePrereleases = form.get('includePrereleases') === 'on';
    }
    config.minChangeChars = Number(form.get('minChangeChars') || 0);

    const res = await api(fetch, `/sources/${params.id}`, {
      method: 'PATCH',
      body: {
        title: String(form.get('title') ?? '').trim(),
        intervalSecs: Number(form.get('intervalSecs')),
        config,
      },
    });
    if (!res.ok) return fail(res.status, { message: res.message });
    return { saved: true };
  },

  remove: async ({ fetch, params }) => {
    const res = await api(fetch, `/sources/${params.id}`, { method: 'DELETE' });
    if (!res.ok) return fail(res.status, { message: res.message });
    redirect(303, '/app/sources');
  },
};
