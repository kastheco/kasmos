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
			name:   "dash format",
			input:  "[planner] session: sess-abc123",
			output: "sess-abc123",
		},
		{
			name:   "json dash format",
			input:  `{"event":"start","session_id":"sess-42"}`,
			output: "sess-42",
		},
		{
			name:   "bare ses line",
			input:  "ses_abc123def456",
			output: "ses_abc123def456",
		},
		{
			name:   "session prefix line",
			input:  "Session: ses_abc123",
			output: "ses_abc123",
		},
		{
			name:   "session id prefix line",
			input:  "Session ID: ses_abc123",
			output: "ses_abc123",
		},
		{
			name:   "session id lowercase label",
			input:  "session id: ses_abc123",
			output: "ses_abc123",
		},
		{
			name: "broad fallback scans tail bottom up",
			input: "session noise\n" +
				"ses_old111\n" +
				"line\n" +
				"latest session is ses_new999",
			output: "ses_new999",
		},
		{
			name: "broad fallback only scans last 50 lines",
			input: "ses_too_old\n" +
				"1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n" +
				"11\n12\n13\n14\n15\n16\n17\n18\n19\n20\n" +
				"21\n22\n23\n24\n25\n26\n27\n28\n29\n30\n" +
				"31\n32\n33\n34\n35\n36\n37\n38\n39\n40\n" +
				"41\n42\n43\n44\n45\n46\n47\n48\n49\n50\n51",
			output: "",
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
			got := ExtractSessionID(tc.input)
			if got != tc.output {
				t.Fatalf("ExtractSessionID() = %q, want %q", got, tc.output)
			}
		})
	}
}
