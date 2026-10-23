<script lang="ts">
  import Button from '#lib/components/ui/button.svelte';

  const compose = `git clone https://github.com/codebyaadi/feedoku lookfar
cd lookfar
cp .env.example .env     # set BETTER_AUTH_SECRET
docker compose --profile app up -d`;

  const mcp = `{
  "mcpServers": {
    "lookfar": {
      "url": "https://your-host/mcp",
      "headers": { "Authorization": "Bearer lf_…" }
    }
  }
}`;
</script>

<section class="px-4 py-24">
  <div class="mx-auto max-w-5xl">
    <p class="text-center text-sm font-semibold text-brand">Own your stack</p>
    <h2
      class="mx-auto mt-3 max-w-3xl text-center text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      Your server. Your keys. Your memory.
    </h2>
    <p
      class="mx-auto mt-5 max-w-2xl text-center text-lg leading-relaxed text-pretty text-muted-foreground"
    >
      Lookfar runs on a small VPS. Everything agents read and write stays in
      your own Postgres, and your other AI tools can use it over MCP.
    </p>

    <div class="mt-14 grid gap-4 md:grid-cols-2">
      <div class="rounded-3xl border bg-card/70 p-7 backdrop-blur">
        <h3 class="text-lg font-bold tracking-tight">Run it anywhere</h3>
        <pre
          class="mt-4 overflow-x-auto rounded-2xl bg-muted p-4 font-mono text-xs leading-relaxed"><code
            >{compose}</code
          ></pre>
      </div>
      <div class="rounded-3xl border bg-card/70 p-7 backdrop-blur">
        <div class="flex items-center justify-between">
          <h3 class="text-lg font-bold tracking-tight">Plug in your AI tools</h3>
          <span
            class="rounded-full bg-secondary px-2.5 py-1 text-xs font-semibold text-secondary-foreground"
            >Coming soon</span
          >
        </div>
        <pre
          class="mt-4 overflow-x-auto rounded-2xl bg-muted p-4 font-mono text-xs leading-relaxed"><code
            >{mcp}</code
          ></pre>
      </div>
    </div>

    <div class="mt-8 flex justify-center">
      <Button
        href="https://github.com/codebyaadi/feedoku"
        variant="outline"
        target="_blank"
        rel="noreferrer">View on GitHub</Button
      >
    </div>
  </div>
</section>
