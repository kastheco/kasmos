# ast-grep Reference

Binary: `ast-grep` (alias `sg` if configured)
Version: 0.41.0

AST-based structural code search, lint, and rewrite using tree-sitter grammars. Matches syntax structure, not text — won't accidentally rename inside strings, comments, or unrelated identifiers.

## Core Commands

```bash
# Search for pattern
ast-grep run --pattern 'PATTERN' --lang LANG [PATHS...]

# Search and rewrite
ast-grep run --pattern 'PATTERN' --rewrite 'REPLACEMENT' --lang LANG [PATHS...]

# Interactive rewrite (confirm each change)
ast-grep run --pattern 'PATTERN' --rewrite 'REPLACEMENT' --lang LANG --interactive

# Apply all rewrites without confirmation
ast-grep run --pattern 'PATTERN' --rewrite 'REPLACEMENT' --lang LANG --update-all

# Scan with rule file
ast-grep scan --rule rule.yml [PATHS...]

# JSON output for scripting
ast-grep run --pattern 'PATTERN' --lang LANG --json
```

Language is inferred from file extensions when `--lang` is omitted and paths are provided.

## Metavariable Syntax

| Syntax | Matches | Example |
|--------|---------|---------|
| `$VAR` | Single AST node | `$FUNC($A)` matches `foo(x)` |
| `$_` | Single node (anonymous, non-capturing) | `$_($A)` matches any call with one arg |
| `$$$` | Zero or more nodes (anonymous) | `foo($$$)` matches `foo()`, `foo(a)`, `foo(a, b, c)` |
| `$$$ARGS` | Zero or more nodes (named, captures all) | `foo($$$ARGS)` captures all args as a unit |
| `$$VAR` | Single node including unnamed tree-sitter nodes | `return $$A` matches `return 123` and `return;` |

**`$$$` vs `$$$ARGS`:** Use `$$$` when you want to ignore the arguments. Use `$$$ARGS` when you need to capture and replay them in the rewrite: `foo($$$ARGS)` → `bar($$$ARGS)`.

**Key behavior:** Same named metavariable used twice must match identical content. `$A == $A` matches `x == x` but not `x == y`.

## Common Patterns

### Go

```bash
# Find all calls to a function
ast-grep run -p 'fmt.Errorf($$$)' -l go

# Rename a function (captures and replays all args)
ast-grep run -p 'oldName($$$ARGS)' -r 'newName($$$ARGS)' -l go -U

# Find method calls on any receiver
ast-grep run -p '$OBJ.Close()' -l go

# Find assert calls where expected is nil
ast-grep run -p 'assert.Equal($T, nil, $$$)' -l go

# Rewrite error wrapping
ast-grep run -p 'errors.New($MSG)' -r 'fmt.Errorf($MSG)' -l go -U

# Find unused error returns
ast-grep run -p '$_, _ = $FUNC($$$)' -l go

# Find if-err blocks ignoring the error
ast-grep run -p 'if $ERR != nil { $_ }' -l go
```

### TypeScript / JavaScript

```bash
# Find all console.log calls
ast-grep run -p 'console.log($$$)' -l ts

# Replace Promise.then with async/await (find candidates)
ast-grep run -p '$PROMISE.then($$$)' -l ts

# Find React hook calls
ast-grep run -p 'useState($INIT)' -l tsx

# Rename import specifier
ast-grep run -p "import { $OLD } from '$MOD'" \
             -r "import { $NEW } from '$MOD'" -l ts -U
```

### Python

```bash
# Find all print calls (Python 2 to 3 migration)
ast-grep run -p 'print $$$' -l python

# Find assertions
ast-grep run -p 'assert $COND, $MSG' -l python

# Find function definitions with specific decorator
ast-grep run -p '@$DECORATOR
def $FUNC($$$): ...' -l python
```

## Output Options

```bash
--json              # Structured JSON output (for piping)
--json=pretty       # Pretty-printed JSON
--json=stream       # Streaming JSON (one match per line)
-A NUM              # Show NUM lines after match
-B NUM              # Show NUM lines before match
-C NUM              # Show NUM context lines around match
--heading always    # Always show filename heading
--color never       # Disable color (for piping)
```

## File Targeting

```bash
ast-grep run -p 'PATTERN' -l go                    # all Go files in cwd
ast-grep run -p 'PATTERN' -l go src/ internal/      # specific directories
ast-grep run -p 'PATTERN' --globs '!*_test.go' -l go  # exclude test files
ast-grep run -p 'PATTERN' --globs '**/*.tsx' -l tsx    # glob filter
```

## Strictness Levels

Control how precisely the pattern must match the AST:

| Level | Behavior |
|-------|----------|
| `cst` | Exact match including all trivial nodes |
| `smart` (default) | Match all except trivial source nodes |
| `ast` | Match only named AST nodes |
| `relaxed` | Like `ast` but ignores comments |
| `signature` | Like `relaxed` but ignores text content |

```bash
ast-grep run -p 'PATTERN' --strictness relaxed -l go
```

## Rule Files (YAML)

For complex matching with constraints and relational checks, use rule files. The `rule` combinator supports `pattern`, `any`, `all`, `not`, `inside`, `has`, `follows`, `precedes`, and `constraints`.

```yaml
# rule.yml — basic lint rule
id: no-console-log
language: typescript
rule:
  pattern: console.log($$$)
message: Remove console.log before committing
severity: warning
```

```yaml
# rule.yml — with constraints (restrict metavariable content)
id: no-empty-error-message
language: go
rule:
  pattern: errors.New($MSG)
  constraints:
    MSG:
      regex: '^""$'     # only match empty string literals
message: errors.New("") creates an unhelpful error
severity: error
```

```yaml
# rule.yml — using 'has' (structural containment)
id: if-err-unused
language: go
rule:
  pattern: if $ERR != nil {$$$}
  not:
    has:
      pattern: return $ERR
message: Error is checked but not returned or wrapped
severity: warning
```

```yaml
# rule.yml — using 'inside' (match node inside a context)
id: no-panic-in-handler
language: go
rule:
  pattern: panic($$$)
  inside:
    pattern: func $NAME(w http.ResponseWriter, r *http.Request) {$$$}
message: panic in HTTP handler will crash the server
severity: error
```

```bash
ast-grep scan --rule rule.yml
ast-grep scan --rule rule.yml src/   # specific path
```

## Debugging Patterns

```bash
# Print the AST of a pattern to understand matching
ast-grep run -p 'your_pattern' --debug-query=ast -l go

# Print CST (includes unnamed nodes)
ast-grep run -p 'your_pattern' --debug-query=cst -l go
```

When a pattern doesn't match what you expect: use `--debug-query=ast` to see the tree structure, then align your metavariables to AST node boundaries.

## Supported Languages

Go, TypeScript, JavaScript, Python, Rust, C, C++, Java, Kotlin, Swift, Ruby, Lua, and many more. Full list: `ast-grep run --help` or https://ast-grep.github.io/reference/languages.html
