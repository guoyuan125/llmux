import type { NextConfig } from "next";

const isDev = process.env.NODE_ENV === "development";

const nextConfig: NextConfig = {
  // Static export only for production build
  ...(isDev ? {} : { output: "export" }),
  trailingSlash: true,
  images: {
    unoptimized: true,
  },
  async rewrites() {
    // Dev proxy: forward API calls to Go backend
    if (!isDev) return [];
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
      {
        source: "/v1/:path*",
        destination: "http://localhost:8080/v1/:path*",
      },
    ];
  },
};

export default nextConfig;
