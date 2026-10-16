import Navbar from '@/components/layouts/navigation/navbar';

export default function LandingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className='ambient relative min-h-screen'>
      <Navbar />
      {children}
    </div>
  );
}
