import { site } from '#lib/site.ts';
import type { RequestHandler } from './$types';

export const prerender = true;

// Only public, indexable pages belong here. The app and auth pages are noindex.
const paths = ['/'];

export const GET: RequestHandler = () => {
  const urls = paths
    .map(
      (p) =>
        `  <url>\n    <loc>${site.url}${p}</loc>\n    <changefreq>weekly</changefreq>\n    <priority>${p === '/' ? '1.0' : '0.5'}</priority>\n  </url>`
    )
    .join('\n');

  return new Response(
    `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`,
    { headers: { 'Content-Type': 'application/xml' } }
  );
};
