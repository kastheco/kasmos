#!/bin/bash
set -u

if ! command -v kas >/dev/null 2>&1; then
    echo "WARN: kas is not on PATH; leaving kasmos-panel inert" >&2
    exit 0
fi

if ! kas monitor bundle --out "$STAGE_DIR/kasmos-monitor"; then
    echo "WARN: kas monitor bundle export failed; leaving kasmos-panel inert" >&2
fi

exit 0
