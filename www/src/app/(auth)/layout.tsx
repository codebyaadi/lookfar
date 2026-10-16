import { Logo } from '@/components/logo';
import { ModeToggle } from '@/components/mode-toggle';
import { CheckIcon } from 'lucide-react';
import Link from 'next/link';

const points = [
  'Paste any site and we find the feed',
  'A calm, keyboard-first reader',
  'AI summaries and briefings on the way',
];

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className='ambient relative flex min-h-dvh w-full'>
      <div className='relative hidden w-1/2 flex-col justify-between overflow-hidden bg-primary p-12 text-primary-foreground lg:flex'>
        <div className='pointer-events-none absolute -top-32 -right-32 size-[28rem] rounded-full bg-primary-foreground/10 blur-3xl' />
        <div className='pointer-events-none absolute -bottom-40 -left-24 size-[26rem] rounded-full bg-primary-foreground/10 blur-3xl' />
        <Link href='/' aria-label='Feedoku home' className='relative w-fit'>
          <Logo tone='inverted' />
        </Link>
        <div className='relative'>
          <h2 className='max-w-md text-5xl leading-[1.05] font-extrabold tracking-tighter text-balance'>
            Read the web. Skip the noise.
          </h2>
          <ul className='mt-8 space-y-3'>
            {points.map((p) => (
              <li key={p} className='flex items-center gap-3 opacity-90'>
                <span className='grid size-6 place-items-center rounded-full bg-primary-foreground/15'>
                  <CheckIcon className='size-3.5' />
                </span>
                {p}
              </li>
            ))}
          </ul>
        </div>
        <p className='relative text-sm opacity-60'>© Feedoku</p>
      </div>

      <div className='relative flex w-full flex-col lg:w-1/2'>
        <div className='flex items-center justify-between p-4 sm:p-6'>
          <Link href='/' aria-label='Feedoku home' className='lg:invisible'>
            <Logo />
          </Link>
          <ModeToggle />
        </div>
        <div className='flex flex-1 items-center justify-center px-4 pb-12'>
          {children}
        </div>
      </div>
    </div>
  );
}
