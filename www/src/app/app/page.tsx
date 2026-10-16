import { ApiError, apiFetch, type User } from '@/lib/api';
import { redirect } from 'next/navigation';
import { Providers } from './providers';
import { Reader } from './reader';

export const metadata = { title: 'Feedoku' };

export default async function AppHome() {
  let user: User;
  let aiEnabled = false;
  try {
    const me = await apiFetch<{ user: User; features?: { ai?: boolean } }>(
      '/v1/me'
    );
    user = me.user;
    aiEnabled = !!me.features?.ai;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401)
      redirect('/signin?next=/app');
    throw err;
  }
  return (
    <Providers>
      <Reader user={user} aiEnabled={aiEnabled} />
    </Providers>
  );
}
