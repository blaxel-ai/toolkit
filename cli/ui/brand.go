// Package ui draws Blaxel's setup experience: a centered full-screen app on
// color terminals, and the same steps as plain lines everywhere else (logs,
// CI, coding agents, NO_COLOR, TERM=dumb).
package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// The wordmark, split into 4-column bands that each take one gradient step.
// Blocks take the band color; the box-drawing shadow takes a darker one.
var wordmark = [][]string{
	{"████", "██╗ ", "██╗ ", "    ", " ███", "██╗ ", "██╗ ", " ██╗", "████", "███╗", "██╗ ", "    "},
	{"██╔═", "═██╗", "██║ ", "    ", "██╔═", "═██╗", "╚██╗", "██╔╝", "██╔═", "═══╝", "██║ ", "    "},
	{"████", "██╔╝", "██║ ", "    ", "████", "███║", " ╚██", "█╔╝ ", "████", "█╗  ", "██║ ", "    "},
	{"██╔═", "═██╗", "██║ ", "    ", "██╔═", "═██║", " ██╔", "██╗ ", "██╔═", "═╝  ", "██║ ", "    "},
	{"████", "██╔╝", "████", "███╗", "██║ ", " ██║", "██╔╝", " ██╗", "████", "███╗", "████", "███╗"},
	{"╚═══", "══╝ ", "╚═══", "═══╝", "╚═╝ ", " ╚═╝", "╚═╝ ", " ╚═╝", "╚═══", "═══╝", "╚═══", "═══╝"},
}

// The compact wordmark for small terminals, one band per letter.
var wordmarkSmall = [][]string{
	{"█▄▄ ", "█   ", "▄▀█ ", "▀▄▀ ", "█▀▀ ", "█  "},
	{"█▄█ ", "█▄▄ ", "█▀█ ", "█ █ ", "██▄ ", "█▄▄"},
}

const (
	wordmarkWidth      = 48
	wordmarkSmallWidth = 23
)

// The gradient runs through the logo colors: red, orange and amber. The
// 256-color steps are picked by eye, since the nearest palette colors are
// pink and olive.
var (
	faceTrue    = []string{"#ef4136", "#f24c36", "#f45636", "#f76135", "#f96b35", "#fc7635", "#fd8036", "#fc8938", "#fc933a", "#fc9d3c", "#fba63e", "#fbb040"}
	shadowTrue  = []string{"#6c1d18", "#6d2218", "#6e2718", "#6f2c18", "#703018", "#713518", "#723a18", "#713e19", "#71421a", "#71471b", "#714b1c", "#714f1d"}
	face256     = []string{"202", "202", "202", "202", "208", "208", "208", "208", "214", "214", "214", "214"}
	shadow256   = []string{"52", "52", "52", "52", "52", "52", "94", "94", "94", "94", "94", "94"}
	brandOrange = "#fd7b35"
)

// palette holds the colors of one terminal's color profile.
type palette struct {
	profile termenv.Profile
}

func (p palette) seq(true, c256, basic string) string {
	switch p.profile {
	case termenv.TrueColor:
		return termenv.TrueColor.Color(true).Sequence(false)
	case termenv.ANSI256:
		return termenv.ANSI256.Color(c256).Sequence(false)
	case termenv.ANSI:
		return basic
	}
	return ""
}

func (p palette) paint(text, sequence string) string {
	if sequence == "" || text == "" {
		return text
	}
	return "\x1b[" + sequence + "m" + text + "\x1b[0m"
}

// renderWordmark draws the wordmark that fits in width. A highlight sweeps
// across it when sweep is a band index; -1 draws it still.
func (p palette) renderWordmark(width int, sweep int) []string {
	rows, bands := wordmark, 12
	if width < wordmarkWidth {
		rows, bands = wordmarkSmall, 6
	}
	if p.profile == termenv.Ascii || width < wordmarkSmallWidth {
		return []string{"B L A X E L"}
	}
	lines := make([]string, len(rows))
	for y, row := range rows {
		var line strings.Builder
		for band, text := range row {
			step := band * (len(faceTrue) - 1) / max(bands-1, 1)
			face := p.seq(faceTrue[step], face256[step], basicFace(step))
			shadow := p.seq(shadowTrue[step], shadow256[step], "90")
			if band == sweep || band == sweep-1 {
				face = p.seq(lighten(faceTrue[step], 0.55), "223", "1;97")
			}
			line.WriteString(paintBand(p, text, face, shadow))
		}
		lines[y] = line.String()
	}
	return lines
}

func basicFace(step int) string {
	if step < 6 {
		return "1;31"
	}
	return "1;33"
}

// paintBand colors blocks with the face color and everything else with the shadow.
func paintBand(p palette, text, face, shadow string) string {
	var out strings.Builder
	var run strings.Builder
	runFace := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if runFace {
			out.WriteString(p.paint(run.String(), face))
		} else {
			out.WriteString(p.paint(run.String(), shadow))
		}
		run.Reset()
	}
	for _, r := range text {
		isFace := r == '█' || r == '▀' || r == '▄'
		if r == ' ' {
			flush()
			out.WriteRune(r)
			continue
		}
		if isFace != runFace {
			flush()
			runFace = isFace
		}
		run.WriteRune(r)
	}
	flush()
	return out.String()
}

// lighten mixes a hex color with white.
func lighten(hex string, amount float64) string {
	value, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	channel := func(shift uint) uint64 {
		c := float64((value >> shift) & 0xff)
		return uint64(c + (255-c)*amount)
	}
	return "#" + strconv.FormatUint(channel(16)<<16|channel(8)<<8|channel(0), 16)
}

// styles are the text styles of the screens.
type styles struct {
	title, label, muted, accent, ok, warn, fail, key, rail, button, buttonIdle lipgloss.Style
}

func newStyles(r *lipgloss.Renderer) styles {
	color := func(true, c256 string) lipgloss.TerminalColor {
		return lipgloss.CompleteColor{TrueColor: true, ANSI256: c256, ANSI: ansiFor(c256)}
	}
	accent := color(brandOrange, "208")
	return styles{
		title:      r.NewStyle().Bold(true),
		label:      r.NewStyle(),
		muted:      r.NewStyle().Foreground(color("#8b919e", "245")),
		accent:     r.NewStyle().Foreground(accent),
		ok:         r.NewStyle().Foreground(color("#3ddc84", "78")),
		warn:       r.NewStyle().Foreground(color("#fbb040", "214")),
		fail:       r.NewStyle().Foreground(color("#ef4136", "203")),
		key:        r.NewStyle().Foreground(color("#c6cad3", "250")),
		rail:       r.NewStyle().Foreground(color("#4b505c", "239")),
		button:     r.NewStyle().Bold(true).Foreground(color("#16181d", "234")).Background(accent).Padding(0, 3),
		buttonIdle: r.NewStyle().Bold(true).Foreground(accent).Padding(0, 3),
	}
}

func ansiFor(c256 string) string {
	switch c256 {
	case "208", "214":
		return "3"
	case "203":
		return "1"
	case "78":
		return "2"
	case "234":
		return "0"
	}
	return "8"
}
