import type { Metadata } from 'next';
import './globals.css';
import { geistMono, sora } from './fonts';
import { Toaster } from '@/components/ui/sonner';
import { siteConfig } from '@/config/site';
import { ThemeProvider } from '@/providers/theme-providers';

export const metadata: Metadata = {
  title: siteConfig.name,
  description: siteConfig.description,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang='en'
      className={`${sora.variable} ${geistMono.variable}`}
      suppressHydrationWarning
    >
      <body className='font-sans antialiased'>
        <ThemeProvider
          attribute='class'
          defaultTheme='system'
          enableSystem
          disableTransitionOnChange
        >
          {children}
        </ThemeProvider>
        <Toaster />
      </body>
    </html>
  );
}
