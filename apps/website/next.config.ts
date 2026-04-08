import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  env: {
    NEXT_PUBLIC_DAPP_URL:
      process.env.NEXT_PUBLIC_DAPP_URL ??
      (process.env.NETLIFY ? "https://nesterdapp.netlify.app" : "http://localhost:3001"),
  },
};

export default nextConfig;
