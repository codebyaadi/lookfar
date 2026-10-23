<script lang="ts">
  import { cn } from '#lib/utils.ts';

  let {
    class: className = '',
    markOnly = false,
  }: { class?: string; markOnly?: boolean } = $props();
</script>

<span class={cn('inline-flex items-center gap-2.5', className)}>
  <span
    class="grid size-9 shrink-0 place-items-center rounded-xl bg-primary text-primary-foreground shadow-sm"
  >
    <svg
      viewBox="0 0 32 32"
      fill="none"
      aria-hidden="true"
      class="size-6"
      stroke-linecap="round"
    >
      <circle cx="9.5" cy="22.5" r="3.6" fill="currentColor" />
      <path
        d="M6.5 12.2a13.3 13.3 0 0 1 13.3 13.3"
        stroke="currentColor"
        stroke-width="4.6"
      />
      <path
        d="M6.5 4.6A20.9 20.9 0 0 1 27.4 25.5"
        stroke="currentColor"
        stroke-width="4.6"
      />
    </svg>
  </span>
  {#if markOnly}
    <span class="sr-only">Lookfar</span>
  {:else}
    <span class="text-[1.4rem] leading-none font-extrabold tracking-tighter">
      Lookfar<span class="text-brand">.</span>
    </span>
  {/if}
</span>
