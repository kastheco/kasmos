import type { Metadata, Viewport } from "next";
import { Footer, Layout, Navbar } from "nextra-theme-docs";
import { Head } from "nextra/components";
import { getPageMap } from "nextra/page-map";
import "nextra-theme-docs/style.css";
import "./globals.css";

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  userScalable: true,
  themeColor: "#232136",
};

export const metadata: Metadata = {
  metadataBase: new URL("https://kasmos.kasthe.co"),
  title: "kasmos docs",
  description:
    "documentation for kasmos — the tui, admin spa, shared http mcp endpoint, daemon services, and global task store behind multi-agent orchestration.",
  keywords: [
    "kasmos",
    "kas",
    "tui",
    "admin ui",
    "mcp",
    "task store",
    "docs",
    "agent",
    "orchestration",
    "daemon",
    "cli",
    "terminal",
  ],
  authors: [{ name: "kastheco" }],
  icons: {
    icon: "/docs/favicon.ico",
  },
  openGraph: {
    title: "kasmos docs",
    description:
      "documentation for kasmos — tui, admin spa, shared http mcp endpoint, daemon services, and global task store",
    url: "https://kasmos.kasthe.co/docs",
    type: "website",
    images: [{ url: "/docs/og-image.png" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "kasmos docs",
    description:
      "documentation for kasmos — tui, admin spa, shared http mcp endpoint, daemon services, and global task store",
    images: ["/docs/og-image.png"],
  },
};

const navbar = (
  <Navbar
    logo={
      <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
        <img src="/docs/logo-k.png" alt="" aria-hidden="true" width={24} height={24} />
        <b>kasmos docs</b>
      </span>
    }
    projectLink="https://github.com/kastheco/kasmos"
  />
);

const footer = (
  <Footer>BSL 1.1 {new Date().getFullYear()} © kastheco.</Footer>
);

export default async function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <Head
        color={{
          hue: 265,
          saturation: 60,
          lightness: { dark: 72, light: 40 },
        }}
      />
      <body>
        <Layout
          navbar={navbar}
          pageMap={await getPageMap()}
          sidebar={{
            defaultMenuCollapseLevel: 1,
            autoCollapse: true,
          }}
          docsRepositoryBase="https://github.com/kastheco/kasmos/blob/main/web/docs/src/content"
          footer={footer}
        >
          {children}
        </Layout>
      </body>
    </html>
  );
}
