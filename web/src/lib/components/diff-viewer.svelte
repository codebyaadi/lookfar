<script lang="ts">
  import type { Diff, Hunk } from '#lib/api.ts';
  import { cn } from '#lib/utils.ts';

  let { diff }: { diff: Diff } = $props();

  let mode = $state<'unified' | 'split'>('unified');

  type Row = {
    kind: 'eq' | 'add' | 'del' | 'skip';
    text: string;
    // Split view: the other side of a replaced line, when there is one.
    left?: string;
    right?: string;
  };

  // Flatten hunks into rows. For split view, a run of deletions followed by
  // additions is paired line by line so a changed value sits side by side.
  const rows = $derived.by(() => {
    const out: Row[] = [];
    const hunks: Hunk[] = diff.hunks;
    for (let i = 0; i < hunks.length; i++) {
      const h = hunks[i];
      if (h.op === 'skip') {
        out.push({ kind: 'skip', text: `${h.skipped} unchanged lines` });
      } else if (h.op === 'eq') {
        for (const l of h.lines ?? []) out.push({ kind: 'eq', text: l });
      } else if (h.op === 'del') {
        const next = hunks[i + 1];
        const adds = next?.op === 'add' ? (next.lines ?? []) : [];
        const dels = h.lines ?? [];
        const n = Math.max(dels.length, adds.length);
        for (let k = 0; k < n; k++) {
          out.push({
            kind: k < dels.length ? 'del' : 'add',
            text: k < dels.length ? dels[k] : adds[k],
            left: dels[k],
            right: adds[k],
          });
        }
        if (adds.length) i++;
      } else {
        for (const l of h.lines ?? []) {
          out.push({ kind: 'add', text: l, right: l });
        }
      }
    }
    return out;
  });

  // Unified view lists every deletion before its additions, as in git.
  const unified = $derived.by(() => {
    const out: Row[] = [];
    for (const h of diff.hunks) {
      if (h.op === 'skip') {
        out.push({ kind: 'skip', text: `${h.skipped} unchanged lines` });
        continue;
      }
      for (const l of h.lines ?? []) out.push({ kind: h.op, text: l });
    }
    return out;
  });

  const lineClass = {
    eq: 'text-muted-foreground',
    add: 'bg-[color-mix(in_srgb,var(--chart-2)_16%,transparent)] text-foreground',
    del: 'bg-destructive/12 text-foreground',
    skip: 'bg-muted text-xs text-muted-foreground italic',
  } as const;
  const marker = { eq: ' ', add: '+', del: '-', skip: '' } as const;
</script>

<div class="space-y-3">
  <div class="flex flex-wrap items-center justify-between gap-2">
    <p class="text-sm text-muted-foreground">
      <span class="font-semibold text-[var(--chart-2)]">+{diff.added}</span>
      <span class="font-semibold text-destructive">-{diff.removed}</span>
      lines
    </p>
    <div
      class="inline-flex rounded-full border border-input p-0.5 text-xs font-semibold"
      role="group"
      aria-label="Diff layout"
    >
      {#each ['unified', 'split'] as const as m (m)}
        <button
          type="button"
          class={cn(
            'rounded-full px-3 py-1 capitalize transition-colors',
            mode === m ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'
          )}
          aria-pressed={mode === m}
          onclick={() => (mode = m)}>{m}</button
        >
      {/each}
    </div>
  </div>

  {#if mode === 'unified'}
    <div
      class="overflow-x-auto rounded-2xl border bg-card/60 font-mono text-[13px] leading-6"
    >
      {#each unified as row, i (i)}
        <div class={cn('flex gap-3 px-4 whitespace-pre-wrap', lineClass[row.kind])}>
          <span class="w-3 shrink-0 select-none" aria-hidden="true"
            >{marker[row.kind]}</span
          >
          <span class="min-w-0 break-words">{row.text}</span>
        </div>
      {/each}
    </div>
  {:else}
    <div class="overflow-x-auto rounded-2xl border bg-card/60 font-mono text-[13px] leading-6">
      <div class="grid min-w-[36rem] grid-cols-2 divide-x">
        {#each rows as row, i (i)}
          {#if row.kind === 'skip'}
            <div class={cn('col-span-2 px-4', lineClass.skip)}>{row.text}</div>
          {:else if row.kind === 'eq'}
            <div class={cn('px-4 whitespace-pre-wrap', lineClass.eq)}>{row.text}</div>
            <div class={cn('px-4 whitespace-pre-wrap', lineClass.eq)}>{row.text}</div>
          {:else}
            <div class={cn('px-4 whitespace-pre-wrap', row.left !== undefined && lineClass.del)}>
              {row.left ?? ''}
            </div>
            <div class={cn('px-4 whitespace-pre-wrap', row.right !== undefined && lineClass.add)}>
              {row.right ?? ''}
            </div>
          {/if}
        {/each}
      </div>
    </div>
  {/if}
</div>
