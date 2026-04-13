package binpath

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kastheco/kasmos/internal/mcpclient"
)

// TransportKind describes how a kasmos MCP entry is wired.
type TransportKind string

const (
	// TransportStdio means the entry launches a local stdio subprocess.
	TransportStdio TransportKind = "stdio"
	// TransportSharedHTTP means the entry points at the shared HTTP MCP endpoint.
	TransportSharedHTTP TransportKind = "shared-http"
)

// ExpectedSharedHTTPURL is the well-known URL of the shared kasmos HTTP MCP
// endpoint. An http/remote entry only qualifies as TransportSharedHTTP when
// its url matches this value exactly. Sourced from mcpclient so binpath and
// probe code can never drift apart.
const ExpectedSharedHTTPURL = mcpclient.SharedEndpointURL

// Reference describes a single discovered kas binary path in a config or service file.
type Reference struct {
	// File is the short name of the source file (e.g. ".mcp.json", "kasmos.service").
	File string
	// Label identifies which field inside the file was parsed (e.g. "mcpServers.kasmos", "ExecStart").
	Label string
	// RawPath is the path as written in the file, before any expansion.
	// For shared-http entries this is the URL rather than a binary path.
	RawPath string
	// Normalized is the canonical absolute path after symlink resolution.
	// Empty when RawPath is a bare name, placeholder, or transport URL.
	Normalized string
	// Note carries a human-readable explanation when the path is unhealthy or not installed.
	Note string
	// Transport identifies how this entry communicates with kasmos.
	// Empty for service-file references that don't use a transport.
	Transport TransportKind
}

// InspectProjectFiles inspects kas MCP config files inside repoDir and returns
// a Reference for each discovered kasmos entry. Files that do not exist are
// silently skipped.
func InspectProjectFiles(repoDir string) []Reference {
	var refs []Reference

	// .mcp.json — Claude/MCP stdio config
	if r, ok := inspectMCPJSON(filepath.Join(repoDir, ".mcp.json")); ok {
		refs = append(refs, r)
	}

	// opencode config — follow the same search order as opencodesession.ProjectConfigPath
	ocCandidates := []string{
		filepath.Join(repoDir, "opencode.jsonc"),
		filepath.Join(repoDir, "opencode.json"),
		filepath.Join(repoDir, ".opencode", "opencode.jsonc"),
		filepath.Join(repoDir, ".opencode", "opencode.json"),
	}
	for _, p := range ocCandidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			if r, ok := inspectOpencodeConfig(p); ok {
				refs = append(refs, r)
			}
			break // use only the first match
		}
	}

	return refs
}

// InspectServiceFiles inspects installed service units for home on the given OS.
// Missing optional service files are included as References with an empty
// Normalized path and a "not installed" Note, so callers can render them
// without counting them as unhealthy.
func InspectServiceFiles(home, goos string) []Reference {
	switch goos {
	case "linux":
		return inspectLinuxServices(home)
	case "darwin":
		return inspectDarwinPlists(home)
	default:
		return nil
	}
}

// ── .mcp.json ────────────────────────────────────────────────────────────────

type mcpConfigFile struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

type mcpServerEntry struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	Command string `json:"command"`
}

func inspectMCPJSON(path string) (Reference, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Reference{}, false
	}

	var cfg mcpConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Reference{}, false
	}

	raw, ok := cfg.MCPServers["kasmos"]
	if !ok {
		return Reference{}, false
	}

	var entry mcpServerEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return Reference{}, false
	}

	ref := Reference{
		File:  ".mcp.json",
		Label: "mcpServers.kasmos",
	}

	if entry.Type == "http" || (entry.Command == "" && entry.URL != "") {
		ref.RawPath = entry.URL
		if entry.URL == ExpectedSharedHTTPURL {
			ref.Transport = TransportSharedHTTP
		} else {
			// Arbitrary http url — do not label as shared http and do not
			// count as healthy. Transport stays empty so the renderer falls
			// through to the unhealthy path.
			ref.Note = "unexpected http url: expected " + ExpectedSharedHTTPURL
		}
		return ref, true
	}

	ref.RawPath = entry.Command
	ref.Normalized, ref.Note = normalizePath(entry.Command)
	return ref, true
}

// ── opencode.jsonc ────────────────────────────────────────────────────────────

type opencodeConfigFile struct {
	MCP map[string]json.RawMessage `json:"mcp"`
}

