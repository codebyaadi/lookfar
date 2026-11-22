<script lang="ts">
  import { Button } from 'bits-ui';
  import GithubLogo from 'phosphor-svelte/lib/GithubLogo';
</script>

<main
  class="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 p-8"
>
  <h1 class="text-3xl font-semibold">lookfar</h1>
  <p class="text-neutral-600">
    Dashboard coming soon. lookfar is in early development.
  </p>
  <p class="font-mono text-sm text-neutral-500">lookfar version v0.0.0-dev</p>
  <Button.Root
    class="inline-flex w-fit items-center gap-2 rounded-md bg-neutral-900 px-4 py-2 text-white"
    href="https://github.com/codebyaadi/lookfar"
  >
    <GithubLogo size={18} weight="bold" />
    Source on GitHub
  </Button.Root>
</main>
