import type { auth } from '#lib/server/auth.ts';

type Session = typeof auth.$Infer.Session;

declare global {
  namespace App {
    interface Locals {
      session?: Session['session'];
      user?: Session['user'];
    }
  }
}
