<script lang="ts">
  const stages = [
    {
      name: 'Detect',
      who: 'No model',
      cost: 'Free',
      body: 'Fetch, normalise, hash and diff. If nothing changed, the run ends here.',
    },
    {
      name: 'Triage',
      who: 'Small model',
      cost: 'Fractions of a cent',
      body: 'One cheap call decides whether the change matters to your mission.',
    },
    {
      name: 'Act',
      who: 'Your main model',
      cost: 'Only when it matters',
      body: 'A tool loop with a hard budget reads, searches memory and writes the report.',
    },
  ];
</script>

<section class="px-4 py-24">
  <div class="mx-auto max-w-5xl">
    <p class="text-center text-sm font-semibold text-brand">
      No change, no tokens
    </p>
    <h2
      class="mx-auto mt-3 max-w-3xl text-center text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      Three gates between the web and your bill.
    </h2>
    <p
      class="mx-auto mt-5 max-w-2xl text-center text-lg leading-relaxed text-pretty text-muted-foreground"
    >
      Every run passes through cheaper checks before it reaches an expensive
      model, and every step shows its cost.
    </p>

    <ol class="mt-14 grid gap-4 md:grid-cols-3">
      {#each stages as s, i (s.name)}
        <li
          class="relative rounded-3xl border bg-card/70 p-7 backdrop-blur"
        >
          <div class="flex items-center justify-between">
            <span class="text-sm font-semibold text-brand">{i + 1}. {s.name}</span>
            <span
              class="rounded-full bg-secondary px-2.5 py-1 text-xs font-medium text-secondary-foreground"
              >{s.who}</span
            >
          </div>
          <p class="mt-5 text-2xl font-extrabold tracking-tighter">{s.cost}</p>
          <p class="mt-2 leading-relaxed text-muted-foreground">{s.body}</p>
        </li>
      {/each}
    </ol>
  </div>
</section>
