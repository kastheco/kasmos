package ui

import (
	"math"

	"github.com/charmbracelet/harmonica"
)

// SpringAnim drives a spring-physics animation from 0 to a target value.
// Used for the banner load-in: target is the number of banner rows (6).
// After all rows are visible, a delay timer starts before the CTA is revealed
// with a horizontal character-by-character unfold.
type SpringAnim struct {
	spring  harmonica.Spring
	pos     float64
	vel     float64
	target  float64
	settled bool

	// CTA delay: counts ticks after all rows are visible.
	ctaDelay int
	ctaTicks int
	ctaReady bool

	// CTA horizontal reveal: chars visible so far, chars per tick.
	ctaChars    int
	ctaPerTick  int
	ctaLen      int // total CTA length, set by SetCTALen
	ctaFullyRev bool
}

// NewSpringAnim creates a spring animation targeting the given value.
// ctaDelayTicks is how many ticks (at 50ms each) to wait after full unfold
// before starting the CTA horizontal reveal.
func NewSpringAnim(target float64, ctaDelayTicks int) *SpringAnim {
	return &SpringAnim{
		spring:     harmonica.NewSpring(harmonica.FPS(20), 4.0, 0.8),
		target:     target,
		ctaDelay:   ctaDelayTicks,
		ctaPerTick: 3, // reveal 3 chars per 50ms tick
	}
}

// SetCTALen tells the animation how long the CTA string is (in runes)
// so it knows when the horizontal reveal is complete.
func (s *SpringAnim) SetCTALen(n int) {
	s.ctaLen = n
}

// Tick advances the spring by one frame. Returns true while still animating.
func (s *SpringAnim) Tick() bool {
	if s.settled && s.ctaFullyRev {
		return false
	}

	if !s.settled {
		s.pos, s.vel = s.spring.Update(s.pos, s.vel, s.target)

		if math.Abs(s.pos-s.target) < 0.01 && math.Abs(s.vel) < 0.01 {
			s.pos = s.target
			s.vel = 0
			s.settled = true
		}
	}

	// Once all rows visible, count CTA delay ticks.
	if s.VisibleRows() >= int(s.target) {
		if !s.ctaReady {
			s.ctaTicks++
			if s.ctaTicks >= s.ctaDelay {
				s.ctaReady = true
			}
		} else if !s.ctaFullyRev {
			// Reveal characters
			s.ctaChars += s.ctaPerTick
			if s.ctaLen > 0 && s.ctaChars >= s.ctaLen {
				s.ctaChars = s.ctaLen
				s.ctaFullyRev = true
			}
		}
	}

	return !s.settled || !s.ctaFullyRev
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

// CTAReady returns true once the CTA delay has elapsed after full unfold.
func (s *SpringAnim) CTAReady() bool {
	return s.ctaReady
}

// CTAVisibleChars returns how many characters of the CTA to show.
// Returns 0 before ready, then increases each tick until full.
func (s *SpringAnim) CTAVisibleChars() int {
	if !s.ctaReady {
		return 0
	}
	return s.ctaChars
}

// Settled returns true once the spring has settled and CTA is fully revealed.
func (s *SpringAnim) Settled() bool {
	return s.settled && s.ctaFullyRev
}
