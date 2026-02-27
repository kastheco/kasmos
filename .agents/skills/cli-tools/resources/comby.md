# comby Reference

Binary: `comby`
Version: 1.8.1

Structural search/replace that understands balanced delimiters, strings, and comments. Superior to regex for code transformations, but has specific behaviors that cause **silent corruption if misused**.

## Core Flags

| Flag | Effect |
|------|--------|
| `-in-place` or `-i` | Modify files on disk (both work) |
| `-diff` | Show unified diff of changes |
| `-stdout` | Print result to stdout |
| `-stdin` | Read from stdin pipe |
| `-matcher .go` | Force language parser (auto-detects from extension) |
| `-match-only -newline-separated -stdout` | Extract matches without rewriting |

**Critical:** Without `-in-place`, comby only previews — no changes are written.

## File Targeting

```bash
comby 'pat' 'repl' path/to/file.go       # single file
comby 'pat' 'repl' .go                    # recursive from cwd, all .go files
comby 'pat' 'repl' .go -d src/            # recursive from specific directory
```

**Warning:** `-d /tmp/` scans ALL of /tmp including restricted dirs. Use full file paths instead.

## Hole Syntax

| Syntax | Matches | When to Use |
|--------|---------|-------------|
| `:[var]` | Everything (lazy) | Inside delimiters: matches within balanced group incl. newlines. Outside: stops at newline or block start |
| `:[_]` | Everything (unnamed) | Wildcard discard — use when you don't need to replay the match |
| `:[[var]]` | `\w+` only | Alphanumeric + underscore — use for identifiers, type names |
| `:[var.]` | Non-space + punctuation | Dotted paths like `a.b.c`, no whitespace |
| `:[var:e]` | Expression | Contiguous non-whitespace OR balanced parens/brackets — use for single expressions like `x + 1`, `f(a, b)` |
| `:[var\n]` | Line rest | Zero or more chars up to and including newline — use to anchor to line boundaries |
| `:[ var]` | Whitespace only | Spaces/tabs, NOT newlines — use to match indentation |

### `:[var:e]` — When to Use vs `:[var]`

`:[var:e]` is for **single expressions** where you want to avoid matching across multiple arguments or statements:

```bash
# :[var] would match "a, b, c" (too greedy for expressions)
# :[var:e] matches just "a" in foo(a, b, c)
comby 'return :[result:e]' 'return wrap(:[result:e])' file.go -i

# Use :[var:e] when the hole should match one expression, not a whole argument list
comby 'assert(:[actual:e], :[expected:e])' \
      'assert(:[expected:e], :[actual:e])' file.go -i
```

### Variable Binding

- Same variable twice = must match identical content: `foo(:[a], :[a])` only matches `foo(x, x)`
- `:[_]` is special wildcard: `foo(:[_], :[_])` matches `foo(x, y)` — no binding

---

## ⛔ CRITICAL: Balanced Delimiters — Read Before Writing Any Pattern

**`{:[body]}` is the ONLY safe way to match a block body.** Comby understands balanced `{}`, `()`, `[]` — the hole matches everything inside, preserving nesting.

### NEVER split braces across lines

```bash
# BROKEN — DO NOT USE
comby 'func A() {
:[body]
}' '...'
```

This puts `:[body]` OUTSIDE the delimiter pair. Effects:
1. Leading indentation of first line in body gets stripped
2. The `}` may match a NESTED `}` instead of the function's closing brace
3. Content between nested `}` and true closing `}` gets **silently eaten** — data loss with no warning

### ALWAYS use inline braces

```bash
# CORRECT — always use this form
comby 'func A() {:[body]}' '...'
```

---

## ⛔ CRITICAL: Newline Collapse on Line Boundaries

When a match template starts with indented content (tab/spaces), comby's whitespace normalization treats `\n\t` in source as equivalent to `\t` in template. This **merges adjacent lines**:

