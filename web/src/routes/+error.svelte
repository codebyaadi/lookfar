<script lang="ts">
  import { page } from '$app/state';
  import { invalidateAll } from '$app/navigation';
  import SearchX from '@lucide/svelte/icons/search-x';
  import ServerCrash from '@lucide/svelte/icons/server-crash';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import Logo from '#lib/components/logo.svelte';
  import Button from '#lib/components/ui/button.svelte';

  const status = $derived(page.status);

  const copy = $derived.by(() => {
    if (status === 404) {
      return {
        icon: SearchX,
        title: 'Nothing to look at here.',
        body: 'The page you asked for does not exist, or it moved.',
      };
    }
    if (status === 503) {
      return {
        icon: ServerCrash,
        title: 'Lookfar cannot reach its API.',
        body: 'The server is up but the backend is not answering. Check that the API and database are running, then try again.',
      };
    }
    return {
      icon: TriangleAlert,
      title: 'Something went wrong.',
      body: page.error?.message || 'An unexpected error happened on our side.',
    };
  });
  const Icon = $derived(copy.icon);
</script>

<svelte:head>
  <title>{status} · Lookfar</title>
</svelte:head>

<main class="ambient grid min-h-screen place-items-center px-4">
  <div
    class="w-full max-w-md space-y-6 rounded-3xl border bg-card/70 p-8 text-center shadow-(--shadow-panel) backdrop-blur-xl sm:p-10"
  >
    <a href="/" class="inline-flex" aria-label="Lookfar home"><Logo /></a>

    <div
      class="mx-auto grid size-11 place-items-center rounded-2xl bg-accent text-accent-foreground"
    >
      <Icon class="size-5" />
    </div>

    <div class="space-y-3">
      <p class="text-sm font-semibold text-brand">Error {status}</p>
      <h1
        class="text-3xl leading-[1.05] font-extrabold tracking-tighter text-balance"
      >
        {copy.title}
      </h1>
      <p class="leading-relaxed text-pretty text-muted-foreground">
        {copy.body}
      </p>
    </div>

    <div class="flex flex-col justify-center gap-3 sm:flex-row">
      {#if status !== 404}
        <Button onclick={() => invalidateAll()}>Try again</Button>
      {/if}
      <Button href="/" variant={status === 404 ? 'default' : 'outline'}
        >Back home</Button
      >
    </div>
  </div>
</main>
