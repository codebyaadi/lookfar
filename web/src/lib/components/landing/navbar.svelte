<script lang="ts">
  import Logo from '#lib/components/logo.svelte';
  import ModeToggle from '#lib/components/mode-toggle.svelte';
  import Button from '#lib/components/ui/button.svelte';

  let { signedIn = false }: { signedIn?: boolean } = $props();

  const links = [
    { href: '#how', label: 'How it works' },
    { href: '#agents', label: 'Agents' },
    { href: '#features', label: 'Features' },
    { href: '#faq', label: 'FAQ' },
  ];
</script>

<header class="fixed inset-x-0 top-0 z-40 px-3 pt-3 sm:px-6">
  <nav
    aria-label="Main"
    class="mx-auto flex h-14 max-w-5xl items-center justify-between rounded-full border bg-card/70 pr-2 pl-4 shadow-(--shadow-panel) backdrop-blur-xl"
  >
    <a href="/" aria-label="Lookfar home"><Logo /></a>

    <ul class="hidden items-center gap-1 lg:flex">
      {#each links as l (l.href)}
        <li>
          <Button
            href={l.href}
            variant="ghost"
            size="sm"
            class="text-muted-foreground">{l.label}</Button
          >
        </li>
      {/each}
    </ul>

    <div class="flex items-center gap-1.5">
      <ModeToggle />
      {#if signedIn}
        <Button href="/app" size="sm">Open inbox</Button>
      {:else}
        <Button href="/signin" variant="ghost" size="sm" class="hidden sm:inline-flex"
          >Sign in</Button
        >
        <Button href="/signup" size="sm">Get started</Button>
      {/if}
    </div>
  </nav>
</header>
