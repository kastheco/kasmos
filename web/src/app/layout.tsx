import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  userScalable: true,
  themeColor: "#232136",
};

export const metadata: Metadata = {
  metadataBase: new URL("https://kasmos.kasthe.co"),
  title: "kasmos - mcp-first multi-agent orchestration",
  description:
    "mcp-first multi-agent orchestration with a tui, admin spa, shared http mcp endpoint, daemon services, and a global task store.",
  keywords: [
    "kasmos", "tui", "admin ui", "mcp", "task store", "daemon",
    "agent", "terminal", "tmux", "claude code", "codex", "aider",
  ],
  authors: [{ name: "kastheco" }],
  openGraph: {
    title: "kasmos",
    description:
      "mcp-first multi-agent orchestration with a tui, admin spa, shared http mcp endpoint, daemon services, and a global task store",
    url: "https://kasmos.kasthe.co",
    type: "website",
    images: [{ url: "/og-image.png" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "kasmos",
    description:
      "mcp-first multi-agent orchestration with a tui, admin spa, shared http mcp endpoint, daemon services, and a global task store",
    images: ["/og-image.png"],
  },
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/icon-192.png", sizes: "192x192", type: "image/png" },
      { url: "/icon-512.png", sizes: "512x512", type: "image/png" },
    ],
    apple: [
      { url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" },
    ],
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${geistSans.variable} ${geistMono.variable}`}>
        {children}
      </body>
    </html>
  );
}
