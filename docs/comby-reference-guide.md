## Comby Usage Guide

Comby v1.8.1. Structural search/replace that understands balanced delimiters, strings, and comments. Far superior to regex for code transformations, but has specific behaviors that cause silent corruption if misused.

### Core Flags
- `-in-place` or `-i`: modify file on disk (both work, both verified)
- `-diff`: show unified diff of changes
- `-stdout`: print result to stdout
- `-stdin`: read from stdin pipe
- `-matcher .go`: force language parser (auto-detects from extension)
- `-match-only -newline-separated -stdout`: extract matches without rewriting

### File Targeting
- Positional arg: `comby 'pat' 'repl' path/to/file.go` — targets single file
- Extension: `comby 'pat' 'repl' .go` — recursive from cwd
- `-d dir`: `comby 'pat' 'repl' .go -d src/` — recursive from dir
- **WARNING**: `-d /tmp/` will scan ALL of /tmp including restricted dirs; use full file path instead

### Hole Syntax
| Syntax | Matches | Key behavior |
|---|---|---|
| `:[var]` | Everything (lazy) | **Inside delimiters**: matches within balanced group including newlines. **Outside delimiters**: stops at newline or block start |
| `:[_]` | Everything (unnamed) | Same as `:[var]` but doesn't bind — can match different content each time |
| `:[[var]]` | `\w+` | Alphanumeric + underscore only |
| `:[var.]` | Non-space + punctuation | Good for dotted paths like `a.b.c` |
| `:[var:e]` | Expression | Contiguous non-whitespace OR balanced parens/brackets |
| `:[var\n]` | Line rest | Zero or more chars up to and including newline |
| `:[ var]` | Whitespace only | Matches spaces/tabs, NOT newlines |

### THE Critical Rule: Balanced Delimiters

**`{:[body]}` is the ONLY safe way to match a Go block body.** Comby understands balanced `{}`, `()`, `[]` — the hole matches everything inside, preserving nesting.

**NEVER use the split pattern:**
```
# BROKEN — DO NOT USE
comby 'func A() {
:[body]
}' '...'
```
This puts `:[body]` OUTSIDE the delimiter pair. Effects:
1. Leading indentation of first line in body gets stripped (whitespace normalization eats the `\n\t`)
2. The `}` on the last template line may match a NESTED `}` instead of the function's closing brace
3. Content between nested `}` and true closing `}` gets silently eaten

**ALWAYS use the inline pattern:**
```
# CORRECT — always use this
comby 'func A() {:[body]}' '...'
```

### THE Critical Bug: Newline Collapse on Line Boundaries

When a match template starts with `\t` (indented content), comby's whitespace normalization treats `\n\t` in the source as equivalent to `\t` in the template. This means:

```go
// Source file:
    }
    defaults.Harness = x

// Template: '	defaults.Harness = :[v]'
// Match captures: '\n\tdefaults.Harness = x' (includes the newline before)
// Replacement: '	defaults.Harness = REPLACED'
// Result: }	defaults.Harness = REPLACED  ← LINE MERGED WITH PREVIOUS!
```

The diff confirms: two lines become one. The `}` from the previous line and `defaults.Harness` merge onto the same line.

**Workarounds:**
1. **Anchor to the previous line**: include the `}` (or whatever precedes) in your match template
2. **Use `:[_\n]` line hole**: `:[_\n]\tdefaults.Harness = :[v]` captures the line boundary
3. **Match the full surrounding block**: `if ... {:[_]}` + content after, so the boundary is inside balanced delimiters
4. **Best: match at function/block level** using `{:[body]}` and rewrite the whole body

### Whitespace Normalization Rules
- Template `\n` ≈ source `\n` ≈ source `\n\t` ≈ source `\n    ` (all treated as "some whitespace")
- Single space in template matches any amount of whitespace in source (including newlines!)
- Blank lines in template DO match blank lines in source (whitespace normalization applies)
- **Rewrite templates preserve literal newlines and indentation as written**

### Safe Patterns for Go

**Add parameter to function:**
```bash
comby 'func Foo(:[params]) :[ret] {:[body]}' \
     'func Foo(:[params], newParam Type) :[ret] {:[body]}' file.go -i
```

**Replace entire function body:**
```bash
comby 'func Foo(:[params]) :[ret] {:[_]}' \
     'func Foo(:[params]) :[ret] {
	// new body
}' file.go -i
```

**Insert function after another:**
```bash
comby 'func Existing(:[p]) :[r] {:[body]}' \
     'func Existing(:[p]) :[r] {:[body]}

func NewFunc() {
	// ...
}' file.go -i
```

**Add to import block:**
```bash
comby 'import (:[imports])' 'import (:[imports]
	"new/package"
)' file.go -i
```

**Replace specific call pattern:**
```bash
comby 'oldFunc(:[args])' 'newFunc(:[args])' file.go -i
```

**Conditional rewrite with rules:**
```bash
comby 'foo(":[arg]")' 'bar(":[arg]")' file.go -i -rule 'where :[arg] == "specific"'
```

### Variable Binding
- Same variable twice = must match identical content: `foo(:[a], :[a])` only matches `foo(x, x)`
- `:[_]` is special: `foo(:[_], :[_])` matches `foo(x, y)` — wildcard, no binding

### Shell Quoting
- Single quotes preserve everything including newlines and tabs
- For Go strings with `\033`, the shell won't interpret them inside single quotes (correct behavior — Go source has literal backslash)
- For apostrophes in comments: use `$'...'` with escaped `'\''` or double quotes

### Anti-Patterns to Avoid
1. **NEVER** match `{` and `}` on separate template lines with `:[body]` between — use `{:[body]}`
2. **NEVER** match indented content at the start of a template without anchoring to the previous line
3. **NEVER** use `:[comment\n]` to match multi-line comments — it matches WAY more than expected
4. **NEVER** use `:[params]` in rewrite without it in match — comby substitutes it literally as `:[params]`
5. **NEVER** `-d /tmp/` — permission errors on systemd private dirs. Use full file paths
6. **AVOID** replacing huge function bodies with literal text — better to use `{:[_]}` to discard and write new body

### ast-grep vs comby Decision Guide
| Task | Use |
|---|---|
| Add/change function parameters | comby — `func F(:[params])` patterns |
| Replace function body entirely | comby — `{:[_]}` to discard, write new |
| Insert code after a function | comby — match function, rewrite with appended code |
| Rename all calls to a function | ast-grep — `--pattern 'oldName($$$)' --rewrite 'newName($$$)'` |
| Find all usages of a pattern | ast-grep — `--pattern 'pattern' --lang go` |
| Add to import block | comby — `import (:[imports])` pattern |
| Multi-line structural rewrite | comby — balanced delimiter matching |
| Simple string replacement | sd — `sd 'old' 'new' file` |
