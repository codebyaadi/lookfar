<script lang="ts">
  import Logo from '#lib/components/logo.svelte';
</script>

<main
  class="ambient mx-auto flex min-h-screen max-w-3xl flex-col justify-center gap-8 px-6"
>
  <Logo />
  <h1
    class="text-5xl leading-[1.02] font-extrabold tracking-tighter text-balance sm:text-6xl"
  >
    Agents you can leave running<span class="text-brand">.</span>
  </h1>
  <p class="text-lg leading-relaxed text-pretty text-muted-foreground">
    Describe what to watch in plain language. Lookfar checks pages, feeds and
    repos on a schedule, tells you only what matters, and asks before doing
    anything with side effects.
  </p>
  <div class="flex gap-3">
    <a
      href="/signup"
      class="rounded-full bg-primary px-6 py-2.5 font-semibold text-primary-foreground"
      >Get started</a
    >
    <a
      href="/signin"
      class="rounded-full border border-input px-6 py-2.5 font-semibold"
      >Sign in</a
    >
  </div>
</main>
