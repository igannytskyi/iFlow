package tui

import "github.com/charmbracelet/lipgloss"

// Theme is how the screen is coloured.
type Theme string

// The themes the screen knows.
const (
	// Light sets the terminal itself to black on white for as long as the
	// screen is open, and marks how things stand with soft blocks of colour.
	Light Theme = "light"
	// Terminal keeps the terminal's own colours, and marks with coloured
	// text that reads on a dark background as on a light one.
	Terminal Theme = "terminal"
)

// styles is every way the screen draws text.
type styles struct {
	faint, strong lipgloss.Style
	// good, bad, active and waiting mark how a thing stands, in text; the
	// blocks mark it as a block of colour, for the six areas.
	good, bad, active, waiting                  lipgloss.Style
	goodBlock, badBlock, activeBlock, waitBlock lipgloss.Style
	idleBlock                                   lipgloss.Style
	chosen                                      lipgloss.Style // what the keys or the mouse act on
	rule                                        lipgloss.Style
	// you and model are who speaks in a conversation.
	you, model lipgloss.Style
}

// stylesOf is the styles of a theme.
func stylesOf(t Theme) styles {
	if t == Light {
		block := func(bg, fg string) lipgloss.Style {
			return lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg)).Padding(0, 1)
		}
		ink := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }
		return styles{
			faint: ink("#7a776f"), strong: ink("#1f1f1d").Bold(true),
			good: ink("#3b6d11"), bad: ink("#a32d2d"), active: ink("#185fa5"), waiting: ink("#854f0b"),
			goodBlock: block("#e4f0d6", "#27500a"), badBlock: block("#fbe3e3", "#791f1f"),
			activeBlock: block("#dfebf8", "#0c447c"), waitBlock: block("#fbecd2", "#633806"),
			idleBlock: block("#f1efe8", "#5f5e5a"),
			chosen:    lipgloss.NewStyle().Background(lipgloss.Color("#dbe7f6")).Foreground(lipgloss.Color("#0c2f57")),
			rule: lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderTop(true).
				BorderForeground(lipgloss.Color("#d9d6cc")),
			you: ink("#185fa5").Bold(true), model: ink("#0f6e56").Bold(true),
		}
	}
	adaptive := func(light, dark string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: light, Dark: dark})
	}
	good, bad := adaptive("#3b6d11", "#97c459"), adaptive("#a32d2d", "#f09595")
	active, waiting := adaptive("#185fa5", "#85b7eb"), adaptive("#854f0b", "#fac775")
	return styles{
		faint: adaptive("#8a8a85", "#7a7a75"), strong: lipgloss.NewStyle().Bold(true),
		good: good, bad: bad, active: active, waiting: waiting,
		goodBlock: good, badBlock: bad, activeBlock: active, waitBlock: waiting,
		idleBlock: adaptive("#8a8a85", "#7a7a75"),
		chosen:    lipgloss.NewStyle().Reverse(true),
		rule: lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderTop(true).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#d3d1c7", Dark: "#444441"}),
		you: active.Bold(true), model: good.Bold(true),
	}
}

// Paint is what sets the terminal to theme t before the screen is drawn,
// and Unpaint what gives the terminal back its own colours after: OSC 10
// and 11 set the text and the background, OSC 110 and 111 reset them, in
// every terminal that reads them, and nothing in one that does not.
func Paint(t Theme) string {
	if t != Light {
		return ""
	}
	return "\x1b]10;#1f1f1d\x07\x1b]11;#ffffff\x07"
}

// Unpaint gives the terminal its own colours back.
func Unpaint(t Theme) string {
	if t != Light {
		return ""
	}
	return "\x1b]110\x07\x1b]111\x07"
}
