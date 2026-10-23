<script lang="ts">
  import '../app.css';
  import '@fontsource-variable/sora';
  import '@fontsource-variable/geist-mono';
  import { ModeWatcher } from 'mode-watcher';
  import favicon from '#lib/assets/favicon.svg';
  import type { LayoutProps } from './$types';

  let { children }: LayoutProps = $props();
</script>

<svelte:head>
  <title>Lookfar</title>
  <link rel="icon" href={favicon} />
</svelte:head>

<ModeWatcher />
{@render children()}
