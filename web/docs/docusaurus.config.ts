import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";

const config: Config = {
  title: "kasmos docs",
  tagline:
    "documentation for kasmos — the tui, admin spa, shared http mcp endpoint, daemon services, and global task store behind multi-agent orchestration.",
  url: "https://kasmos.kasthe.co",
  baseUrl: "/docs/",
  trailingSlash: true,
  favicon: "img/favicon.ico",
  onBrokenLinks: "warn",
  onBrokenMarkdownLinks: "warn",
  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },
  themes: [
    "@docusaurus/theme-mermaid",
    [
      require.resolve("@easyops-cn/docusaurus-search-local"),
      {
        hashed: true,
        indexBlog: false,
        docsRouteBasePath: "/",
      },
    ],
  ],
  markdown: { mermaid: true },
  presets: [
    [
      "classic",
      {
        blog: false,
        docs: {
          routeBasePath: "/",
          sidebarPath: "./sidebars.ts",
          lastVersion: "current",
          versions: {
            current: {
              label: "latest",
              path: "",
            },
          },
          editUrl: ({ versionDocsDirPath, docPath }) =>
            `https://github.com/kastheco/kasmos/edit/main/web/docs/${versionDocsDirPath}/${docPath}`,
        },
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],
  themeConfig: {
    navbar: {
      logo: { alt: "kasmos", src: "img/logo-full.png" },
      items: [
        { to: "/", label: "docs", position: "left" },
        {
          type: "docsVersionDropdown",
          position: "left",
          dropdownActiveClassDisabled: true,
        },
        {
          href: "https://github.com/kastheco/kasmos",
          label: "github",
          position: "right",
        },
      ],
    },
    mermaid: {
      theme: { light: "neutral", dark: "dark" },
      options: {
        themeVariables: {
          background: "#232136",
          primaryColor: "#c4a7e7",
          primaryTextColor: "#e0def4",
          lineColor: "#9ccfd8",
        },
      },
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
