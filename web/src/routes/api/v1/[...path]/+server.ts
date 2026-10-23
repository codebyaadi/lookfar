import { API_URL } from '$app/env/private';
import type { RequestHandler } from './$types';

// BFF proxy: forwards /api/v1/* to the Go API with the browser's cookies.
const handler: RequestHandler = async ({ request, params, url }) => {
  const headers = new Headers();
  for (const name of ['cookie', 'content-type', 'accept', 'authorization']) {
    const v = request.headers.get(name);
    if (v) headers.set(name, v);
  }
  const hasBody = !['GET', 'HEAD'].includes(request.method);
  const res = await fetch(`${API_URL}/v1/${params.path}${url.search}`, {
    method: request.method,
    headers,
    body: hasBody ? await request.arrayBuffer() : undefined,
    redirect: 'manual',
  });
  return new Response(res.body, {
    status: res.status,
    headers: {
      'content-type': res.headers.get('content-type') ?? 'application/json',
    },
  });
};

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
