package style

import (
	"os"
	"strings"
)

// ANSI escape codes
const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Dim   = "\033[2m"
)

// ANSI 256-color codes (more compatible than RGB)
const (
	Purple256 = "\033[38;5;141m" // Regent Purple #9B59D0
	Blue256   = "\033[38;5;69m"  // Royal Blue #5B7FFF
	Green256  = "\033[38;5;42m"  // Emerald #10B981
	Amber256  = "\033[38;5;214m" // Amber #F59E0B
	Red256    = "\033[38;5;196m" // Rose #EF4444
	Gray256   = "\033[38;5;103m" // Slate #64748B
)

var noColor = os.Getenv("NO_COLOR") != ""

// stylize applies a color/style if NO_COLOR is not set
func stylize(style, text string) string {
	if noColor || text == "" {
		return text
	}
	return style + text + Reset
}

// Brand returns "re_gent" in purple (for brand moments like version, init header)
func Brand(text string) string {
	// Default to "re_gent" if no text provided
	if text == "" {
		text = "re_gent"
	}
	return stylize(Purple256, text)
}

// Title returns bold text with no color (for headings)
func Title(text string) string {
	return stylize(Bold, text)
}

// Label returns text in blue (for label: value pairs)
func Label(text string) string {
	return stylize(Blue256, text)
}

// Value returns plain text (for values in label: value pairs)
func Value(text string) string {
	return text
}

// DimText returns dimmed/faint text (for timestamps, metadata, secondary info)
func DimText(text string) string {
	return stylize(Dim, text)
}

// Success returns green checkmark + plain text
func Success(text string) string {
	check := stylize(Green256, "✓")
	if text == "" {
		return check
	}
	return check + " " + text
}

// Error returns red X + plain text
func Error(text string) string {
	x := stylize(Red256, "✗")
	if text == "" {
		return x
	}
	return x + " " + text
}

// Warning returns amber warning symbol + plain text
func Warning(text string) string {
	warn := stylize(Amber256, "⚠")
	if text == "" {
		return warn
	}
	return warn + " " + text
}

// Hash returns plain text (hashes must be pipeable)
func Hash(text string) string {
	return text
}

// Timestamp returns dimmed text (for timestamps)
func Timestamp(text string) string {
	return stylize(Dim, text)
}

// Divider returns a dimmed divider line
func Divider(text string) string {
	if text == "" {
		text = strings.Repeat("━", 60)
	}
	return stylize(Dim, text)
}

// DividerFull returns a full-width dimmed divider (for major sections)
func DividerFull(text string) string {
	if text == "" {
		text = strings.Repeat("━", 70)
	}
	return stylize(Dim, text)
}

// SectionHeader returns a divider with bold text (e.g., "━━━ Step 1/3: Title")
func SectionHeader(text string) string {
	prefix := stylize(Dim, "━━━")
	return prefix + " " + stylize(Bold, text)
}

// SectionDivider returns a styled section separator (for show command)
func SectionDivider(text string) string {
	left := stylize(Dim, "═══")
	right := stylize(Dim, "═══")
	return left + " " + text + " " + right
}

// Prompt returns styled prompt text (question is plain, options are dimmed)
func Prompt(question, options string) string {
	if options == "" {
		return question
	}
	return question + " " + stylize(Dim, options)
}
