import adapter from '@sveltejs/adapter-static';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [
    tailwindcss(),
    sveltekit({
      compilerOptions: {
        // Force runes mode for the project, except for dependencies.
        runes: ({ filename }) =>
          filename.includes('node_modules') ? undefined : true,
      },
      // Built into the Go package that embeds it.
      adapter: adapter({
        pages: '../internal/webui/dist',
        assets: '../internal/webui/dist',
        fallback: 'index.html',
        precompress: false,
        strict: true,
      }),
    }),
  ],
  server: {
    // Proxy API calls to `lookfar serve` during `bun run dev`.
    proxy: { '/api': 'http://localhost:8080' },
  },
  test: {
    include: ['src/**/*.test.ts'],
  },
});
