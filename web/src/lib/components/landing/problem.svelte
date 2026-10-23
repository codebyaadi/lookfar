<script lang="ts">
  import BellOff from '@lucide/svelte/icons/bell-off';
  import Coins from '@lucide/svelte/icons/coins';
  import Lock from '@lucide/svelte/icons/lock';

  const points = [
    {
      icon: BellOff,
      title: 'Alerts that cry wolf',
      body: 'Page monitors fire on every cookie banner and "updated 3 minutes ago". Lookfar judges relevance against your mission and stays quiet unless it matters.',
    },
    {
      icon: Coins,
      title: 'AI that burns tokens on nothing',
      body: 'Most agents re-read the web on every run. Lookfar diffs first, with no model, and only calls one when something actually changed.',
    },
    {
      icon: Lock,
      title: 'Agents you cannot trust alone',
      body: 'Anything with side effects waits in your inbox with the exact payload shown. Nothing is sent, posted or called without a yes.',
    },
  ];
</script>

<section class="px-4 py-24">
  <div class="mx-auto max-w-5xl">
    <p class="text-center text-sm font-semibold text-brand">Why Lookfar</p>
    <h2
      class="mx-auto mt-3 max-w-3xl text-center text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      Background AI you can actually leave running.
    </h2>
    <div class="mt-14 grid gap-4 md:grid-cols-3">
      {#each points as p (p.title)}
        <div
          class="rounded-3xl border bg-card/70 p-7 backdrop-blur transition-shadow hover:shadow-(--shadow-panel)"
        >
          <div
            class="grid size-11 place-items-center rounded-2xl bg-accent text-accent-foreground"
          >
            <p.icon class="size-5" />
          </div>
          <h3 class="mt-6 text-xl font-bold tracking-tight">{p.title}</h3>
          <p class="mt-2 leading-relaxed text-muted-foreground">{p.body}</p>
        </div>
      {/each}
    </div>
  </div>
</section>
