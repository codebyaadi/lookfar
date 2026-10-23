<script lang="ts">
  import Check from '@lucide/svelte/icons/check';
  import GitCompare from '@lucide/svelte/icons/git-compare';
  import Globe from '@lucide/svelte/icons/globe';
  import ScrollText from '@lucide/svelte/icons/scroll-text';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import Sparkles from '@lucide/svelte/icons/sparkles';

  const steps = [
    {
      icon: Globe,
      title: 'Checked 4 pricing pages',
      meta: '1.2s · no model used',
    },
    {
      icon: GitCompare,
      title: '1 of 4 pages changed',
      meta: 'Pro plan: $49 → $59, limits unchanged',
    },
    {
      icon: Sparkles,
      title: 'Triage: relevant, high significance',
      meta: 'Real price change, not a copy edit · small model',
    },
    {
      icon: ScrollText,
      title: 'Report written',
      meta: 'Saved to memory · push sent',
    },
  ];
</script>

<div
  class="overflow-hidden rounded-3xl border bg-card/70 text-left shadow-(--shadow-panel) backdrop-blur-xl"
  aria-label="Example agent run"
  role="img"
>
  <div class="flex items-center justify-between border-b px-5 py-3.5">
    <div class="flex items-center gap-3">
      <span class="flex gap-1.5" aria-hidden="true">
        <span class="size-2.5 rounded-full bg-muted-foreground/30"></span>
        <span class="size-2.5 rounded-full bg-muted-foreground/30"></span>
        <span class="size-2.5 rounded-full bg-muted-foreground/30"></span>
      </span>
      <span class="text-sm font-semibold">Competitor pricing</span>
      <span class="hidden text-xs text-muted-foreground sm:inline"
        >Run #42 · Monday 09:00</span
      >
    </div>
    <span
      class="rounded-full bg-accent px-2.5 py-1 text-xs font-semibold text-accent-foreground"
      >Cost $0.004</span
    >
  </div>

  <div class="grid gap-0 md:grid-cols-[1.15fr_1fr]">
    <ol class="space-y-5 p-5 sm:p-6">
      {#each steps as s (s.title)}
        <li class="flex gap-3.5">
          <span
            class="grid size-9 shrink-0 place-items-center rounded-xl bg-accent text-accent-foreground"
          >
            <s.icon class="size-4" />
          </span>
          <div class="min-w-0">
            <p class="text-sm font-semibold">{s.title}</p>
            <p class="text-xs leading-relaxed text-muted-foreground">
              {s.meta}
            </p>
          </div>
        </li>
      {/each}
    </ol>

    <div class="border-t bg-muted/60 p-5 sm:p-6 md:border-t-0 md:border-l">
      <div class="flex items-center gap-2 text-xs font-semibold text-brand">
        <ShieldCheck class="size-4" /> Needs your approval
      </div>
      <p class="mt-3 text-sm font-semibold">Send email to team@acme.dev</p>
      <div
        class="mt-3 rounded-2xl border bg-card/80 p-3.5 font-mono text-xs leading-relaxed text-muted-foreground"
      >
        <p>
          <span class="text-foreground">Subject:</span> Rival raised Pro to $59
        </p>
        <p class="mt-1.5">
          Their Pro plan went from $49 to $59 today. Limits are unchanged.
        </p>
      </div>
      <div class="mt-4 flex gap-2">
        <span
          class="inline-flex h-8 items-center gap-1.5 rounded-full bg-primary px-3.5 text-sm font-semibold text-primary-foreground"
        >
          <Check class="size-3.5" /> Approve
        </span>
        <span
          class="inline-flex h-8 items-center rounded-full border border-input px-3.5 text-sm font-semibold"
          >Deny</span
        >
      </div>
    </div>
  </div>
</div>
