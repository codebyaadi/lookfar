<script lang="ts">
  import Agents from '#lib/components/landing/agents.svelte';
  import Cta from '#lib/components/landing/cta.svelte';
  import Faq from '#lib/components/landing/faq.svelte';
  import Features from '#lib/components/landing/features.svelte';
  import Footer from '#lib/components/landing/footer.svelte';
  import Hero from '#lib/components/landing/hero.svelte';
  import HowItWorks from '#lib/components/landing/how-it-works.svelte';
  import Navbar from '#lib/components/landing/navbar.svelte';
  import Pipeline from '#lib/components/landing/pipeline.svelte';
  import Problem from '#lib/components/landing/problem.svelte';
  import SelfHost from '#lib/components/landing/self-host.svelte';
  import Seo from '#lib/components/seo.svelte';
  import { site } from '#lib/site.ts';
  import type { PageProps } from './$types';

  let { data }: PageProps = $props();

  // Escape "<" so the JSON can never close the script tag early.
  const jsonLd = JSON.stringify({
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'WebSite',
        '@id': `${site.url}/#website`,
        url: site.url,
        name: site.name,
        description: site.description,
      },
      {
        '@type': 'SoftwareApplication',
        '@id': `${site.url}/#software`,
        name: site.name,
        url: site.url,
        description: site.description,
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'Linux, macOS, Windows (Docker)',
        isAccessibleForFree: true,
        license: 'https://opensource.org/licenses/MIT',
        codeRepository: site.github,
        keywords: site.keywords.join(', '),
        image: `${site.url}${site.ogImage}`,
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
      },
    ],
  }).replaceAll('<', '\\u003c');

  const ldTag = ['<script type="application/ld+json">', jsonLd, '</', 'script>'].join('');
</script>

<Seo title={site.title} />

<svelte:head>
  {@html ldTag}
</svelte:head>

<div class="ambient min-h-screen">
  <Navbar signedIn={data.signedIn} />
  <main>
    <Hero />
    <Problem />
    <HowItWorks />
    <Pipeline />
    <Agents />
    <Features />
    <SelfHost />
    <Faq />
    <Cta />
  </main>
  <Footer />
</div>
