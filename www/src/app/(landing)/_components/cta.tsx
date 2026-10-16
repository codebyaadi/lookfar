import { Button } from '@/components/ui/button';
import { ArrowRightIcon } from 'lucide-react';
import Link from 'next/link';

export function Cta() {
  return (
    <section className='px-4 pb-24'>
      <div className='mx-auto max-w-5xl rounded-[2rem] border bg-card/70 px-6 py-16 text-center shadow-[var(--shadow-panel)] backdrop-blur-xl sm:py-20'>
        <h2 className='mx-auto max-w-2xl text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl'>
          Take back your reading time.
        </h2>
        <p className='mx-auto mt-4 max-w-lg text-muted-foreground'>
          Create an account, add a few feeds, and see how quiet the internet can
          be.
        </p>
        <Button asChild size='lg' className='mt-8'>
          <Link href='/signup'>
            Get started free <ArrowRightIcon />
          </Link>
        </Button>
      </div>
    </section>
  );
}
