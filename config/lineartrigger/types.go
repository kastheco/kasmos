package lineartrigger

// Verb enumerates the recognised slash-command verbs.
type Verb string

const (
	VerbHelp   Verb = "help"
	VerbStatus Verb = "status"
	VerbLink   Verb = "link"
	VerbCreate Verb = "create"
	VerbPlan   Verb = "plan"
	VerbStart  Verb = "start"
)

// AllVerbs returns every recognised slash-command verb.
func AllVerbs() []Verb {
	return []Verb{VerbHelp, VerbStatus, VerbLink, VerbCreate, VerbPlan, VerbStart}
}

// Source identifies where a Linear trigger originated.
type Source string

const (
	SourceComment Source = "comment"
	SourceLabel   Source = "label"
)

// ParsedIntent is what the parser returns. The dispatcher consumes this directly.
type ParsedIntent struct {
	Source      Source
	Verb        Verb
	TaskFileArg string // optional one-word task identifier; never contains path separators
	IssueID     string // Linear UUID
	Identifier  string // e.g. ENG-123
	CommentID   string // empty for label-source intents
	LabelID     string // empty for comment-source intents
	AuthorID    string // Linear User.id; empty for label intents
	AuthorEmail string // optional; empty when not provided by Linear
}
