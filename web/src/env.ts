import { defineEnvVars } from '@sveltejs/kit/env';

export const variables = defineEnvVars({
  DATABASE_URL: {
    description: 'Postgres connection string shared with the Go API',
  },
  BETTER_AUTH_SECRET: {
    description: 'Must match the Go API BETTER_AUTH_SECRET',
  },
  BETTER_AUTH_URL: { description: 'Public origin of this app' },
  API_URL: {
    description: 'Base URL of the Go API',
    schema: (value) => value || 'http://localhost:8080',
  },
  ALLOW_SIGNUP: {
    description: 'Keep sign-up open after the first account exists',
    schema: (value) => value === 'true',
  },
});
