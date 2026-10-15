import { ModeToggle } from '@/components/mode-toggle';
import { AvatarStack } from './_components/avatar-scatter';
import { siteConfig } from '@/config/site';
import Link from 'next/link';

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className='relative flex h-screen w-full items-center'>
      <nav className='fixed top-0 z-1 w-full bg-transparent px-0 lg:px-6'>
        <div className='mx-4 flex items-center justify-between py-2'>
          <Link href='/' className='font-semibold text-amber-300 lg:text-white'>
            {siteConfig.name}
          </Link>
          <div className='flex items-center justify-center gap-2 font-normal'>
            <ModeToggle />
          </div>
        </div>
      </nav>
      <div className='hidden h-full w-1/2 flex-col items-center justify-center bg-amber-300 lg:flex'>
        <AvatarStack />
        <p className='text-xl font-semibold text-white'>&more</p>
      </div>
      <div className='flex w-full items-center justify-center lg:w-1/2'>
        {children}
      </div>
    </div>
  );
}
