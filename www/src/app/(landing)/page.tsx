import { Cta } from './_components/cta';
import { Faq } from './_components/faq';
import { Features } from './_components/features';
import { Footer } from './_components/footer';
import { Hero } from './_components/hero';
import { HowItWorks } from './_components/how-it-works';

export default function Home() {
  return (
    <main>
      <Hero />
      <Features />
      <HowItWorks />
      <Faq />
      <Cta />
      <Footer />
    </main>
  );
}