```
Source:     }
            defaults.Harness = x

Template:   '	defaults.Harness = :[v]'
Result:     }	defaults.Harness = REPLACED   ← TWO LINES MERGED INTO ONE
```

### Workarounds

1. **Anchor to previous line:** Include the `}` (or whatever precedes) in your match template
2. **Use `:[_\n]` line hole:** `:[_\n]\tdefaults.Harness = :[v]` captures the boundary
3. **Match full surrounding block:** `if ... {:[_]}` + content after
4. **Best: match at function/block level** using `{:[body]}` and rewrite the whole body

---

## Whitespace Normalization Rules

- Template `\n` ≈ source `\n` ≈ source `\n\t` ≈ source `\n    ` (all treated as "some whitespace")
- Single space in template matches any amount of whitespace in source (including newlines)
- Blank lines in template DO match blank lines in source
- **Rewrite templates preserve literal newlines and indentation as written**

## Safe Patterns for Go

### Add parameter to function

```bash
comby 'func Foo(:[params]) :[ret] {:[body]}' \
     'func Foo(:[params], newParam Type) :[ret] {:[body]}' file.go -i
```

### Remove a parameter from a function

```bash
# Remove second parameter: func Foo(a Type, b OtherType) → func Foo(a Type)
comby 'func Foo(:[[first]] :[[firstType]], :[[second]] :[[secondType]]) :[ret] {:[body]}' \
     'func Foo(:[first] :[firstType]) :[ret] {:[body]}' file.go -i
```

### Replace entire function body

```bash
comby 'func Foo(:[params]) :[ret] {:[_]}' \
     'func Foo(:[params]) :[ret] {
	// new body here
}' file.go -i
```

### Wrap function body in an if-check

```bash
comby 'func Foo(:[params]) :[ret] {:[body]}' \
     'func Foo(:[params]) :[ret] {
	if !initialized {
		return
	}
	:[body]
}' file.go -i
```

### Insert function after another

```bash
comby 'func Existing(:[p]) :[r] {:[body]}' \
     'func Existing(:[p]) :[r] {:[body]}

func NewFunc() {
	// ...
}' file.go -i
```

### Add to import block

```bash
comby 'import (:[imports])' 'import (:[imports]
	"new/package"
)' file.go -i
```

### Replace specific call pattern

```bash
comby 'oldFunc(:[args])' 'newFunc(:[args])' file.go -i
```

### Conditional rewrite with rules

```bash
comby 'foo(":[arg]")' 'bar(":[arg]")' file.go -i -rule 'where :[arg] == "specific"'
```

## Anti-Patterns

| Anti-Pattern | Why It Breaks |
|-------------|---------------|
| Split `{` and `}` on separate lines with `:[body]` between | Hole is outside delimiter pair — silent data loss, wrong `}` matched |
| Match indented content at start of template without anchor | Newline collapse merges adjacent lines silently |
| Use `:[comment\n]` for multi-line comments | Matches far more than expected — use `:[_]` inside `/* */` instead |
| Use `:[params]` in rewrite without it in match | Comby substitutes it literally as `:[params]` text |
| `-d /tmp/` | Permission errors on systemd private dirs |
| Replace huge function bodies with literal text | Better to use `{:[_]}` to discard and write new body inline |
| Use `:[var]` when you mean a single expression | `:[var]` is greedy inside delimiters — use `:[var:e]` for expressions |
| Forget `-in-place` | Without it, comby only prints to stdout — no files changed |

## Shell Quoting

Single-quote your patterns. This preserves literal newlines, tabs, and backslashes:

```bash
comby 'func Foo(:[p]) {:[b]}' 'func Foo(:[p]) {:[b]}' file.go -i
```

If your pattern contains a literal single quote (e.g., in a comment), use `$'...'` syntax:
```bash
comby $'// it\'s done' '// done' file.go -i
```

Shell quoting is mostly a non-issue for code patterns — single quotes work in >95% of cases.
