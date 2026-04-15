package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNonLoopbackAdminWarning(t *testing.T) {
	tests := []struct {
		addr        string
		wantWarn    bool
		wantContain string
	}{
		{
			addr:        "0.0.0.0:7433",
			wantWarn:    true,
			wantContain: "admin API has no built-in auth",
		},
		{
			addr:        ":7433",
			wantWarn:    true,
			wantContain: "admin API has no built-in auth",
		},
		{
			addr:        "192.168.1.5:7433",
			wantWarn:    true,
			wantContain: "admin API has no built-in auth",
		},
		{
			addr:     "localhost:7433",
			wantWarn: false,
		},
		{
			addr:     "127.0.0.1:7433",
			wantWarn: false,
		},
		{
			addr:     "[::1]:7433",
			wantWarn: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			msg, warn := nonLoopbackAdminWarning(tc.addr)
			assert.Equal(t, tc.wantWarn, warn, "warn flag mismatch for addr %q", tc.addr)
			if tc.wantWarn {
				assert.Contains(t, msg, tc.wantContain, "warning text mismatch for addr %q", tc.addr)
			} else {
				assert.Empty(t, msg, "expected no warning message for loopback addr %q", tc.addr)
			}
		})
	}
}
