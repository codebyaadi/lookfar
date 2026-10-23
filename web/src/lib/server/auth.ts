import {
  ALLOW_SIGNUP,
  BETTER_AUTH_SECRET,
  BETTER_AUTH_URL,
  DATABASE_URL,
} from '$app/env/private';
import { getRequestEvent } from '$app/server';
import { betterAuth } from 'better-auth';
import { APIError } from 'better-auth/api';
import { sveltekitCookies } from 'better-auth/svelte-kit';
import { Pool } from 'pg';

const pool = new Pool({ connectionString: DATABASE_URL });

export const auth = betterAuth({
  secret: BETTER_AUTH_SECRET,
  baseURL: BETTER_AUTH_URL,
  database: pool,
  emailAndPassword: { enabled: true },
  databaseHooks: {
    user: {
      create: {
        // Invite-only: the first account becomes the admin, then sign-up
        // closes unless ALLOW_SIGNUP=true. Invites replace this later.
        before: async () => {
          if (ALLOW_SIGNUP) return;
          const { rows } = await pool.query('SELECT 1 FROM "user" LIMIT 1');
          if (rows.length > 0) {
            throw new APIError('FORBIDDEN', {
              message: 'Sign-up is closed on this instance',
            });
          }
        },
      },
    },
  },
  plugins: [sveltekitCookies(getRequestEvent)],
});
