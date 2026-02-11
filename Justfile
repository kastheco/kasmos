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

# Install kasmos binary to ~/.cargo/bin
install:
  cargo install --path crates/kasmos --force

# Build
build:
  cargo build

# Run (pass-through args)
run +ARGS:
  cargo run -p kasmos -- {{ARGS}}

# Launch orchestration by feature path or prefix (e.g. 001)
launch feature mode="continuous":
  cargo run -p kasmos -- launch {{feature}} --mode {{mode}}

# Show orchestration status (current dir if feature omitted)
status feature="":
  if [ -n "{{feature}}" ]; then cargo run -p kasmos -- status {{feature}}; else cargo run -p kasmos -- status; fi

# Attach to running orchestration by feature path or prefix (e.g. 001)
attach feature:
  cargo run -p kasmos -- attach {{feature}}

# Stop orchestration (current dir if feature omitted)
stop feature="":
  if [ -n "{{feature}}" ]; then cargo run -p kasmos -- stop {{feature}}; else cargo run -p kasmos -- stop; fi

# Test
test:
  cargo test

# Lint
lint:
  cargo clippy --all-targets --all-features -- -D warnings
