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
  const res = await fetch('/api/v1/inbox');
  if (!res.ok) error(res.status, 'Could not load your inbox');
  const { items } = (await res.json()) as { items: InboxItem[] };
  return { items };
};
