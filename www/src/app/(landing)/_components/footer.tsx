import { Logo } from '@/components/logo';
import { siteConfig } from '@/config/site';
import Link from 'next/link';

export function Footer() {
  return (
    <footer className='border-t px-4 py-10'>
      <div className='mx-auto flex max-w-5xl flex-col items-center justify-between gap-6 sm:flex-row'>
        <div className='flex flex-col items-center sm:items-start'>
          <Logo />
          <p className='mt-2 max-w-xs text-sm text-muted-foreground'>
            Read the web. Skip the noise.
          </p>
        </div>
        <nav
          aria-label='Footer'
          className='flex gap-6 text-sm text-muted-foreground'
        >
          <a className='hover:text-foreground' href={siteConfig.links.github}>
            GitHub
          </a>
          <a className='hover:text-foreground' href={siteConfig.links.x}>
            X
          </a>
          <Link className='hover:text-foreground' href='/signin'>
            Log in
          </Link>
        </nav>
      </div>
      <p className='mx-auto mt-8 max-w-5xl text-center text-xs text-muted-foreground sm:text-left'>
        © Feedoku. Open source under the license in the repository.
      </p>
    </footer>
  );
}
