import { Button } from '@/components/ui/button';
import { AvatarStack } from './_components/avatar-stack';
import { Brands } from './_components/brands';

export default function Home() {
  return (
    <main className='mx-4 flex flex-col items-center justify-center'>
      <div className='mt-24 flex max-w-4xl flex-col items-center justify-center'>
        <div className='mb-6'>
          <AvatarStack />
          <div className='w-full text-center'>&more</div>
        </div>
        <div className='text-center'>
          <h1 className='text-4xl font-semibold text-amber-300 md:text-4xl lg:text-5xl'>
            Stay Informed. Stay Connected
          </h1>
          <p className='leading-tight text-muted-foreground'>
            Feedoku brings the power of real-time news and content aggregation
            to your fingertips. Discover, organize, and read your favorite RSS
            feeds from across the web in one simple platform.
          </p>
          <div className='mt-4 flex items-center justify-center gap-2'>
            <Button variant='amber'>Get Started</Button>
            <Button variant='outline' className='rounded shadow-none'>
              Learn more
            </Button>
          </div>
        </div>
      </div>
      <Brands />
    </main>
  );
}
