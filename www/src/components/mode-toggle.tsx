'use client';

import { Button } from '@/components/ui/button';
import { MoonIcon, SunIcon } from 'lucide-react';
import { useTheme } from 'next-themes';

export function ModeToggle() {
  const { theme, setTheme } = useTheme();

  return (
    <Button
      variant='ghost'
      type='button'
      size='icon'
      className='size-8 rounded px-2'
      onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
    >
      <SunIcon className='h-[1.2rem] w-[1.2rem] text-foreground dark:hidden' />
      <MoonIcon className='hidden h-[1.2rem] w-[1.2rem] text-foreground dark:block' />
    </Button>
  );
}
