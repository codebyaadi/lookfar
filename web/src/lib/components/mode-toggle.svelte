<script lang="ts">
  import Moon from '@lucide/svelte/icons/moon';
  import Sun from '@lucide/svelte/icons/sun';
  import { toggleMode } from 'mode-watcher';
  import Button from '#lib/components/ui/button.svelte';
</script>

<Button
  variant="ghost"
  size="sm"
  class="size-8 px-0"
  onclick={toggleMode}
  aria-label="Toggle theme"
>
  <Sun class="size-4 dark:hidden" />
  <Moon class="hidden size-4 dark:block" />
</Button>
