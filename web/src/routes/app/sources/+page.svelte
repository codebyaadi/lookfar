<script lang="ts">
  import { enhance } from '$app/forms';
  import Plus from '@lucide/svelte/icons/plus';
  import Eye from '@lucide/svelte/icons/eye';
  import Seo from '#lib/components/seo.svelte';
  import Badge from '#lib/components/ui/badge.svelte';
  import Button from '#lib/components/ui/button.svelte';
  import {
    hostOf,
    intervalLabel,
    intervalOptions,
    kindLabels,
    timeAgo,
  } from '#lib/format.ts';
  import type { Source, SourceKind } from '#lib/api.ts';
  import type { PageProps } from './$types';

  let { data, form }: PageProps = $props();

  let kind = $state<SourceKind>('page');
  let busy = $state(false);

  const placeholders: Record<SourceKind, string> = {
    page: 'https://example.com/pricing',
    rss: 'https://example.com/feed.xml',
    github: 'https://github.com/owner/repo',
  };

  function status(s: Source) {
    if (s.paused) return { label: 'Paused', variant: 'outline' as const };
    if (s.errorCount > 0) return { label: 'Failing', variant: 'destructive' as const };
    if (!s.lastCheckedAt) return { label: 'Waiting', variant: 'secondary' as const };
    return { label: 'Watching', variant: 'accent' as const };
  }

  const field =
    'h-10 w-full rounded-full border border-input bg-transparent px-4 text-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none';
</script>

<Seo title="Sources" noindex />

<div class="space-y-10">
  <header class="space-y-2">
    <h1 class="text-3xl font-extrabold tracking-tighter">Sources</h1>
    <p class="max-w-2xl text-muted-foreground">
      Pages, feeds and repos Lookfar checks on a schedule. Each change is stored
      as a snapshot with a diff, so you can see exactly what moved.
    </p>
  </header>

  <section
    aria-labelledby="add-heading"
    class="rounded-3xl border bg-card/70 p-6 backdrop-blur"
  >
    <h2 id="add-heading" class="flex items-center gap-2 text-lg font-bold tracking-tight">
      <Plus class="size-5 text-brand" /> Watch something new
    </h2>
    <form
      method="POST"
      action="?/create"
      class="mt-5 grid gap-4 sm:grid-cols-6"
      use:enhance={() => {
        busy = true;
        return async ({ update }) => {
          await update({ reset: false });
          busy = false;
        };
      }}
    >
      <label class="space-y-1.5 sm:col-span-2">
        <span class="text-sm font-medium">Type</span>
        <select name="kind" bind:value={kind} class={field}>
          {#each Object.entries(kindLabels) as [value, label] (value)}
            <option {value}>{label}</option>
          {/each}
        </select>
      </label>
      <label class="space-y-1.5 sm:col-span-4">
        <span class="text-sm font-medium">URL</span>
        <input
          name="url"
          type="text"
          required
          inputmode="url"
          autocomplete="off"
          placeholder={placeholders[kind]}
          value={form?.values?.url ?? ''}
          class={field}
        />
      </label>
      <label class="space-y-1.5 sm:col-span-2">
        <span class="text-sm font-medium">Check</span>
        <select name="intervalSecs" class={field}>
          {#each intervalOptions as o (o.secs)}
            <option value={o.secs} selected={o.secs === 3600}>{o.label}</option>
          {/each}
        </select>
      </label>
      <label class="space-y-1.5 sm:col-span-4">
        <span class="text-sm font-medium"
          >Name <span class="font-normal text-muted-foreground">(optional)</span></span
        >
        <input
          name="title"
          type="text"
          maxlength="200"
          placeholder="Taken from the page if left empty"
          value={form?.values?.title ?? ''}
          class={field}
        />
      </label>
      {#if kind === 'page'}
        <label class="space-y-1.5 sm:col-span-6">
          <span class="text-sm font-medium"
            >Only watch this part of the page <span class="font-normal text-muted-foreground"
              >(CSS selector, optional)</span
            ></span
          >
          <input
            name="selector"
            type="text"
            placeholder="main .pricing-table"
            value={form?.values?.selector ?? ''}
            class={field}
          />
        </label>
      {/if}

      <div class="flex flex-wrap items-center gap-3 sm:col-span-6">
        <Button type="submit" disabled={busy}>
          {busy ? 'Checking it now…' : 'Start watching'}
        </Button>
        {#if form?.message}
          <p class="text-sm text-destructive" role="alert">{form.message}</p>
        {/if}
      </div>
    </form>
  </section>

  <section aria-labelledby="list-heading" class="space-y-4">
    <h2 id="list-heading" class="text-lg font-bold tracking-tight">
      Your sources <span class="font-normal text-muted-foreground">({data.sources.length})</span>
    </h2>

    {#if data.sources.length === 0}
      <div class="rounded-3xl border border-dashed p-10 text-center">
        <div
          class="mx-auto grid size-11 place-items-center rounded-2xl bg-accent text-accent-foreground"
        >
          <Eye class="size-5" />
        </div>
        <p class="mt-4 font-semibold">Nothing is being watched yet</p>
        <p class="mt-1 text-sm text-muted-foreground">
          Add a pricing page, a blog feed or a GitHub repo above to get started.
        </p>
      </div>
    {:else}
      <ul class="space-y-3">
        {#each data.sources as s (s.id)}
          {@const st = status(s)}
          <li>
            <a
              href="/app/sources/{s.id}"
              class="block rounded-3xl border bg-card/70 p-5 backdrop-blur transition-shadow hover:shadow-(--shadow-panel) focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
            >
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate font-bold tracking-tight">
                    {s.title || hostOf(s.url)}
                  </p>
                  <p class="truncate text-sm text-muted-foreground">{s.url}</p>
                </div>
                <div class="flex items-center gap-2">
                  <Badge variant="outline">{kindLabels[s.kind]}</Badge>
                  <Badge variant={st.variant}>{st.label}</Badge>
                </div>
              </div>
              <p class="mt-3 text-xs text-muted-foreground">
                {intervalLabel(s.intervalSecs)} · checked {timeAgo(s.lastCheckedAt)} ·
                {s.lastChangedAt ? `changed ${timeAgo(s.lastChangedAt)}` : 'no changes yet'}
                · {s.snapshotCount ?? 0} snapshot{s.snapshotCount === 1 ? '' : 's'}
              </p>
              {#if s.lastError}
                <p class="mt-2 text-sm text-destructive">{s.lastError}</p>
              {/if}
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>
