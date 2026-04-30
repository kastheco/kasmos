package linearreceipt

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kastheco/kasmos/config/taskfsm"
)

// LifecycleInput is the data fed to the lifecycle template.
type LifecycleInput struct {
	Project    string
	Filename   string
	Branch     string
	Identifier string
	URL        string
	Event      taskfsm.Event
	From       taskfsm.Status
	To         taskfsm.Status
	PRURL      string
	ReviewBody string // truncated to ReviewBodyLimit
	When       time.Time
}

// PRInput covers the pull request side-effect receipt.
type PRInput struct {
	Project    string
	Filename   string
	Branch     string
	Identifier string
	PRURL      string
	When       time.Time
}

// MergeInput covers the merge side-effect receipt.
type MergeInput struct {
	Project    string
	Filename   string
	Branch     string
	Identifier string
	MergeRef   string
	When       time.Time
}

// CancelInput covers the cancellation side-effect receipt.
type CancelInput struct {
	Project    string
	Filename   string
	Reason     string
	Identifier string
	When       time.Time
}

const ReviewBodyLimit = 800

const truncatedMarker = "…[truncated]"

// FormatLifecycle renders a deterministic lifecycle receipt body.
func FormatLifecycle(input LifecycleInput) string {
	input.Branch = branchOrFallback(input.Branch)
	input.Identifier = identifierOrFallback(input.Identifier, input.Filename)
	input.ReviewBody = truncateReviewBody(input.ReviewBody)
	return executeReceiptTemplate(lifecycleTemplate, input)
}

// FormatPROpened renders a deterministic pull request receipt body.
func FormatPROpened(input PRInput) string {
	input.Branch = branchOrFallback(input.Branch)
	input.Identifier = identifierOrFallback(input.Identifier, input.Filename)
	return executeReceiptTemplate(prOpenedTemplate, input)
}

// FormatMerged renders a deterministic merge receipt body.
func FormatMerged(input MergeInput) string {
	input.Branch = branchOrFallback(input.Branch)
	input.Identifier = identifierOrFallback(input.Identifier, input.Filename)
	return executeReceiptTemplate(mergedTemplate, input)
}

// FormatCancelled renders a deterministic cancellation receipt body.
func FormatCancelled(input CancelInput) string {
	input.Identifier = identifierOrFallback(input.Identifier, input.Filename)
	return executeReceiptTemplate(cancelledTemplate, input)
}

func executeReceiptTemplate(t receiptTemplate, data any) string {
	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		panic(err)
	}
	return strings.TrimRight(body.String(), "\n")
}

func identifierOrFallback(identifier, filename string) string {
	if identifier != "" {
		return identifier
	}
	return filename
}

func branchOrFallback(branch string) string {
	if branch != "" {
		return branch
	}
	return "(no branch)"
}

func truncateReviewBody(body string) string {
	if utf8.RuneCountInString(body) <= ReviewBodyLimit {
		return body
	}
	runes := []rune(body)
	return string(runes[:ReviewBodyLimit]) + truncatedMarker
}
