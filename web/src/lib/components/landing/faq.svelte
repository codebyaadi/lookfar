<script lang="ts">
  import ChevronDown from '@lucide/svelte/icons/chevron-down';

  const items = [
    {
      q: 'Is Lookfar ready to use today?',
      a: 'It is early. Accounts, API tokens, and watching pages, feeds and GitHub releases with diffs work now. The agent runtime, approvals, memory and MCP are being built in that order, and every feature on this page is labelled Live or Coming soon.',
    },
    {
      q: 'What will it cost to run?',
      a: 'Lookfar is free and open source. You pay only for your server and for whatever LLM provider you choose. Change detection is free, a small model triages, and per-run and monthly budgets stop spend cleanly.',
    },
    {
      q: 'Which models can I use?',
      a: 'Any OpenAI-compatible provider (Groq, OpenRouter, OpenAI, local Ollama or LM Studio) and Anthropic natively. A free Groq → OpenRouter chain is the zero-config default, though tool-heavy agents work best with a stronger model of your own.',
    },
    {
      q: 'Can an agent do something I did not approve?',
      a: 'No. In v1 every outward action, such as sending an email or calling a webhook, goes through the approvals inbox, and the card shows the exact payload. Fetched web content is treated as untrusted data, and webhooks can only reach endpoints you registered.',
    },
    {
      q: 'Does it work on JavaScript-heavy pages?',
      a: 'Version one fetches pages over plain HTTP and extracts readable text or a CSS selector. Headless-browser sources are on the roadmap as an opt-in.',
    },
    {
      q: 'Is it multi-user?',
      a: 'It is built self-host first for one person or a small team. The first account becomes the admin and sign-up then closes, with invites coming later.',
    },
  ];
</script>

<section id="faq" class="scroll-mt-24 px-4 py-24">
  <div class="mx-auto max-w-3xl">
    <p class="text-center text-sm font-semibold text-brand">FAQ</p>
    <h2
      class="mt-3 text-center text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      Straight answers.
    </h2>
    <div class="mt-12 space-y-3">
      {#each items as item (item.q)}
        <details
          class="group rounded-3xl border bg-card/70 px-6 py-5 backdrop-blur open:shadow-(--shadow-panel)"
        >
          <summary
            class="flex cursor-pointer list-none items-center justify-between gap-4 font-semibold"
          >
            {item.q}
            <ChevronDown
              class="size-5 shrink-0 text-muted-foreground transition-transform group-open:rotate-180"
            />
          </summary>
          <p class="mt-3 leading-relaxed text-muted-foreground">{item.a}</p>
        </details>
      {/each}
    </div>
  </div>
</section>
