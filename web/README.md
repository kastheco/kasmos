# web

This directory contains three separate web surfaces:

- `src/app` — the public marketing site (Next.js) at `kasmos.kasthe.co`
- `docs/` — the documentation site (Docusaurus 3, versioned docs, local search)
- `admin/` — the admin spa (Vite) that `kas serve` exposes at `/admin/`

## local development

```bash
cd web && npm ci && npm run dev
cd web/docs && npm ci && npm run dev
cd web/admin && npm ci && npm run dev
```

## builds

```bash
cd web && npm run build
cd web/docs && npm run build
cd web/admin && npm run build
```

`kas serve` does not serve `web/admin/src` directly. It serves the built `web/admin/dist` assets, embedded by `web/admin_assets.go`, unless `--admin-dir` points at a local build.
