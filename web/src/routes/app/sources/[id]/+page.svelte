<script lang="ts">
  import { enhance } from '$app/forms';
  import { page } from '$app/state';
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import ExternalLink from '@lucide/svelte/icons/external-link';
  import Seo from '#lib/components/seo.svelte';
  import DiffViewer from '#lib/components/diff-viewer.svelte';
  import Badge from '#lib/components/ui/badge.svelte';
  import Button from '#lib/components/ui/button.svelte';
  import {
    formatDateTime,
    hostOf,
    intervalOptions,
    kindLabels,
    timeAgo,
  } from '#lib/format.ts';
  import { cn } from '#lib/utils.ts';
  import type { PageProps } from './$types';

  let { data, form }: PageProps = $props();

  const src = $derived(data.source);
  const cfg = $derived(src.config);

  let checking = $state(false);
  let confirmDelete = $state(false);

  const field =
    'h-10 w-full rounded-full border border-input bg-transparent px-4 text-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none';
  const area =
    'w-full rounded-2xl border border-input bg-transparent px-4 py-3 font-mono text-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none';

  const checkNote = $derived.by(() => {
    const c = form?.check;
    if (!c) return null;
    if (!c.ok) return { tone: 'error', text: c.error ?? 'The check failed' };
    if (c.changed) return { tone: 'ok', text: 'Changed: a new snapshot was saved.' };
    if (c.belowThreshold) return { tone: 'ok', text: 'Content moved, but below your minimum change size.' };
    if (c.notModified) return { tone: 'ok', text: 'No change: the server said nothing was modified.' };
    return { tone: 'ok', text: 'No change since the last snapshot.' };
  });
</script>

<Seo title={src.title || hostOf(src.url)} noindex />

