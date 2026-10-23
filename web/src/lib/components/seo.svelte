<script lang="ts">
  import { page } from '$app/state';
  import { site } from '#lib/site.ts';

  type Props = {
    /** Page title. The site name is appended unless it is the full default. */
    title?: string;
    description?: string;
    /** Keep the page out of search results. */
    noindex?: boolean;
  };

  let { title, description = site.description, noindex = false }: Props =
    $props();

  const fullTitle = $derived(
    title === undefined || title === site.title
      ? site.title
      : `${title} · ${site.name}`
  );
  const canonical = $derived(`${site.url}${page.url.pathname}`);
  const image = `${site.url}${site.ogImage}`;
</script>

<svelte:head>
  <title>{fullTitle}</title>
  <meta name="description" content={description} />
  <link rel="canonical" href={canonical} />
  <meta
    name="robots"
    content={noindex ? 'noindex, nofollow' : 'index, follow, max-image-preview:large'}
  />

  <meta property="og:type" content="website" />
  <meta property="og:site_name" content={site.name} />
  <meta property="og:title" content={fullTitle} />
  <meta property="og:description" content={description} />
  <meta property="og:url" content={canonical} />
  <meta property="og:image" content={image} />
  <meta property="og:image:width" content="1200" />
  <meta property="og:image:height" content="630" />
  <meta property="og:image:alt" content={site.ogImageAlt} />
  <meta property="og:locale" content="en_US" />

  <meta name="twitter:card" content="summary_large_image" />
  <meta name="twitter:site" content={site.twitter} />
  <meta name="twitter:creator" content={site.twitter} />
  <meta name="twitter:title" content={fullTitle} />
  <meta name="twitter:description" content={description} />
  <meta name="twitter:image" content={image} />
  <meta name="twitter:image:alt" content={site.ogImageAlt} />
</svelte:head>
