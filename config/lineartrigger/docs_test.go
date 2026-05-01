package lineartrigger

import (
	"os"
	"strings"
	"testing"

	"github.com/kastheco/kasmos/config/auditlog"
)

func TestLinearTriggersDocsPresence(t *testing.T) {
	body, err := os.ReadFile("../../web/docs/docs/guides/linear-triggers.mdx")
	if err != nil {
		t.Fatal(err)
	}

	docs := string(body)
	for _, verb := range AllVerbs() {
		if !strings.Contains(docs, string(verb)) {
			t.Fatalf("linear triggers docs missing verb %q", verb)
		}
	}

	for _, kind := range []auditlog.EventKind{
		auditlog.EventTaskLinearTriggerReceived,
		auditlog.EventTaskLinearTriggerDispatched,
		auditlog.EventTaskLinearTriggerRejected,
		auditlog.EventTaskLinearTriggerIgnored,
		auditlog.EventTaskLinearTriggerCommentFailed,
	} {
		if !strings.Contains(docs, string(kind)) {
			t.Fatalf("linear triggers docs missing audit kind %q", kind)
		}
	}

	for _, key := range []string{
		"enabled",
		"poll_interval",
		"lookback",
		"max_issues_per_poll",
		"verbs",
		"ack_comment_body",
		"routes",
		"team_id",
		"project_id",
		"require_labels",
		"topic",
		"branch_prefix",
		"labels",
		"create",
		"plan",
		"start",
		"ack",
		"actor",
		"allowed_user_ids",
		"allowed_user_emails",
		"allow_public_status",
		"start_guard",
		"require_start_label",
		"allow_label_start",
	} {
		if !strings.Contains(docs, key) {
			t.Fatalf("linear triggers docs missing config key %q", key)
		}
	}
}
