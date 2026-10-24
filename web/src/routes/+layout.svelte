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
  <link rel="icon" href={favicon} type="image/svg+xml" />
  <link rel="apple-touch-icon" href="/apple-touch-icon.png" />
  <link rel="manifest" href="/manifest.webmanifest" />
  <meta name="theme-color" content="#fbf5eb" media="(prefers-color-scheme: light)" />
  <meta name="theme-color" content="#0f0b07" media="(prefers-color-scheme: dark)" />
</svelte:head>

<ModeWatcher />
{@render children()}
