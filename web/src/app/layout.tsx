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
  title: "kasmos - agent-driven IDE for AI pair programming",
  description:
    "a TUI-based agent-driven IDE that manages multiple AI agents (Claude Code, Codex, Aider, Gemini) in isolated workspaces, so you can work on multiple tasks simultaneously.",
  keywords: [
    "kasmos", "tui", "ai", "ide", "agent", "terminal", "tmux",
    "claude code", "codex", "aider", "pair programming",
  ],
  authors: [{ name: "kastheco" }],
  openGraph: {
    title: "kasmos",
    description:
      "a TUI-based agent-driven IDE for managing multiple AI agents in isolated workspaces",
    url: "https://kasmos.kasthe.co",
    type: "website",
    images: [{ url: "/og-image.png" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "kasmos",
    description:
      "a TUI-based agent-driven IDE for managing multiple AI agents in isolated workspaces",
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