type opencodeServerEntry struct {
	Type    string          `json:"type"`
	URL     string          `json:"url"`
	Command json.RawMessage `json:"command"`
}

func inspectOpencodeConfig(path string) (Reference, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Reference{}, false
	}

	data = stripJSONC(data)

	var cfg opencodeConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Reference{}, false
	}

	raw, ok := cfg.MCP["kasmos"]
	if !ok {
		return Reference{}, false
	}

	var entry opencodeServerEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return Reference{}, false
	}

	shortName := filepath.Base(path)
	ref := Reference{
		File:  shortName,
		Label: "mcp.kasmos",
	}

	if entry.Type == "remote" || (len(entry.Command) == 0 && entry.URL != "") {
		ref.RawPath = entry.URL
		if entry.URL == ExpectedSharedHTTPURL {
			ref.Transport = TransportSharedHTTP
		} else {
			// Arbitrary remote url — do not label as shared http and do not
			// count as healthy. Transport stays empty so the renderer falls
			// through to the unhealthy path.
			ref.Note = "unexpected remote url: expected " + ExpectedSharedHTTPURL
		}
		return ref, true
	}

	cmd, _ := parseOpencodeCommand(entry.Command)
	ref.RawPath = cmd
	ref.Normalized, ref.Note = normalizePath(cmd)
	return ref, true
}

// parseOpencodeCommand handles command fields that can be a string or array.
func parseOpencodeCommand(raw json.RawMessage) (string, []string) {
	if len(raw) == 0 {
		return "", nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr[0], arr[1:]
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s, nil
	}
	return "", nil
}

// ── Linux systemd ─────────────────────────────────────────────────────────────

func inspectLinuxServices(home string) []Reference {
	svcDir := filepath.Join(home, ".config", "systemd", "user")

	var refs []Reference

	// kasmos.service — ExecStart and ExecStop
	kasmosPath := filepath.Join(svcDir, "kasmos.service")
	kasmosData, err := os.ReadFile(kasmosPath)
	if err != nil {
		refs = append(refs,
			Reference{File: "kasmos.service", Label: "ExecStart", Note: "not installed"},
			Reference{File: "kasmos.service", Label: "ExecStop", Note: "not installed"},
		)
	} else {
		refs = append(refs, parseSystemdRefs("kasmos.service", home, kasmosData, []string{"ExecStart", "ExecStop"})...)
	}

	// kasmosdb.service — ExecStart only
	kasmosdbPath := filepath.Join(svcDir, "kasmosdb.service")
	kasmosdbData, err := os.ReadFile(kasmosdbPath)
	if err != nil {
		refs = append(refs, Reference{File: "kasmosdb.service", Label: "ExecStart", Note: "not installed"})
	} else {
		refs = append(refs, parseSystemdRefs("kasmosdb.service", home, kasmosdbData, []string{"ExecStart"})...)
	}

	return refs
}

// parseSystemdRefs extracts the binary (first word of the value) from each
// target key in a systemd unit file. %h is expanded to home.
func parseSystemdRefs(file, home string, data []byte, keys []string) []Reference {
	wanted := make(map[string]bool, len(keys))
	for _, k := range keys {
		wanted[k] = true
	}
	found := make(map[string]Reference, len(keys))

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		for _, k := range keys {
			prefix := k + "="
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			value := strings.TrimPrefix(line, prefix)
			// First word is the binary path.
			parts := strings.Fields(value)
			if len(parts) == 0 {
				continue
			}
			rawBin := parts[0]
			// Expand %h to home for normalization only; preserve RawPath as written.
			expanded := strings.ReplaceAll(rawBin, "%h", home)

			var normalized, note string
			if rawBin == expanded {
				// No %h expansion needed.
				normalized, note = normalizePath(rawBin)
			} else {
				// %h was expanded — normalize the expanded path but keep rawBin.
				normalized, note = normalizePath(expanded)
			}

			found[k] = Reference{
				File:       file,
				Label:      k,
				RawPath:    rawBin,
				Normalized: normalized,
				Note:       note,
			}
		}
	}

	// Build result in keys order; missing directives get empty refs.
	refs := make([]Reference, 0, len(keys))
	for _, k := range keys {
		if r, ok := found[k]; ok {
			refs = append(refs, r)
		} else if wanted[k] {
			refs = append(refs, Reference{File: file, Label: k, Note: "directive not found"})
		}
	}
	return refs
}

// ── macOS launchd ─────────────────────────────────────────────────────────────

