<script lang="ts">
  import { goto } from '$app/navigation';
  import { authClient } from '#lib/auth-client.ts';

  let email = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    busy = true;
    error = '';
    const { error: err } = await authClient.signIn.email({ email, password });
    busy = false;
    if (err) error = err.message ?? 'Could not sign in';
    else await goto('/app');
  }
</script>

<form onsubmit={submit} class="space-y-4">
  <h1 class="text-2xl font-extrabold tracking-tight">Sign in</h1>
  <input
    type="email"
    bind:value={email}
    placeholder="Email"
    required
    autocomplete="email"
    class="w-full rounded-full border border-input bg-background px-4 py-2.5"
  />
  <input
    type="password"
    bind:value={password}
    placeholder="Password"
    required
    autocomplete="current-password"
    class="w-full rounded-full border border-input bg-background px-4 py-2.5"
  />
  {#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
  <button
    disabled={busy}
    class="w-full rounded-full bg-primary py-2.5 font-semibold text-primary-foreground disabled:opacity-60"
  >
    {busy ? 'Signing in...' : 'Sign in'}
  </button>
  <p class="text-sm text-muted-foreground">
    No account? <a href="/signup" class="font-semibold text-brand">Sign up</a>
  </p>
</form>
