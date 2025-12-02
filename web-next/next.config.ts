import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'export',
  trailingSlash: true,
  // When deployed behind Go server, API calls will be proxied
  // No need to configure rewrites for static export
};

export default nextConfig;
