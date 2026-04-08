package chat

import "strings"

// SlashCmd is a parsed slash command entered by the user.
// Name is always lowercased so callers can switch on it directly.
// Arg holds the remainder of the line after the command name, trimmed
// of surrounding whitespace. Empty when no argument was supplied.
type SlashCmd struct {
	Name string // e.g., "new", "exit", "get", "cancel", "help"
	Arg  string // optional argument, e.g., a task id for "/get <id>"
}

// ParseInputLine trims and classifies a raw input line.
//
// It returns:
//   - trimmed: the input with surrounding whitespace removed
//   - isSlash: true iff the trimmed input starts with a "/"
//   - sc:      the parsed SlashCmd (zero value when !isSlash)
//
// This function is deliberately pure so both the TUI and the simple-mode
// runner share exactly the same parsing rules, and so it can be
// table-driven unit tested without any I/O.
func ParseInputLine(line string) (trimmed string, isSlash bool, sc SlashCmd) {
	trimmed = strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return trimmed, false, SlashCmd{}
	}
	rest := strings.TrimPrefix(trimmed, "/")
	name, arg, _ := strings.Cut(rest, " ")
	return trimmed, true, SlashCmd{
		Name: strings.ToLower(name),
		Arg:  strings.TrimSpace(arg),
	}
}
