<script lang="ts">
  const steps = [
    {
      n: '01',
      title: 'Describe the mission',
      body: 'Write what to watch and what counts as important, in plain language. Add pages, feeds or GitHub repos as sources.',
    },
    {
      n: '02',
      title: 'Lookfar watches quietly',
      body: 'On a schedule it fetches, diffs and triages. Unchanged pages cost nothing. Real changes get a short, sourced report.',
    },
    {
      n: '03',
      title: 'You stay in control',
      body: 'Reports, alerts and questions land in one inbox. Anything outward-facing waits for your approval, from your phone if you like.',
    },
  ];
</script>

<section id="how" class="scroll-mt-24 px-4 py-24">
  <div
    class="mx-auto max-w-5xl rounded-[2rem] bg-primary p-8 text-primary-foreground sm:p-14"
  >
    <p class="text-sm font-semibold opacity-70">How it works</p>
    <h2
      class="mt-3 max-w-2xl text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      Three steps, then forget about it.
    </h2>
    <ol class="mt-12 grid gap-10 md:grid-cols-3">
      {#each steps as s (s.n)}
        <li>
          <span class="text-5xl font-extrabold tracking-tighter opacity-30"
            >{s.n}</span
          >
          <h3 class="mt-4 text-xl font-bold tracking-tight">{s.title}</h3>
          <p class="mt-2 leading-relaxed opacity-80">{s.body}</p>
        </li>
      {/each}
    </ol>
  </div>
</section>
