package linearreceipt

import (
	"io"
	"text/template"
	"time"
)

type receiptTemplate struct {
	*template.Template
}

var (
	lifecycleTemplate = mustReceiptTemplate("lifecycle", `kasmos status receipt

task: {{.Filename}}
project: {{.Project}}
branch: {{.Branch}}
identifier: {{.Identifier}}
event: {{.Event}}
status: {{.From}} -> {{.To}}
{{- if .PRURL}}
pr: {{.PRURL}}
{{- end}}
{{- if .ReviewBody}}

review notes:
{{.ReviewBody}}
{{- end}}
time: {{utcTimestamp .When}}
`)

	prOpenedTemplate = mustReceiptTemplate("pr_opened", `kasmos pr receipt

task: {{.Filename}}
project: {{.Project}}
branch: {{.Branch}}
identifier: {{.Identifier}}
pr: {{.PRURL}}
time: {{utcTimestamp .When}}
`)

	mergedTemplate = mustReceiptTemplate("merged", `kasmos merge receipt

task: {{.Filename}}
project: {{.Project}}
branch: {{.Branch}}
identifier: {{.Identifier}}
merge: {{.MergeRef}}
time: {{utcTimestamp .When}}
`)

	cancelledTemplate = mustReceiptTemplate("cancelled", `kasmos cancellation receipt

task: {{.Filename}}
project: {{.Project}}
identifier: {{.Identifier}}
reason: {{.Reason}}
time: {{utcTimestamp .When}}
`)
)

func mustReceiptTemplate(name, body string) receiptTemplate {
	t := template.Must(template.New(name).Funcs(template.FuncMap{
		"utcTimestamp": utcTimestamp,
	}).Option("missingkey=zero").Parse(body))
	return receiptTemplate{Template: t}
}

func (t receiptTemplate) Execute(w io.Writer, data any) error {
	return t.Template.Execute(w, data)
}

func utcTimestamp(when time.Time) string {
	return when.UTC().Format("2006-01-02T15:04:05Z")
}
