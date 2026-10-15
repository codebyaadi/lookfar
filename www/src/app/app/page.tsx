import { ApiError, apiFetch, type User } from '@/lib/api';
import { redirect } from 'next/navigation';
import { SignOutButton } from './sign-out-button';

export const metadata = { title: 'Feedoku' };

// Phase 0 placeholder: proves the browser → Next → Go → Postgres session path
// works end to end. Replaced by the reader shell in Phase 1.
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
    <main className='mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 px-4'>
      <h1 className='text-3xl font-semibold'>Hi, {user.name}</h1>
      <p className='text-muted-foreground'>
        You&apos;re signed in as {user.email}. The Go API verified your session,
        so the stack is wired up. Your feeds will show up here.
      </p>
      <div>
        <SignOutButton />
      </div>
    </main>
  );
}
