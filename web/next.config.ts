import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  output: 'standalone',
  experimental: {
    typedEnv: true,
  },
  turbopack: {
    root: process.cwd(),
  },
}

export default nextConfig
