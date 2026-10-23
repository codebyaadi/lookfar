<script lang="ts">
  import GitCompare from '@lucide/svelte/icons/git-compare';
  import Brain from '@lucide/svelte/icons/brain';
  import ListChecks from '@lucide/svelte/icons/list-checks';
  import Plug from '@lucide/svelte/icons/plug';
  import Server from '@lucide/svelte/icons/server';
  import Wallet from '@lucide/svelte/icons/wallet';

  const features = [
    {
      icon: Server,
      title: 'Self-hosted, your keys',
      body: 'One compose file brings up Postgres, the API, the worker and the web app. Add your own Anthropic, OpenAI or OpenRouter keys, or start on the free default.',
      live: true,
    },
    {
      icon: ListChecks,
      title: 'Approvals inbox',
      body: 'Reports, alerts, questions and approvals in one place, with push and email notifications. Approvals show the exact recipient and payload.',
      live: false,
    },
    {
      icon: Wallet,
      title: 'Budgets and a full run log',
      body: 'Per-run and monthly spend caps. Every run has a step-by-step timeline with tokens, cost and duration for each step.',
      live: false,
    },
    {
      icon: Brain,
      title: 'Memory that sticks',
      body: 'Reports and snapshots are chunked, embedded and searchable, so agents remember what changed last time.',
      live: false,
    },
    {
      icon: Plug,
      title: 'MCP for your other AI tools',
      body: 'Connect Claude, Cursor or ChatGPT to your memory, and let any agent ask you for an approval through the same inbox.',
      live: false,
    },
    {
      icon: GitCompare,
      title: 'Change detection that ignores noise',
      body: 'Watch pages, RSS feeds and GitHub releases. Text is normalised, cookie banners and "5 minutes ago" are ignored, and every change is stored as a readable diff.',
      live: true,
    },
  ];
</script>

<section id="features" class="scroll-mt-24 px-4 py-24">
  <div class="mx-auto max-w-5xl">
    <p class="text-center text-sm font-semibold text-brand">Features</p>
    <h2
      class="mx-auto mt-3 max-w-3xl text-center text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      Built for trust, not for demos.
    </h2>
    <div class="mt-14 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {#each features as f (f.title)}
        <div
          class="rounded-3xl border bg-card/70 p-7 backdrop-blur transition-shadow hover:shadow-(--shadow-panel)"
        >
          <div class="flex items-start justify-between">
            <div
              class="grid size-11 place-items-center rounded-2xl bg-accent text-accent-foreground"
            >
              <f.icon class="size-5" />
            </div>
            <span
              class={[
                'rounded-full px-2.5 py-1 text-xs font-semibold',
                f.live
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-secondary text-secondary-foreground',
              ]}>{f.live ? 'Live' : 'Coming soon'}</span
            >
          </div>
          <h3 class="mt-6 text-xl font-bold tracking-tight">{f.title}</h3>
          <p class="mt-2 leading-relaxed text-muted-foreground">{f.body}</p>
        </div>
      {/each}
    </div>
  </div>
</section>
