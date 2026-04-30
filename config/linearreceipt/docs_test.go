package linearreceipt

import (
	"os"
	"strings"
	"testing"
)

func TestLinearReceiptsDocsListDefaultEvents(t *testing.T) {
	body, err := os.ReadFile("../../web/docs/docs/guides/linear-receipts.mdx")
	if err != nil {
		t.Fatal(err)
	}

	docs := string(body)
	for _, event := range defaultEvents() {
		if !strings.Contains(docs, string(event)) {
			t.Fatalf("linear receipts docs missing default event %q", event)
		}
	}
}
