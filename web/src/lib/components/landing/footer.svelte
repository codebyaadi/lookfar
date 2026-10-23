<script lang="ts">
  import Logo from '#lib/components/logo.svelte';

  const year = new Date().getFullYear();
</script>

<footer class="border-t px-4 py-12">
  <div
    class="mx-auto flex max-w-5xl flex-col items-start justify-between gap-8 sm:flex-row sm:items-center"
  >
    <div class="space-y-3">
      <Logo />
      <p class="max-w-xs text-sm text-muted-foreground">
        Agents that look far, so you don't have to.
      </p>
    </div>
    <nav aria-label="Footer" class="flex flex-wrap gap-x-6 gap-y-2 text-sm">
      <a href="#how" class="text-muted-foreground hover:text-foreground"
        >How it works</a
      >
      <a href="#features" class="text-muted-foreground hover:text-foreground"
        >Features</a
      >
      <a href="#faq" class="text-muted-foreground hover:text-foreground">FAQ</a>
      <a
        href="https://github.com/codebyaadi/lookfar"
        target="_blank"
        rel="noreferrer"
        class="text-muted-foreground hover:text-foreground">GitHub</a
      >
    </nav>
  </div>
  <p class="mx-auto mt-8 max-w-5xl text-xs text-muted-foreground">
    © {year} Lookfar · MIT licensed
  </p>
</footer>
