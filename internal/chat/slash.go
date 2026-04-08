package chat

import "strings"

// SuggestionItem describes a single slash-command entry used by both
// the TUI autocomplete dropdown and the `/help` output.
type SuggestionItem struct {
	Name string // literal command including leading slash, e.g., "/new"
	Help string // human-readable one-liner
}

// AllSuggestions is the canonical, ordered list of supported slash
// commands. Order determines the default display order in the TUI
// dropdown and in `/help` output.
var AllSuggestions = []SuggestionItem{
	{Name: "/new", Help: "Start a new conversation (reset task/context)"},
	{Name: "/get", Help: "Show current task details (optional: /get <task-id>)"},
	{Name: "/cancel", Help: "Cancel current task (optional: /cancel <task-id>)"},
	{Name: "/help", Help: "Show this help"},
	{Name: "/exit", Help: "Exit the chat"},
	{Name: "/quit", Help: "Exit the chat"},
}

// FilterSuggestions returns the subset of AllSuggestions whose Name has
// the given input as a case-insensitive prefix. Used by the TUI to
// populate its dropdown as the user types.
//
// Empty input and inputs that do not start with "/" return nil so the
// caller can simply hide the dropdown.
func FilterSuggestions(input string) []SuggestionItem {
	if input == "" || !strings.HasPrefix(input, "/") {
		return nil
	}
	lower := strings.ToLower(input)
	var out []SuggestionItem
	for _, s := range AllSuggestions {
		if strings.HasPrefix(strings.ToLower(s.Name), lower) {
			out = append(out, s)
		}
	}
	return out
}
