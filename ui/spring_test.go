package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSpringAnim(t *testing.T) {
	s := NewSpringAnim(6.0, 15)
	s.SetCTALen(20)
	require.NotNil(t, s)
	assert.Equal(t, 0, s.VisibleRows())
	assert.False(t, s.Settled())
	assert.False(t, s.CTAReady())
}

func TestSpringAnim_ConvergesToTarget(t *testing.T) {
	s := NewSpringAnim(6.0, 15)
	s.SetCTALen(20)

	for i := 0; i < 300; i++ {
		if s.Settled() {
			break
		}
		s.Tick()
	}

	assert.True(t, s.Settled(), "spring should settle within 300 ticks")
	assert.Equal(t, 6, s.VisibleRows(), "should converge to target")
	assert.True(t, s.CTAReady(), "CTA should be ready after settling")
}

func TestSpringAnim_VisibleRowsClamped(t *testing.T) {
	s := NewSpringAnim(6.0, 0)
	s.SetCTALen(10)

	for i := 0; i < 300; i++ {
		rows := s.VisibleRows()
		assert.GreaterOrEqual(t, rows, 0, "rows should not be negative")
		assert.LessOrEqual(t, rows, 6, "rows should not exceed target")
		s.Tick()
	}
}

func TestSpringAnim_CTADelayed(t *testing.T) {
	s := NewSpringAnim(6.0, 10)
	s.SetCTALen(20)

	// Tick until all rows visible
	for i := 0; i < 300; i++ {
		if s.VisibleRows() >= 6 {
			break
		}
		s.Tick()
	}
	require.Equal(t, 6, s.VisibleRows())

	// CTA should not be ready immediately
	assert.False(t, s.CTAReady(), "CTA should not be ready immediately after unfold")

	// Tick through delay
	for i := 0; i < 10; i++ {
		s.Tick()
	}
	assert.True(t, s.CTAReady(), "CTA should be ready after delay ticks")
}

func TestSpringAnim_CTAHorizontalReveal(t *testing.T) {
	s := NewSpringAnim(6.0, 0) // no delay
	s.SetCTALen(12)

	// Tick until CTA starts revealing
	for i := 0; i < 300; i++ {
		if s.CTAReady() {
			break
		}
		s.Tick()
	}
	require.True(t, s.CTAReady())

	// One more tick to start revealing chars
	s.Tick()
	chars := s.CTAVisibleChars()
	assert.Greater(t, chars, 0, "should have some chars visible after tick")

	// Tick until fully revealed
	for i := 0; i < 20; i++ {
		s.Tick()
	}
	assert.GreaterOrEqual(t, s.CTAVisibleChars(), 12, "should reveal all chars")
}

func TestSpringAnim_TickReturnsFalseWhenSettled(t *testing.T) {
	s := NewSpringAnim(6.0, 5)
	s.SetCTALen(9) // 3 chars/tick * 3 ticks = done fast

	assert.True(t, s.Tick(), "should return true while animating")

	for i := 0; i < 300; i++ {
		if !s.Tick() {
			break
		}
	}

	assert.False(t, s.Tick(), "should return false after fully settled")
	assert.True(t, s.CTAReady())
}