<div class="space-y-8">
  <a
    href="/app/sources"
    class="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground"
  >
    <ArrowLeft class="size-4" /> All sources
  </a>

  <header class="space-y-3">
    <div class="flex flex-wrap items-center gap-2">
      <Badge variant="outline">{kindLabels[src.kind]}</Badge>
      {#if src.paused}
        <Badge variant="outline">Paused</Badge>
      {:else if src.errorCount > 0}
        <Badge variant="destructive">Failing</Badge>
      {:else}
        <Badge variant="accent">Watching</Badge>
      {/if}
    </div>
    <h1 class="text-3xl font-extrabold tracking-tighter text-balance">
      {src.title || hostOf(src.url)}
    </h1>
    <a
      href={src.url}
      target="_blank"
      rel="noreferrer"
      class="inline-flex max-w-full items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
    >
      <span class="truncate">{src.url}</span>
      <ExternalLink class="size-3.5 shrink-0" />
    </a>
    <p class="text-sm text-muted-foreground">
      Checked {timeAgo(src.lastCheckedAt)} · next check {src.paused ? 'paused' : timeAgo(src.nextCheckAt)}
      · {src.lastChangedAt ? `last change ${timeAgo(src.lastChangedAt)}` : 'no changes yet'}
    </p>
  </header>

  {#if data.justCreated}
    <p class="rounded-2xl bg-accent px-4 py-3 text-sm text-accent-foreground" role="status">
      {#if src.lastError}
        Added, but the first check failed. See the error below and adjust the source.
      {:else}
        Added. This first snapshot is your baseline; future changes are compared against it.
      {/if}
    </p>
  {/if}

  {#if src.lastError}
    <div
      class="rounded-2xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm"
      role="alert"
    >
      <p class="font-semibold text-destructive">
        The last check failed ({src.errorCount} in a row)
      </p>
      <p class="mt-1 text-muted-foreground">{src.lastError}</p>
      <p class="mt-1 text-xs text-muted-foreground">
        Lookfar backs off automatically and keeps trying.
      </p>
    </div>
  {/if}

  <div class="flex flex-wrap items-center gap-3">
    <form
      method="POST"
      action="?/check"
      use:enhance={() => {
        checking = true;
        return async ({ update }) => {
          await update();
          checking = false;
        };
      }}
    >
      <Button type="submit" disabled={checking}>
        {checking ? 'Checking…' : 'Check now'}
      </Button>
    </form>

    <form method="POST" action="?/pause" use:enhance>
      <input type="hidden" name="paused" value={src.paused ? 'false' : 'true'} />
      <Button type="submit" variant="outline">{src.paused ? 'Resume' : 'Pause'}</Button>
    </form>

    {#if !confirmDelete}
      <Button variant="ghost" class="text-destructive" onclick={() => (confirmDelete = true)}
        >Delete</Button
      >
    {:else}
      <form method="POST" action="?/remove" use:enhance class="flex items-center gap-2">
        <span class="text-sm text-muted-foreground">Delete it and all its history?</span>
        <Button type="submit" variant="destructive" size="sm">Yes, delete</Button>
        <Button type="button" variant="ghost" size="sm" onclick={() => (confirmDelete = false)}
          >Cancel</Button
        >
      </form>
    {/if}

    {#if checkNote}
      <p
        class={cn('text-sm', checkNote.tone === 'error' ? 'text-destructive' : 'text-muted-foreground')}
        role="status"
      >
        {checkNote.text}
      </p>
    {:else if form?.checkMessage}
      <p class="text-sm text-destructive" role="alert">{form.checkMessage}</p>
    {/if}
  </div>

  <section aria-labelledby="changes-heading" class="space-y-4">
    <h2 id="changes-heading" class="text-xl font-bold tracking-tight">Changes</h2>

    {#if data.snapshots.length === 0}
      <div class="rounded-3xl border border-dashed p-8 text-center text-sm text-muted-foreground">
        No snapshot yet. Run a check to capture the baseline.
      </div>
    {:else}
      <div class="grid gap-6 lg:grid-cols-[16rem_minmax(0,1fr)]">
        <ol class="space-y-2" aria-label="Snapshot history">
          {#each data.snapshots as s (s.id)}
            {@const active = data.snapshot?.id === s.id}
            <li>
              <a
                href="?snapshot={s.id}"
                data-sveltekit-noscroll
                aria-current={active ? 'true' : undefined}
                class={cn(
                  'block rounded-2xl border px-4 py-3 text-sm transition-colors hover:bg-accent/50',
                  active && 'bg-accent'
                )}
              >
                <span class="block font-semibold">{formatDateTime(s.fetchedAt)}</span>
                <span class="mt-0.5 block text-xs text-muted-foreground">
                  {#if s.baseline}
                    Baseline snapshot
                  {:else}
                    <span class="text-[var(--chart-2)]">+{s.addedLines}</span>
                    <span class="text-destructive">-{s.removedLines}</span> lines
                  {/if}
                </span>
              </a>
            </li>
          {/each}
        </ol>

        <div class="min-w-0">
          {#if data.snapshot}
            <p class="mb-3 text-sm text-muted-foreground">
              Captured {formatDateTime(data.snapshot.fetchedAt)}
            </p>
            {#if data.snapshot.diff}
              <DiffViewer diff={data.snapshot.diff} />
            {:else}
              <div class="space-y-2">
                <p class="text-sm text-muted-foreground">
                  This is the baseline: everything Lookfar saw on the first check.
                </p>
                <pre
                  class="max-h-[28rem] overflow-auto rounded-2xl border bg-card/60 p-4 font-mono text-[13px] leading-6 whitespace-pre-wrap">{data.snapshot.content}</pre>
              </div>
            {/if}
          {/if}
        </div>
      </div>
    {/if}
  </section>

  <section aria-labelledby="settings-heading">
    <details class="group rounded-3xl border bg-card/70 px-6 py-5 backdrop-blur" open={Boolean(form?.message)}>
      <summary
        id="settings-heading"
        class="cursor-pointer list-none text-lg font-bold tracking-tight marker:hidden"
      >
        Settings
      </summary>

      <form method="POST" action="?/save" use:enhance class="mt-5 grid gap-4 sm:grid-cols-2">
        <label class="space-y-1.5 sm:col-span-2">
          <span class="text-sm font-medium">Name</span>
          <input name="title" type="text" maxlength="200" value={src.title} class={field} />
        </label>
        <label class="space-y-1.5">
          <span class="text-sm font-medium">Check</span>
          <select name="intervalSecs" class={field}>
            {#each intervalOptions as o (o.secs)}
              <option value={o.secs} selected={o.secs === src.intervalSecs}>{o.label}</option>
            {/each}
            {#if !intervalOptions.some((o) => o.secs === src.intervalSecs)}
              <option value={src.intervalSecs} selected>Every {Math.round(src.intervalSecs / 60)} minutes</option>
            {/if}
          </select>
        </label>
        <label class="space-y-1.5">
          <span class="text-sm font-medium">Ignore changes smaller than</span>
          <span class="flex items-center gap-2">
            <input
              name="minChangeChars"
              type="number"
              min="0"
              max="100000"
              value={cfg.minChangeChars ?? 0}
              class={field}
            />
            <span class="text-sm text-muted-foreground">characters</span>
          </span>
        </label>

        {#if src.kind === 'page'}
          <label class="space-y-1.5 sm:col-span-2">
            <span class="text-sm font-medium">Only watch this part of the page (CSS selector)</span>
            <input name="selector" type="text" value={cfg.selector ?? ''} placeholder="main .pricing-table" class={field} />
          </label>
          <label class="space-y-1.5">
            <span class="text-sm font-medium">Ignore these elements (one selector per line)</span>
            <textarea name="ignoreSelectors" rows="3" class={area} placeholder=".promo-banner&#10;footer"
              >{(cfg.ignoreSelectors ?? []).join('\n')}</textarea
            >
          </label>
          <label class="space-y-1.5">
            <span class="text-sm font-medium">Ignore lines matching (one regex per line)</span>
            <textarea name="ignorePatterns" rows="3" class={area} placeholder="^Visitors today"
              >{(cfg.ignorePatterns ?? []).join('\n')}</textarea
            >
          </label>
          <label class="flex items-start gap-3 sm:col-span-2">
            <input
              name="keepTimestamps"
              type="checkbox"
              checked={cfg.keepTimestamps ?? false}
              class="mt-1 size-4 accent-[var(--brand)]"
            />
            <span class="text-sm">
              <span class="font-medium">Treat dates and times as content</span>
              <span class="block text-muted-foreground">
                Off by default, so "updated 5 minutes ago" and clock text never count as a change.
              </span>
            </span>
          </label>
        {/if}

        {#if src.kind === 'github'}
          <label class="flex items-start gap-3 sm:col-span-2">
            <input
              name="includePrereleases"
              type="checkbox"
              checked={cfg.includePrereleases ?? false}
              class="mt-1 size-4 accent-[var(--brand)]"
            />
            <span class="text-sm font-medium">Include pre-releases</span>
          </label>
        {/if}

        <div class="flex flex-wrap items-center gap-3 sm:col-span-2">
          <Button type="submit">Save settings</Button>
          {#if form?.saved}
            <p class="text-sm text-muted-foreground" role="status">Saved.</p>
          {/if}
          {#if form?.message}
            <p class="text-sm text-destructive" role="alert">{form.message}</p>
          {/if}
        </div>
      </form>
    </details>
  </section>
</div>
