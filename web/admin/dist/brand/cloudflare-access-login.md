# cloudflare zero trust access login handoff

apply these settings in cloudflare zero trust → settings → custom pages → login page.
changes propagate to every access application under this organization login page —
confirm with your team before saving.

- organization name: kasmos hq
- logo url: https://<admin-host>/admin/brand/kasmos-hq-access-logo.svg
- background color: #232136
- header text: sign in to kasmos hq
- footer text: secured by cloudflare zero trust

replace `<admin-host>` with the production host that serves the embedded admin spa.
if cloudflare rejects svg, re-export the lockup as a png, commit to
`web/admin/public/brand/kasmos-hq-access-logo.png`, update `BRAND.accessLogoPublicPath`
in `web/admin/src/brand.ts`, and update the logo url above.

preview the composition locally at `/admin/access-login-preview`.
