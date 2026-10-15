'use client';

import { useEffect, useState } from 'react';
import { motion } from 'motion/react';
import { useTheme } from 'next-themes';
import { BrandsLogoType, CustomSVGProps } from '../_types';
import { brandsLogo } from '../_brands';

const shuffleArray = (array: BrandsLogoType<CustomSVGProps>[]) => {
  return array
    .map((item) => ({ ...item, sort: Math.random() }))
    .toSorted((a, b) => a.sort - b.sort)
    .map((item) => item);
};

export const Brands = () => {
  const [shuffledBrands, setShuffledBrands] = useState(brandsLogo);
  const { theme } = useTheme();
  const fillColor = theme === 'dark' ? 'white' : 'black';

  // Shuffle the logos on every animation cycle (using a timer for this example)
  useEffect(() => {
    const interval = setInterval(() => {
      setShuffledBrands(shuffleArray(brandsLogo));
    }, 3000); // Adjust time interval as needed

    return () => clearInterval(interval); // Clean up on unmount
  }, []);

  return (
    <div className='mt-20 hidden w-full flex-col items-center justify-between gap-12 px-4 py-4 md:flex lg:flex lg:px-10'>
      <div className='text-center'>
        Stay up to date with the world&apos;s best content platforms
      </div>
      <div className='grid grid-cols-3 items-center gap-24'>
        {shuffledBrands.map(({ id, fc: Logo }) => (
          <motion.div
            key={id}
            className='mx-auto'
            layout
            initial={{ opacity: 0, scale: 0.5, filter: 'blur(10px)' }}
            exit={{ opacity: 0, scale: 0.5, filter: 'blur(10px)' }}
            animate={{
              opacity: 1,
              scale: 1,
              filter: 'blur(0px)',
            }}
            transition={{
              duration: 1.5,
              ease: 'easeInOut',
            }}
          >
            <Logo fillcolor={fillColor} />
          </motion.div>
        ))}
      </div>
    </div>
  );
};
