#!/bin/bash
set -euo pipefail
cat '/home/kas/dev/kasmos/kitty-specs/004-workflow-cheatsheet/.kasmos/prompts/WP01.md' | opencode -p 'context:'
