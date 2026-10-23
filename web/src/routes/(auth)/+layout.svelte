<script lang="ts">
  import Logo from '#lib/components/logo.svelte';
  import type { LayoutProps } from './$types';

  let { children }: LayoutProps = $props();
</script>

<main class="ambient grid min-h-screen place-items-center px-6">
  <div
    class="w-full max-w-sm space-y-6 rounded-3xl border border-border bg-card/70 p-8 shadow-(--shadow-panel) backdrop-blur"
  >
    <a href="/"><Logo /></a>
    {@render children()}
  </div>
</main>
