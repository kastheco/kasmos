import nextra from "nextra";
import type { NextConfig } from "next";

const withNextra = nextra({});

const nextConfig: NextConfig = {
  output: "export",
  trailingSlash: true,
  basePath: "/docs",
  images: {
    unoptimized: true,
  },
  eslint: {
    ignoreDuringBuilds: true,
  },
  turbopack: {
    resolveAlias: {
      "next-mdx-import-source-file": "./mdx-components.tsx",
    },
  },
};

export default withNextra(nextConfig);
