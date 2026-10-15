import type { NextConfig } from 'next';

const apiUrl = process.env.API_URL ?? 'http://localhost:8080';

const nextConfig: NextConfig = {
  // Browser calls /api/v1/* on the same origin so the better-auth cookie is
  // sent along; Next forwards them to the Go API.
  async rewrites() {
    return [{ source: '/api/v1/:path*', destination: `${apiUrl}/v1/:path*` }];
  },
  images: {
    remotePatterns: [
      { protocol: 'https', hostname: 'images.unsplash.com', pathname: '/**' },
      { protocol: 'https', hostname: 'res.cloudinary.com', pathname: '/**' },
    ],
  },
};

export default nextConfig;
