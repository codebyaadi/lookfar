<script lang="ts">
  import Seo from '#lib/components/seo.svelte';
  import type { PageProps } from './$types';

  let { data }: PageProps = $props();
</script>

<Seo title="Inbox" noindex />

<h1 class="text-3xl font-extrabold tracking-tighter">Inbox</h1>

{#if data.items.length === 0}
  <p class="mt-6 text-muted-foreground">
    Nothing here yet. Reports, alerts and approvals from your agents will show
    up in this inbox.
  </p>
{:else}
  <ul class="mt-6 space-y-3">
    {#each data.items as item (item.id)}
      <li class="rounded-3xl border border-border bg-card/70 p-5">
        <span class="text-xs font-semibold text-brand uppercase"
          >{item.kind}</span
        >
        <h2 class="font-bold tracking-tight">{item.title}</h2>
      </li>
    {/each}
  </ul>
{/if}
