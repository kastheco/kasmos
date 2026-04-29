package ui

import "strings"

// fallbackBannerRaw is the 6-row KASMOS block-art banner using Unicode box-drawing characters.
var fallbackBannerRaw = `██╗  ██╗ █████╗ ███████╗███╗   ███╗ ██████╗ ███████╗
██║ ██╔╝██╔══██╗██╔════╝████╗ ████║██╔═══██╗██╔════╝
█████╔╝ ███████║███████╗██╔████╔██║██║   ██║███████╗
██╔═██╗ ██╔══██║╚════██║██║╚██╔╝██║██║   ██║╚════██║
██║  ██╗██║  ██║███████║██║ ╚═╝ ██║╚██████╔╝███████║
╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝`

// blockPeriod is a block-art period glyph, 6 rows tall to match the banner height.
// The visible dot occupies the bottom two rows; the upper four rows are blank.
var blockPeriod = [6]string{
	"   ",
	"   ",
	"   ",
	"   ",
	"██╗",
	"╚═╝",
}

// bannerFrames holds the precomputed gradient-rendered banner strings for each animation frame.
// Frames progress: base → one period → two periods → three periods (then cycle).
var bannerFrames = buildBannerFrames()

func buildBannerFrames() []string {
	base := strings.Split(fallbackBannerRaw, "\n")

	type glyph = [6]string
	suffixes := [][]glyph{
		{},                                      // KASMOS
		{blockPeriod},                           // KASMOS.
		{blockPeriod, blockPeriod},              // KASMOS..
		{blockPeriod, blockPeriod, blockPeriod}, // KASMOS...
	}

	frames := make([]string, len(suffixes))
	for i, glyphs := range suffixes {
		lines := make([]string, 6)
		copy(lines, base)
		for _, g := range glyphs {
			for row := 0; row < 6; row++ {
				lines[row] += " " + g[row]
			}
		}
		frames[i] = GradientText(strings.Join(lines, "\n"), GradientStart, GradientEnd)
	}
	return frames
}

func rebuildBannerFrames() {
	bannerFrames = buildBannerFrames()
}

// FallBackText returns the precomputed gradient banner string for the given animation tick.
// The frame index wraps around automatically.
func FallBackText(frame int) string {
	return bannerFrames[frame%len(bannerFrames)]
}

// BannerLines returns the precomputed gradient banner split into individual lines
// for the given animation frame. Always returns exactly 6 lines.
func BannerLines(frame int) []string {
	return strings.Split(bannerFrames[frame%len(bannerFrames)], "\n")
}
