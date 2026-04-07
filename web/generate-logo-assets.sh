#!/usr/bin/env bash
# generate-logo-assets.sh — Regenerate all derived logo assets from source PNGs.
# Run from any working directory: bash web/generate-logo-assets.sh
set -euo pipefail

# Resolve repo root relative to this script's location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

k_src="$REPO_ROOT/assets/kasmos_k.png"
full_src="$REPO_ROOT/assets/kasmos_full.png"
web_public="$REPO_ROOT/web/public"
docs_public="$REPO_ROOT/web/docs/public"

# Validate source art exists
if [[ ! -f "$k_src" ]]; then
  echo "ERROR: source art not found: $k_src" >&2
  exit 1
fi
if [[ ! -f "$full_src" ]]; then
  echo "ERROR: source art not found: $full_src" >&2
  exit 1
fi

mkdir -p "$web_public" "$docs_public"

echo "→ logo-k.png (200×200)"
magick "$k_src" -resize 200x200 -strip -define png:compression-level=9 "$web_public/logo-k.png"

echo "→ logo-full.png (1200 wide)"
magick "$full_src" -resize 1200x -strip -define png:compression-level=9 "$web_public/logo-full.png"

echo "→ favicon.ico (16/32/48)"
magick "$k_src" -background none -define icon:auto-resize=16,32,48 "$web_public/favicon.ico"

echo "→ apple-touch-icon.png (180×180)"
magick "$k_src" -resize 180x180 -strip -define png:compression-level=9 "$web_public/apple-touch-icon.png"

echo "→ icon-192.png"
magick "$k_src" -resize 192x192 -strip -define png:compression-level=9 "$web_public/icon-192.png"

echo "→ icon-512.png"
magick "$k_src" -resize 512x512 -strip -define png:compression-level=9 "$web_public/icon-512.png"

echo "→ og-image.png (1200×630)"
magick -size 1200x630 canvas:'#232136' \
  "$full_src" -resize 740x -gravity center -composite \
  -strip -define png:compression-level=9 \
  "$web_public/og-image.png"

echo "→ copying to docs/public/"
cp "$web_public/logo-k.png"  "$docs_public/logo-k.png"
cp "$web_public/favicon.ico" "$docs_public/favicon.ico"
cp "$web_public/og-image.png" "$docs_public/og-image.png"

# Validate file-size budgets
logo_k_size=$(stat -c%s "$web_public/logo-k.png")
logo_full_size=$(stat -c%s "$web_public/logo-full.png")

if [[ "$logo_k_size" -gt 51200 ]]; then
  echo "WARNING: logo-k.png is ${logo_k_size} bytes (budget: 51200). Consider reducing resize dimensions." >&2
fi
if [[ "$logo_full_size" -gt 81920 ]]; then
  echo "WARNING: logo-full.png is ${logo_full_size} bytes (budget: 81920). Consider reducing resize dimensions." >&2
fi

echo ""
echo "Generated assets:"
for f in \
  "$web_public/logo-k.png" \
  "$web_public/logo-full.png" \
  "$web_public/favicon.ico" \
  "$web_public/apple-touch-icon.png" \
  "$web_public/icon-192.png" \
  "$web_public/icon-512.png" \
  "$web_public/og-image.png" \
  "$docs_public/logo-k.png" \
  "$docs_public/favicon.ico" \
  "$docs_public/og-image.png"; do
  printf "  %-50s %s bytes\n" "${f#$REPO_ROOT/}" "$(stat -c%s "$f")"
done
echo "Done."
