package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSpringAnim(t *testing.T) {
	s := NewSpringAnim(6.0)
	require.NotNil(t, s)
	assert.Equal(t, 0, s.VisibleRows())
	assert.False(t, s.Settled())
}

func TestSpringAnim_ConvergesToTarget(t *testing.T) {
	s := NewSpringAnim(6.0)

	// Tick until settled (should settle within 30 ticks at 20fps = 1.5s max)
	for i := 0; i < 60; i++ {
		if s.Settled() {
			break
		}
		s.Tick()
	}

	assert.True(t, s.Settled(), "spring should settle within 60 ticks")
	assert.Equal(t, 6, s.VisibleRows(), "should converge to target")
}

func TestSpringAnim_VisibleRowsClamped(t *testing.T) {
	s := NewSpringAnim(6.0)

	// Tick a bunch — visible rows should never exceed 6 or go below 0
	for i := 0; i < 60; i++ {
		rows := s.VisibleRows()
		assert.GreaterOrEqual(t, rows, 0, "rows should not be negative")
		assert.LessOrEqual(t, rows, 6, "rows should not exceed target")
		s.Tick()
	}
}

func TestSpringAnim_TickReturnsFalseWhenSettled(t *testing.T) {
	s := NewSpringAnim(6.0)

	// Should return true while animating
	assert.True(t, s.Tick(), "should return true while animating")

	// Tick until settled
	for i := 0; i < 60; i++ {
		if !s.Tick() {
			break
		}
	}

	// After settling, Tick returns false
	assert.False(t, s.Tick(), "should return false after settling")
}
