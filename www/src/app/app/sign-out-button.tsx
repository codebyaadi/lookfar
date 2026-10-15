'use client';

import { Button } from '@/components/ui/button';
import { authClient } from '@/lib/auth-client';
import { useRouter } from 'next/navigation';

export function SignOutButton() {
  const router = useRouter();
  return (
    <Button
      variant='outline'
      onClick={async () => {
        await authClient.signOut();
        router.push('/signin');
      }}
    >
      Sign out
    </Button>
  );
}
