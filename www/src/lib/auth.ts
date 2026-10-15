import { betterAuth } from 'better-auth';
import { nextCookies } from 'better-auth/next-js';
import { Pool } from 'pg';

export const auth = betterAuth({
  secret: process.env.BETTER_AUTH_SECRET,
  baseURL: process.env.BETTER_AUTH_URL,
  database: new Pool({
    connectionString: process.env.POSTGRES_URL,
  }),
  emailAndPassword: {
    enabled: true,
  },
  plugins: [nextCookies()],
});
