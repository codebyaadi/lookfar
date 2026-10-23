<script lang="ts">
  import ArrowRight from '@lucide/svelte/icons/arrow-right';
  import Radar from '@lucide/svelte/icons/radar';
  import Button from '#lib/components/ui/button.svelte';
  import ProductPreview from './product-preview.svelte';
</script>

<section class="relative px-4 pt-36 pb-16 sm:pt-44">
  <div
    class="mx-auto flex max-w-4xl animate-in flex-col items-center text-center duration-700 fade-in slide-in-from-bottom-4"
  >
    <span
      class="inline-flex items-center gap-2 rounded-full border bg-card/70 px-3.5 py-1.5 text-xs font-medium text-muted-foreground backdrop-blur"
    >
      <Radar class="size-3.5 text-brand" />
      Self-hosted watcher agents
    </span>
    <h1
      class="mt-6 text-5xl leading-[1.02] font-extrabold tracking-tighter text-balance sm:text-6xl lg:text-7xl"
    >
      Agents that look far,
      <br />
      <span class="text-brand">so you don't have to.</span>
    </h1>
    <p
      class="mt-6 max-w-2xl text-lg leading-relaxed text-pretty text-muted-foreground"
    >
      Describe what to watch in plain language. Lookfar checks pages, feeds and
      repos on a schedule, tells you only when something real changes, and asks
      before it does anything on your behalf.
    </p>
    <div class="mt-9 flex flex-col items-center gap-3 sm:flex-row">
      <Button href="/signup" size="lg">
        Get started free <ArrowRight class="size-4" />
      </Button>
      <Button href="#how" variant="outline" size="lg"
        >See how it works</Button
      >
    </div>
    <p class="mt-5 text-xs text-muted-foreground">
      Open source · Bring your own LLM keys · One <code class="font-mono"
        >docker compose up</code
      >
    </p>
  </div>

  <div
    class="mx-auto mt-16 max-w-5xl animate-in duration-1000 fade-in slide-in-from-bottom-8"
  >
    <ProductPreview />
  </div>
</section>
