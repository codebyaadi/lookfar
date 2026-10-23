<script lang="ts">
  import Briefcase from '@lucide/svelte/icons/briefcase';
  import Newspaper from '@lucide/svelte/icons/newspaper';
  import PackageSearch from '@lucide/svelte/icons/package-search';
  import Tag from '@lucide/svelte/icons/tag';

  const agents = [
    {
      icon: Tag,
      name: 'Competitor pricing',
      mission:
        'Check these 4 pricing pages every Monday. Report only real price, plan or limit changes, not copy edits.',
    },
    {
      icon: PackageSearch,
      name: 'Dependency radar',
      mission:
        'Watch Go, Postgres and pgx releases. Tell me only about breaking changes or security fixes that affect a Go + Postgres 17 app.',
    },
    {
      icon: Briefcase,
      name: 'Opportunity scout',
      mission:
        'Watch these grant and job boards. Alert me about remote Go roles over $150k or grants for open-source infra.',
    },
    {
      icon: Newspaper,
      name: 'Story tracker',
      mission:
        'Follow this topic across my feeds. Send a short "what changed since last time" each evening.',
    },
  ];
</script>

<section id="agents" class="scroll-mt-24 px-4 py-24">
  <div class="mx-auto max-w-5xl">
    <p class="text-center text-sm font-semibold text-brand">Starter agents</p>
    <h2
      class="mx-auto mt-3 max-w-3xl text-center text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      A mission is all it takes.
    </h2>
    <div class="mt-14 grid gap-4 sm:grid-cols-2">
      {#each agents as a (a.name)}
        <div
          class="rounded-3xl border bg-card/70 p-7 backdrop-blur transition-shadow hover:shadow-(--shadow-panel)"
        >
          <div
            class="grid size-11 place-items-center rounded-2xl bg-accent text-accent-foreground"
          >
            <a.icon class="size-5" />
          </div>
          <h3 class="mt-6 text-xl font-bold tracking-tight">
            {a.name}
          </h3>
          <p class="mt-2 leading-relaxed text-muted-foreground">
            "{a.mission}"
          </p>
        </div>
      {/each}
    </div>
  </div>
</section>
