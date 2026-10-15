'use client';

import { Button } from '@/components/ui/button';

export default function AppError({
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  return (
    <main className='mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 px-4'>
      <h1 className='text-2xl font-semibold'>
        Can&apos;t reach the Feedoku API
      </h1>
      <p className='text-muted-foreground'>
        Check that the Go API is running (<code>make api</code>) and that
        API_URL is set.
      </p>
      <div>
        <Button onClick={reset}>Try again</Button>
      </div>
    </main>
  );
}
