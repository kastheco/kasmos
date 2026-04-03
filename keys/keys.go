package keys

import (
	"charm.land/bubbles/v2/key"
)

type KeyName int

const (
	KeyUp KeyName = iota
	KeyDown
	KeyEnter
	KeyKill  // ctrl+k — soft kill: terminates tmux session, keeps instance in list
	KeyAbort // ctrl+shift+k — full abort: kills tmux, removes worktree, removes from list
	KeyQuit
	KeyReview
	KeyPush
	KeySubmit

	KeyTab        // Tab is a special keybinding for cycling the focus ring.
	KeySubmitName // SubmitName is a special keybinding for submitting the name of a new instance.

	KeyCheckout
	KeyResume
	KeyPrompt // New key for entering a prompt
	KeyHelp   // Key for showing help screen

	KeyNewPlan    // Key for creating a new plan
	KeySearch     // Key for activating search
	KeyArrowLeft  // Key for in-pane horizontal navigation left (tree collapse, etc.)
	KeyArrowRight // Key for in-pane horizontal navigation right (tree expand, etc.)

	KeyCreatePR // Key for creating a pull request

	KeySendPrompt // Key for sending a prompt to a running instance
	KeySendYes    // Key for sending yes to a waiting instance

	KeySpace // Key for opening context menu on selected item
	KeyCommandLauncher

	KeyInfoTab // Key for jumping directly to info tab

	// Tab switching keybindings
	KeyTabAgent
	KeyTabInfo

	KeySpawnAgent    // S - spawn ad-hoc agent session
	KeyQuickLaunch   // s - quick launch ad-hoc agent session
	KeyFocusList     // Key for focusing the right sidebar / instance list
	KeyViewPlan      // Key for viewing the selected plan's markdown
	KeyToggleSidebar // Key for toggling sidebar visibility
	KeyExitFocus     // Key for exiting focus/interactive mode (ctrl+space)
	KeySubmitExit    // Key for submitting input and exiting focus/interactive mode (ctrl+shift+enter)
	KeySpaceExpand   // Space key with expand/collapse label (sidebar context)

	KeyTmuxBrowser // t - browse orphaned tmux sessions

	KeyAuditToggle // L - toggle audit log pane visibility
	KeyAuditCursor // A - enter audit log cursor mode (navigate log lines)
	KeyBrowser     // b - open the admin plan browser
)

// Backward-compatible aliases; prefer KeyInfoTab/KeyTabInfo.
const (
	KeyGitTab = KeyInfoTab
	KeyTabGit = KeyTabInfo
)

// GlobalKeyStringsMap is a global, immutable map string to keybinding.
var GlobalKeyStringsMap = map[string]KeyName{
	"up":           KeyUp,
	"down":         KeyDown,
	"N":            KeyPrompt,
	"enter":        KeyEnter,
	"o":            KeyEnter,
	"n":            KeyNewPlan,
	"ctrl+k":       KeyKill,
	"ctrl+shift+k": KeyAbort,
	"q":            KeyQuit,
	"tab":          KeyTab,
	"r":            KeyResume,
	"?":            KeyHelp,
	"S":            KeySpawnAgent,
	"/":            KeySearch,
	"left":         KeyArrowLeft,
	"right":        KeyArrowRight,
	"P":            KeyCreatePR,
	"i":            KeySendPrompt,
	"y":            KeySendYes,
	" ":            KeySpace,
	"space":        KeySpace, // msg.String() returns "space" for tea.KeySpace with Text=" "
	"t":            KeyTmuxBrowser,
	"s":            KeyQuickLaunch,
	"L":            KeyAuditToggle,
	"A":            KeyAuditCursor,
	"b":            KeyBrowser,
	"p":            KeyViewPlan,
	"ctrl+s":       KeyToggleSidebar,
	"ctrl+space":   KeyExitFocus,
	"I":            KeyInfoTab,
	"!":            KeyTabAgent,
}

// GlobalkeyBindings is a global, immutable map of KeyName tot keybinding.
var GlobalkeyBindings = map[KeyName]key.Binding{
	KeyUp: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	KeyDown: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	KeyEnter: key.NewBinding(
		key.WithKeys("enter", "o"),
		key.WithHelp("↵/o", "select"),
	),
	KeyKill: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "kill"),
	),
	KeyAbort: key.NewBinding(
		key.WithKeys("ctrl+shift+k"),
		key.WithHelp("ctrl+shift+k", "abort"),
	),
	KeyHelp: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	KeyQuit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	KeyNewPlan: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new plan"),
	),
	KeyPrompt: key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "new with prompt"),
	),
	KeyTab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "cycle panes"),
	),
	KeyResume: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "resume"),
	),
	KeySearch: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	KeyCreatePR: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "create PR"),
	),
	KeyArrowLeft: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "left"),
	),
	KeyArrowRight: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "right"),
	),
	KeySendPrompt: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "interactive"),
	),
	KeySendYes: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "yes"),
	),
	KeySpace: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "menu"),
	),
	KeyCommandLauncher: key.NewBinding(
		key.WithKeys("shift+space"),
		key.WithHelp("shift+space", "commands"),
	),
	KeySpawnAgent: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "spawn agent"),
	),
	KeyQuickLaunch: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "quick launch"),
	),
	KeyViewPlan: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "view plan"),
	),
	KeyToggleSidebar: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "toggle sidebar"),
	),
	KeyInfoTab: key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("I", "info tab"),
	),
	KeyTabAgent: key.NewBinding(
		key.WithKeys("!"),
		key.WithHelp("!", "switch tab"),
	),
	KeyExitFocus: key.NewBinding(
		key.WithKeys("ctrl+space"),
		key.WithHelp("ctrl+space", "exit focus"),
	),
	KeySubmitExit: key.NewBinding(
		key.WithKeys("ctrl+shift+enter"),
		key.WithHelp("ctrl+shift+↵", "submit + exit"),
	),

	KeySpaceExpand: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle"),
	),

	KeyTmuxBrowser: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "tmux sessions"),
	),

	KeyAuditToggle: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "log"),
	),

	KeyAuditCursor: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "log actions"),
	),

	KeyBrowser: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "browser"),
	),

	// -- Special keybindings --

	KeySubmitName: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "submit name"),
	),
}
