import { ApiError, apiFetch, type User } from '@/lib/api';
import { redirect } from 'next/navigation';
import { Providers } from './providers';
import { Reader } from './reader';

export const metadata = { title: 'Feedoku' };

export default async function AppHome() {
  let user: User;
  try {
    ({ user } = await apiFetch<{ user: User }>('/v1/me'));
  } catch (err) {
    if (err instanceof ApiError && err.status === 401)
      redirect('/signin?next=/app');
    throw err;
  }
  return (
    <Providers>
      <Reader user={user} />
    </Providers>
  );
}
