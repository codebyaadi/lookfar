<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import Logo from '#lib/components/logo.svelte';
  import ModeToggle from '#lib/components/mode-toggle.svelte';
  import Button from '#lib/components/ui/button.svelte';
  import { authClient } from '#lib/auth-client.ts';
  import { cn } from '#lib/utils.ts';
  import type { LayoutProps } from './$types';

  let { data, children }: LayoutProps = $props();

  const nav = [
    { href: '/app', label: 'Inbox', exact: true },
    { href: '/app/sources', label: 'Sources', exact: false },
  ];

  const isActive = (href: string, exact: boolean) =>
    exact
      ? page.url.pathname === href
      : page.url.pathname === href || page.url.pathname.startsWith(`${href}/`);

  async function signOut() {
    await authClient.signOut();
    await goto('/signin');
  }
</script>

<div class="ambient min-h-screen">
  <header
    class="flex flex-wrap items-center justify-between gap-x-4 gap-y-3 border-b border-border px-4 py-3 sm:px-6 sm:py-4"
  >
    <a href="/app" aria-label="Lookfar inbox"><Logo /></a>

    <nav
      aria-label="App"
      class="order-last flex w-full gap-1 sm:order-none sm:w-auto"
    >
      {#each nav as item (item.href)}
        <Button
          href={item.href}
          variant={isActive(item.href, item.exact) ? 'secondary' : 'ghost'}
          size="sm"
          class={cn(!isActive(item.href, item.exact) && 'text-muted-foreground')}
          aria-current={isActive(item.href, item.exact) ? 'page' : undefined}
          >{item.label}</Button
        >
      {/each}
    </nav>

    <div class="flex items-center gap-3 text-sm">
      <span class="hidden text-muted-foreground md:inline">{data.user.email}</span>
      <ModeToggle />
      <Button variant="outline" size="sm" onclick={signOut}>Sign out</Button>
    </div>
  </header>
  <div class="mx-auto max-w-5xl px-4 py-8 sm:px-6 sm:py-10">{@render children()}</div>
</div>
