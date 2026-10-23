<script lang="ts">
  import ArrowRight from '@lucide/svelte/icons/arrow-right';
  import Button from '#lib/components/ui/button.svelte';
</script>

<section class="px-4 py-24">
  <div
    class="mx-auto max-w-5xl rounded-[2rem] border bg-card/70 p-8 text-center shadow-(--shadow-panel) backdrop-blur-xl sm:p-16"
  >
    <h2
      class="mx-auto max-w-2xl text-4xl font-extrabold tracking-tighter text-balance sm:text-5xl"
    >
      Put an agent on watch<span class="text-brand">.</span>
    </h2>
    <p
      class="mx-auto mt-5 max-w-xl text-lg leading-relaxed text-pretty text-muted-foreground"
    >
      Create your account, point Lookfar at the pages that matter, and get on
      with your day.
    </p>
    <div class="mt-8 flex justify-center">
      <Button href="/signup" size="lg">
        Get started free <ArrowRight class="size-4" />
      </Button>
    </div>
  </div>
</section>
