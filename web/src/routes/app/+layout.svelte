<script lang="ts">
  import { goto } from '$app/navigation';
  import Logo from '#lib/components/logo.svelte';
  import { authClient } from '#lib/auth-client.ts';
  import type { LayoutProps } from './$types';

  let { data, children }: LayoutProps = $props();

  async function signOut() {
    await authClient.signOut();
    await goto('/signin');
  }
</script>

<div class="ambient min-h-screen">
  <header
    class="flex items-center justify-between border-b border-border px-6 py-4"
  >
    <a href="/app"><Logo /></a>
    <div class="flex items-center gap-4 text-sm">
      <span class="text-muted-foreground">{data.user.email}</span>
      <button
        onclick={signOut}
        class="rounded-full border border-input px-4 py-1.5 font-semibold"
        >Sign out</button
      >
    </div>
  </header>
  <div class="mx-auto max-w-3xl px-6 py-10">{@render children()}</div>
</div>
