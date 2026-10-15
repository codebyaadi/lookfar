'use client';

import React from 'react';
import Link from 'next/link';
import { ModeToggle } from '@/components/mode-toggle';
import { Button } from '@/components/ui/button';
import { siteConfig } from '@/config/site';
import { authClient } from '@/lib/auth-client';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';

const handleSignOut = async () => {
  await authClient.signOut();
};

const Navbar = () => {
  const { data: session } = authClient.useSession();

  return (
    <nav className='fixed top-0 z-1 w-full bg-transparent px-0 lg:px-6'>
      <div className='mx-4 flex items-center justify-between py-2'>
        <Link href='/' className='font-semibold text-amber-300'>
          {siteConfig.name}
        </Link>
        <div className='flex items-center justify-center gap-2 font-normal'>
          <ModeToggle />
          {session ? (
            <DropdownMenu>
              <DropdownMenuTrigger className='focus:outline-none'>
                <Avatar>
                  <AvatarImage src={session.user.image || ''} />
                  <AvatarFallback>{session.user.name[0]}</AvatarFallback>
                </Avatar>
              </DropdownMenuTrigger>
              <DropdownMenuContent className='w-42 rounded'>
                <DropdownMenuLabel>My Account</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuGroup>
                  <DropdownMenuItem>
                    Profile
                    <DropdownMenuShortcut>⇧⌘P</DropdownMenuShortcut>
                  </DropdownMenuItem>
                  <DropdownMenuItem>
                    Settings
                    <DropdownMenuShortcut>⌘S</DropdownMenuShortcut>
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onSelect={handleSignOut}>
                    Log out
                    <DropdownMenuShortcut>⇧⌘Q</DropdownMenuShortcut>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <>
              <Button
                variant='secondary'
                size='sm'
                className='rounded-sm shadow-none'
              >
                <Link href='/signin'>Log In</Link>
              </Button>
              <Button variant='amber' size='sm'>
                <Link href='/signup'>Sign Up</Link>
              </Button>
            </>
          )}
        </div>
      </div>
    </nav>
  );
};

export default Navbar;
