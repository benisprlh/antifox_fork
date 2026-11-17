/** @type {import('next').NextConfig} */

// NEXT_EXPORT=1 next build emits static files into web/out for Go or nginx hosting.
// Leave unset in development to retain API proxying and hot reload.
const isExport = process.env.NEXT_EXPORT === "1";
// Vercel mock demo has no backend and needs no API proxy.
const isMock = process.env.NEXT_PUBLIC_MOCK === "1";

const nextConfig = {
  reactCompiler: true,
  // Allow LAN IP access to development resources/HMR as needed.
  // Permit IPv4 origins for /_next/* and HMR during development, including changing LAN addresses.
  // Next rejects bare '*'; segmented '*.*.*.*' matches IPv4 origins.
  allowedDevOrigins: ["*.*.*.*"],
  compiler: {
    removeConsole: process.env.NODE_ENV === "production",
  },
  ...(isExport
    ? {
        // Static export has no Node runtime or image optimization; each route emits an index.html.
        output: "export",
        images: { unoptimized: true },
        trailingSlash: true,
      }
    : isMock
      ? {
          // Mock demo has no backend or API proxy.
          images: { unoptimized: true },
        }
      : {
          // Development proxies /api/* to Go at :8787, overridable with AUTOPENTEST_API.
          async rewrites() {
            const backend = process.env.AUTOPENTEST_API ?? "http://localhost:8787";
            return [{ source: "/api/:path*", destination: `${backend}/api/:path*` }];
          },
        }),
};

export default nextConfig;
