set shell := ["bash", "-cu"]
set dotenv-load := true

# Spec Kitty manual swarm lifecycle
# Usage: just swarm <prefix> [flags...]
#   just swarm 001                  # start next wave
#   just swarm 003 --status         # show board + waves
#   just swarm 001 --cleanup        # start with orphan cleanup
#   just swarm 001 --review WP02
#   just swarm 001 --done WP01
#   just swarm 001 --reject WP02 --feedback /tmp/fb.md
#   just swarm 001 --dry-run
swarm +ARGS:
  @scripts/sk-start.sh {{ARGS}}

# Build
build:
  cargo build

# Run
run:
  cargo run -p kasmos

# Test
test:
  cargo test

# Lint
lint:
  cargo clippy --all-targets --all-features -- -D warnings
