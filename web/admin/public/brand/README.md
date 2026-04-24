# kasmos hq brand assets

The product name is `kasmos hq` for the admin SPA only; it does not rename the marketing site or docs.

The preferred source is a separate transparent `hq` suffix PNG or SVG that composes with the existing `web/admin/src/assets/logo-full.png` wordmark.

If no `hq` source is supplied, use the committed SVG fallback in this directory.

The Cloudflare Access logo is served from this directory at `/admin/brand/<file>` as a stable, unhashed public asset. Never reference Vite-hashed `/admin/assets/*` paths from Cloudflare.

To replace the fallback, drop a new file here, update `BRAND.accessLogoPublicPath` in `web/admin/src/brand.ts`, and update the logo URL in `web/admin/public/brand/cloudflare-access-login.md`.
