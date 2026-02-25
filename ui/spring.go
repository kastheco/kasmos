package ui

import (
	"math"

	"github.com/charmbracelet/harmonica"
)

// SpringAnim drives a spring-physics animation from 0 to a target value.
// Used for the banner load-in: target is the number of banner rows (6).
type SpringAnim struct {
	spring  harmonica.Spring
	pos     float64
	vel     float64
	target  float64
	settled bool
}

// NewSpringAnim creates a spring animation targeting the given value.
// Tuned for 20fps (50ms tick), under-damped for a satisfying bounce.
func NewSpringAnim(target float64) *SpringAnim {
	return &SpringAnim{
		spring: harmonica.NewSpring(harmonica.FPS(20), 6.0, 0.5),
		target: target,
	}
}

// Tick advances the spring by one frame. Returns true while still animating,
// false once settled.
func (s *SpringAnim) Tick() bool {
	if s.settled {
		return false
	}
	s.pos, s.vel = s.spring.Update(s.pos, s.vel, s.target)

	// Settled when close to target with negligible velocity.
	if math.Abs(s.pos-s.target) < 0.01 && math.Abs(s.vel) < 0.01 {
		s.pos = s.target
		s.vel = 0
		s.settled = true
		return false
	}
	return true
}

// VisibleRows returns the current number of visible rows, clamped to [0, target].
func (s *SpringAnim) VisibleRows() int {
	rows := int(math.Round(s.pos))
	if rows < 0 {
		return 0
	}
	maxRows := int(s.target)
	if rows > maxRows {
		return maxRows
	}
	return rows
}

// Settled returns true once the spring has reached equilibrium.
func (s *SpringAnim) Settled() bool {
	return s.settled
}
