import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = ({ locals, url }) => {
  if (!locals.user) {
    redirect(303, `/signin?next=${encodeURIComponent(url.pathname)}`);
  }
  return { user: { name: locals.user.name, email: locals.user.email } };
};
