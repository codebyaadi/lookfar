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
  import type { PageProps } from './$types';

  let { data }: PageProps = $props();
</script>

<svelte:head>
  <title>Lookfar · Agents that look far, so you don't have to</title>
  <meta
    name="description"
    content="Self-hosted watcher agents. Describe what to watch, and Lookfar checks pages, feeds and repos on a schedule, reports only what matters, and asks before acting."
  />
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
