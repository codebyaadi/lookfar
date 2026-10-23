import { error, fail, redirect } from '@sveltejs/kit';
import { api, type CheckResult, type Source } from '#lib/api.ts';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await api<{ sources: Source[] }>(fetch, '/sources');
  if (!res.ok) error(res.status, res.message);
  return { sources: res.data.sources };
};

export const actions: Actions = {
  create: async ({ request, fetch }) => {
    const form = await request.formData();
    const kind = String(form.get('kind') ?? 'page');
    const values = {
      kind,
      url: String(form.get('url') ?? '').trim(),
      title: String(form.get('title') ?? '').trim(),
      intervalSecs: Number(form.get('intervalSecs') ?? 3600),
      selector: String(form.get('selector') ?? '').trim(),
    };

    const config: Record<string, unknown> = {};
    if (kind === 'page' && values.selector) config.selector = values.selector;

    const res = await api<{ source: Source; check: CheckResult }>(
      fetch,
      '/sources',
      {
        method: 'POST',
        body: {
          kind,
          url: values.url,
          title: values.title,
          intervalSecs: values.intervalSecs,
          config,
        },
      }
    );
    if (!res.ok) return fail(res.status, { message: res.message, values });
    redirect(303, `/app/sources/${res.data.source.id}?created=1`);
  },
};
