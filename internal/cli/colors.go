package cli

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// isTerminal returns true when stdout is connected to a terminal.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// colorize wraps text with an ANSI escape code when tty is true.
func colorize(tty bool, code, text string) string {
	if !tty {
		return text
	}
	return fmt.Sprintf("\033[%sm%s\033[0m", code, text)
}

func green(tty bool, s string) string  { return colorize(tty, "32", s) }
func red(tty bool, s string) string    { return colorize(tty, "31", s) }
func yellow(tty bool, s string) string { return colorize(tty, "33", s) }
func bold(tty bool, s string) string   { return colorize(tty, "1", s) }
func dim(tty bool, s string) string    { return colorize(tty, "2", s) }

// jobColors is the palette used to color-code parallel job prefixes.
var jobColors = []string{"36", "33", "35", "32", "34", "91", "92", "93", "94", "95"}
