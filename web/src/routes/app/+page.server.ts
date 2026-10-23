import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

type InboxItem = {
  id: number;
  kind: 'report' | 'alert' | 'question' | 'approval';
  title: string;
  bodyMd: string;
  status: string;
  createdAt: string;
};

export const load: PageServerLoad = async ({ fetch }) => {
  let res: Response;
  try {
    res = await fetch('/api/v1/inbox');
  } catch {
    error(503, 'The Lookfar API is unreachable');
  }
  // The proxy turns a dead upstream into a network error or a 5xx.
  if (res.status >= 500) error(503, 'The Lookfar API is unreachable');
  if (!res.ok) error(res.status, 'Could not load your inbox');

  const { items } = (await res.json()) as { items: InboxItem[] };
  return { items };
};
