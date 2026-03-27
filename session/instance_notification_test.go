package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldSendCompletionNotification(t *testing.T) {
	tests := []struct {
		name string
		inst Instance
		want bool
	}{
		{
			name: "implementation complete standalone",
			inst: Instance{ImplementationComplete: true},
			want: true,
		},
		{
			name: "exited standalone",
			inst: Instance{Exited: true},
			want: true,
		},
		{
			name: "idle ready while still running",
			inst: Instance{},
			want: false,
		},
		{
			name: "wave task suppressed",
			inst: Instance{ImplementationComplete: true, TaskNumber: 2},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.inst.shouldSendCompletionNotification())
		})
	}
}
