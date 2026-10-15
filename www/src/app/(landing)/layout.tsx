import Navbar from '@/components/layouts/navigation/navbar';
import React from 'react';

export default function LandingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className='relative pt-16'>
      <Navbar />
      <div className='absolute top-0 h-1/3 w-full' />
      {children}
    </div>
  );
}
