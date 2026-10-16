import { Button } from '@/components/ui/button';
import { ArrowRightIcon, SparklesIcon } from 'lucide-react';
import Link from 'next/link';
import { ProductPreview } from './product-preview';

export function Hero() {
  return (
    <section className='relative px-4 pt-36 pb-16 sm:pt-44'>
      <div className='mx-auto flex max-w-4xl animate-in flex-col items-center text-center duration-700 fade-in slide-in-from-bottom-4'>
        <span className='inline-flex items-center gap-2 rounded-full border bg-card/70 px-3.5 py-1.5 text-xs font-medium text-muted-foreground backdrop-blur'>
          <SparklesIcon className='size-3.5 text-brand' />
          The AI-native RSS reader
        </span>
        <h1 className='mt-6 text-5xl leading-[1.02] font-extrabold tracking-tighter text-balance sm:text-6xl lg:text-7xl'>
          Read the web.
          <br />
          <span className='text-brand'>Skip the noise.</span>
        </h1>
        <p className='mt-6 max-w-2xl text-lg leading-relaxed text-pretty text-muted-foreground'>
          Feedoku pulls every blog, newsletter and news source you follow into
          one calm reader, then uses AI to summarize, group and brief you, so
          you spend minutes instead of hours.
        </p>
        <div className='mt-9 flex flex-col items-center gap-3 sm:flex-row'>
          <Button asChild size='lg'>
            <Link href='/signup'>
              Get started free <ArrowRightIcon />
            </Link>
          </Button>
          <Button asChild size='lg' variant='outline'>
            <a href='#how'>See how it works</a>
          </Button>
        </div>
        <p className='mt-5 text-xs text-muted-foreground'>
          Open source · Import your OPML in one click · Works with RSS, Atom and
          JSON Feed
        </p>
      </div>
      <div className='mx-auto mt-16 max-w-5xl animate-in duration-1000 fade-in slide-in-from-bottom-8'>
        <ProductPreview />
      </div>
    </section>
  );
}
