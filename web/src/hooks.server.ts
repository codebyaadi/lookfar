import { building } from '$app/env';
import { auth } from '#lib/server/auth.ts';
import { svelteKitHandler } from 'better-auth/svelte-kit';
import type { Handle } from '@sveltejs/kit/hooks';

export const handle: Handle = async ({ event, resolve }) => {
  // A failing session lookup (e.g. the database is down) must not take down
  // public pages: treat the visitor as signed out and carry on.
  try {
    const session = await auth.api.getSession({
      headers: event.request.headers,
    });
    if (session) {
      event.locals.session = session.session;
      event.locals.user = session.user;
    }
  } catch (err) {
    console.warn('session lookup failed:', err);
  }
  if (
    event.url.pathname.startsWith('/app') ||
    event.url.pathname.startsWith('/api')
  ) {
    event.setHeaders({ 'X-Robots-Tag': 'noindex, nofollow' });
  }
  return svelteKitHandler({ event, resolve, auth, building });
};
