import { getSessionCookie } from 'better-auth/cookies';
import { NextRequest, NextResponse } from 'next/server';

// Optimistic redirect only: it checks that a session cookie exists. The Go
// API and server components still verify the session itself.
export function proxy(request: NextRequest) {
  if (!getSessionCookie(request)) {
    const url = new URL('/signin', request.url);
    url.searchParams.set('next', request.nextUrl.pathname);
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  matcher: ['/app/:path*'],
};
