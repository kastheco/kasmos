---
work_package_id: "WP05"
title: "Upgrade nix 0.31"
lane: "planned"
dependencies: []
subtasks: ["T016", "T017", "T018", "T019"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP05: Upgrade nix 0.31 (Syscall/FIFO)

**Priority**: P2 | **Risk**: Low
**User Story**: US4 — File System Operations After Nix Upgrade
**Implements**: FR-006

**Implementation command**:
```bash
spec-kitty implement WP05
```

## Objective

Upgrade nix from 0.29 to 0.31, migrating through the I/O safety changes introduced in nix 0.30. The primary code change is in `cmd.rs` where `open()` now returns `OwnedFd` instead of `RawFd`. This migration actually improves code safety by removing an `unsafe` block.

## Context

kasmos uses nix in two files:
- `crates/kasmos/src/cmd.rs:139-169` — `send_fifo_command()` opens a FIFO pipe with `nix::fcntl::open()` and converts the fd to a `std::fs::File`. Currently uses `unsafe { File::from_raw_fd(fd) }`.
- `crates/kasmos/src/commands.rs:74-75` — `mkfifo(path, Mode)` creates FIFO pipes. API unchanged in nix 0.31.

**Key research findings**:
- nix 0.30 adopted Rust I/O safety: `open()` returns `OwnedFd` instead of `RawFd`
- `OwnedFd` implements `Into<File>` via `File::from(owned_fd)` — safe, no `unsafe` needed
- `mkfifo` doesn't use file descriptors, so it's completely unaffected
- `PollFd` lost `Copy` in 0.31 — not used by kasmos
- `SigHandler` lost `Eq`/`PartialEq` in 0.31 — not used by kasmos

## Subtasks

### T016: Update Cargo.toml

**Purpose**: Bump nix version constraint.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Change `nix = { version = "0.29", features = ["fs"] }` to `nix = { version = "0.31", features = ["fs"] }`

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: File saved with correct version string.

---

### T017: Migrate send_fifo_command() to use OwnedFd

**Purpose**: Replace the `unsafe` RawFd conversion with safe OwnedFd conversion.

**Steps**:
1. Open `crates/kasmos/src/cmd.rs`
2. In the `send_fifo_command()` function (starting at line 139):

   **Remove** line 143:
   ```rust
   use std::os::fd::FromRawFd;
   ```

   **Replace** line 162:
   ```rust
   let mut file = unsafe { std::fs::File::from_raw_fd(fd) };
   ```
   **With**:
   ```rust
   let mut file = std::fs::File::from(fd);
   ```

   This works because `nix::fcntl::open()` now returns `OwnedFd`, and `std::fs::File` implements `From<OwnedFd>`. The `OwnedFd` type owns the file descriptor and will close it when dropped, which is exactly what `File::from_raw_fd` was doing — but without the `unsafe`.

3. The rest of the function (`write_all`, `flush`, error handling) is unchanged.
4. The `open()` call itself (lines 145-160) is unchanged — the only difference is that `fd` is now `OwnedFd` instead of `RawFd`, but the match arms and error handling are the same.

**Files**: `crates/kasmos/src/cmd.rs`

**Validation**: `cargo check` passes. The `unsafe` block is removed.

**Before** (lines 139-169):
```rust
fn send_fifo_command(fifo_path: &Path, command: &str) -> Result<()> {
    use nix::errno::Errno;
    use nix::fcntl::{open, OFlag};
    use nix::sys::stat::Mode;
    use std::os::fd::FromRawFd;           // ← REMOVE

    let fd = match open(...) { ... };

    let mut file = unsafe { std::fs::File::from_raw_fd(fd) };  // ← CHANGE
    ...
}
```

**After**:
```rust
fn send_fifo_command(fifo_path: &Path, command: &str) -> Result<()> {
    use nix::errno::Errno;
    use nix::fcntl::{open, OFlag};
    use nix::sys::stat::Mode;
    // FromRawFd import removed — no longer needed

    let fd = match open(...) { ... };     // fd is now OwnedFd

    let mut file = std::fs::File::from(fd);  // Safe conversion
    ...
}
```

---

### T018: Verify mkfifo is unaffected

**Purpose**: Confirm FIFO creation code compiles without changes.

**Steps**:
1. Check `crates/kasmos/src/commands.rs:74-75` — the `mkfifo` call takes a `&Path` and `Mode`, which is unchanged in nix 0.31
2. Run `cargo check` and confirm `commands.rs` compiles without modification

**Files**: `crates/kasmos/src/commands.rs`

**Validation**: No changes needed in `commands.rs`.

---

### T019: Full validation

**Purpose**: Run the complete validation suite.

**Steps**:
1. `cargo build` — must succeed with zero errors
2. `cargo test` — must pass with zero new failures
3. `cargo clippy` — must produce zero new warnings
4. Verify no remaining `unsafe` blocks in `cmd.rs` related to fd handling (the `from_raw_fd` unsafe should be gone)

**Validation**: All three commands pass clean. `unsafe` removed from FIFO code.

## Definition of Done

- [ ] `crates/kasmos/Cargo.toml` has `nix = { version = "0.31", features = ["fs"] }`
- [ ] `cmd.rs` uses `File::from(fd)` instead of `unsafe { File::from_raw_fd(fd) }`
- [ ] `use std::os::fd::FromRawFd` import removed from `cmd.rs`
- [ ] `commands.rs` compiles unchanged (mkfifo unaffected)
- [ ] `cargo build` succeeds
- [ ] `cargo test` passes
- [ ] `cargo clippy` clean

## Risks

- **Low risk**. The transformation is well-understood: `OwnedFd` → `File::from()` is a standard Rust I/O safety pattern. The net result is safer code.
- **Edge case**: If `open()` returns a different error type, the `match` arms in `send_fifo_command` might need adjustment. Research indicates `Errno` handling is unchanged.

## Reviewer Guidance

1. Verify the `unsafe` block is removed — this is a safety improvement
2. Verify `FromRawFd` import is removed (dead import would cause clippy warning)
3. Verify `commands.rs` has NO changes (mkfifo API stable)
4. Check that the FIFO open/write/flush logic is otherwise unchanged
