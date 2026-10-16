'use client';

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Logo } from '@/components/logo';
import { ModeToggle } from '@/components/mode-toggle';
import { authClient } from '@/lib/auth-client';
import Link from 'next/link';

const links = [
  { href: '#features', label: 'Features' },
  { href: '#how', label: 'How it works' },
  { href: '#faq', label: 'FAQ' },
];

export default function Navbar() {
  const { data: session } = authClient.useSession();

  return (
    <header className='fixed inset-x-0 top-0 z-40 px-3 pt-3 sm:px-6'>
      <nav
        aria-label='Main'
        className='mx-auto flex h-14 max-w-5xl items-center justify-between rounded-full border bg-card/70 pr-2 pl-4 shadow-[var(--shadow-panel)] backdrop-blur-xl'
      >
        <Link href='/' aria-label='Feedoku home'>
          <Logo />
        </Link>

        <ul className='hidden items-center gap-1 md:flex'>
          {links.map((l) => (
            <li key={l.href}>
              <Button
                asChild
                variant='ghost'
                size='sm'
                className='text-muted-foreground'
              >
                <a href={l.href}>{l.label}</a>
              </Button>
            </li>
          ))}
        </ul>

        <div className='flex items-center gap-1.5'>
          <ModeToggle />
          {session ? (
            <>
              <Button asChild size='sm'>
                <Link href='/app'>Open reader</Link>
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger
                  aria-label='Account menu'
                  className='rounded-full focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
                >
                  <Avatar className='size-8'>
                    <AvatarImage src={session.user.image || ''} />
                    <AvatarFallback>{session.user.name[0]}</AvatarFallback>
                  </Avatar>
                </DropdownMenuTrigger>
                <DropdownMenuContent
                  align='end'
                  className='w-48 rounded-2xl p-1.5'
                >
                  <DropdownMenuLabel className='truncate'>
                    {session.user.email}
                  </DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem asChild className='rounded-xl'>
                    <Link href='/app'>Open reader</Link>
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className='rounded-xl'
                    onSelect={() => authClient.signOut()}
                  >
                    Log out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          ) : (
            <>
              <Button
                asChild
                variant='ghost'
                size='sm'
                className='hidden sm:inline-flex'
              >
                <Link href='/signin'>Log in</Link>
              </Button>
              <Button asChild size='sm'>
                <Link href='/signup'>Get started</Link>
              </Button>
            </>
          )}
        </div>
      </nav>
    </header>
  );
}
