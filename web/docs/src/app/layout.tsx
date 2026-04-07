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
  title: "kas docs",
  description:
    "Documentation for kasmos — a TUI-based orchestration platform for managing AI agents, wave-based tasks, headless execution, daemon workflows, and the kas CLI.",
  keywords: [
    "kasmos",
    "kas",
    "tui",
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
    title: "kas docs",
    description:
      "Documentation for kasmos — TUI orchestration, headless execution, wave-based workflows, and CLI reference",
    url: "https://kasmos.kasthe.co/docs",
    type: "website",
    images: [{ url: "/docs/og-image.png" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "kas docs",
    description:
      "Documentation for kasmos — TUI orchestration, headless execution, wave-based workflows, and CLI reference",
    images: ["/docs/og-image.png"],
  },
};

const navbar = (
  <Navbar
    logo={
      <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
        <img src="/docs/logo-k.png" alt="" aria-hidden="true" width={24} height={24} />
        <b>kas docs</b>
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
          docsRepositoryBase="https://github.com/kastheco/kasmos/blob/main/web/docs/src/content"
          footer={footer}
        >
          {children}
        </Layout>
      </body>
    </html>
  );
}