// plistDoc is the minimal XML structure for reading a launchd plist.
type plistDoc struct {
	Dict plistDict `xml:"dict"`
}

type plistDict struct {
	Entries []plistEntry
}

type plistEntry struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// parsePlist extracts ProgramArguments[0] from a launchd plist file using
// encoding/xml so we don't need a third-party plist library.
func parsePlist(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Walk the XML manually to find <array> after <key>ProgramArguments</key>.
	type xmlToken struct {
		kind  string // "key", "string", "array-start", "array-end"
		value string
	}

	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var tokens []xmlToken
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "array" {
				tokens = append(tokens, xmlToken{kind: "array-start"})
			}
		case xml.EndElement:
			if v.Name.Local == "array" {
				tokens = append(tokens, xmlToken{kind: "array-end"})
			}
		case xml.CharData:
			s := strings.TrimSpace(string(v))
			if s != "" {
				tokens = append(tokens, xmlToken{kind: "chardata", value: s})
			}
		}
	}

	// Find ProgramArguments key then take first string in following array.
	for i, t := range tokens {
		if t.kind != "chardata" || t.value != "ProgramArguments" {
			continue
		}
		// Find array-start after this token.
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].kind == "array-start" {
				// First chardata inside the array is ProgramArguments[0].
				for k := j + 1; k < len(tokens) && tokens[k].kind != "array-end"; k++ {
					if tokens[k].kind == "chardata" {
						return tokens[k].value, nil
					}
				}
			}
			// If we hit another chardata before an array-start, stop looking.
			if tokens[j].kind == "chardata" {
				break
			}
		}
	}
	return "", nil
}

func inspectDarwinPlists(home string) []Reference {
	laDir := filepath.Join(home, "Library", "LaunchAgents")

	plists := []struct {
		file  string
		label string
	}{
		{"com.kasmos.daemon.plist", "ProgramArguments[0]"},
		{"com.kasmos.taskstore.plist", "ProgramArguments[0]"},
	}

	var refs []Reference
	for _, p := range plists {
		path := filepath.Join(laDir, p.file)
		bin, err := parsePlist(path)
		if err != nil || bin == "" {
			refs = append(refs, Reference{File: p.file, Label: p.label, Note: "not installed"})
			continue
		}
		normalized, note := normalizePath(bin)
		refs = append(refs, Reference{
			File:       p.file,
			Label:      p.label,
			RawPath:    bin,
			Normalized: normalized,
			Note:       note,
		})
	}
	return refs
}

// ── Normalization ─────────────────────────────────────────────────────────────

// normalizePath returns the canonical absolute path and a note explaining
// why normalization was skipped (for bare names, placeholders, etc.).
// Returns ("", "placeholder") for __KAS_BIN__ and similar template markers.
// Returns ("", "bare name: use absolute path") for bare commands like "kas".
// Returns (evalSymlinks(path), "") for absolute paths.
func normalizePath(raw string) (normalized, note string) {
	if raw == "" {
		return "", "empty path"
	}

	// Placeholder pattern: all-caps with underscores, surrounded by __.
	if strings.HasPrefix(raw, "__") && strings.HasSuffix(raw, "__") {
		return "", "placeholder: template not substituted"
	}

	// Bare name (no path separator, no leading /).
	if !strings.Contains(raw, string(filepath.Separator)) && !strings.HasPrefix(raw, "/") {
		return "", "bare name: use absolute path"
	}

	// Absolute path — resolve symlinks.
	abs, err := filepath.Abs(raw)
	if err != nil {
		return raw, ""
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// File may not exist yet; fall back to abs.
		return abs, ""
	}
	return canonical, ""
}

// ── JSONC stripping ───────────────────────────────────────────────────────────

var trailingCommaRe = regexp.MustCompile(`,\s*([}\]])`)

// stripJSONC converts JSONC (JSON with // comments and trailing commas) to
// valid JSON, mirroring the logic in internal/clickup/detect.go.
func stripJSONC(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if idx := findLineComment(line); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	out := strings.Join(lines, "\n")
	out = trailingCommaRe.ReplaceAllString(out, "$1")
	return []byte(out)
}

// findLineComment returns the byte index of a // comment outside a JSON string,
// or -1 if none found.
func findLineComment(line string) int {
	inString := false
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '"' && (i == 0 || line[i-1] != '\\'):
			inString = !inString
		case !inString && i+1 < len(line) && line[i] == '/' && line[i+1] == '/':
			return i
		}
	}
	return -1
}
