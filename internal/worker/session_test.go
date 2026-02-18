package worker

import "testing"

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "text format",
			input:  "[coder] session: ses_ab12CD34",
			output: "ses_ab12CD34",
		},
		{
			name:   "json format",
			input:  `{"event":"start","session_id":"ses_Z9y8X7"}`,
			output: "ses_Z9y8X7",
		},
		{
			name:   "no match",
			input:  "no session in this output",
			output: "",
		},
		{
			name:   "partial text",
			input:  "session: ses_",
			output: "",
		},
		{
			name:   "partial json",
			input:  `{"session_id":"ses_"}`,
			output: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractSessionID(tc.input)
			if got != tc.output {
				t.Fatalf("extractSessionID() = %q, want %q", got, tc.output)
			}
		})
	}
}
