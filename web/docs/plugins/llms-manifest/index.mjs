import { emitManifest } from "./emit.mjs";

export default function llmsManifestPlugin(context, _options) {
  return {
    name: "llms-manifest",
    async postBuild({ outDir, siteConfig }) {
      await emitManifest({
        outDir,
        docsDir: "docs",
        versionedDocsDir: "versioned_docs",
        siteDir: context.siteDir,
        baseUrl: siteConfig.url + siteConfig.baseUrl,
      });
    },
  };
}
