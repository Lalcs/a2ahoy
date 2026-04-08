package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/Lalcs/a2ahoy/internal/cardcheck"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintAgentCard writes a formatted display of an AgentCard.
func PrintAgentCard(w io.Writer, card *a2a.AgentCard) error {
	fmt.Fprintf(w, "%s\n", styledHeader("=== Agent Card ==="))
	fmt.Fprintf(w, "%s %s\n", styledLabel("Name:       "), styledSuccess(card.Name))
	fmt.Fprintf(w, "%s %s\n", styledLabel("Description:"), card.Description)
	fmt.Fprintf(w, "%s %s\n", styledLabel("Version:    "), card.Version)

	if card.Provider != nil {
		fmt.Fprintf(w, "%s %s (%s)\n", styledLabel("Provider:   "), card.Provider.Org, card.Provider.URL)
	}

	if card.DocumentationURL != "" {
		fmt.Fprintf(w, "%s %s\n", styledLabel("Docs:       "), card.DocumentationURL)
	}

	// Capabilities
	fmt.Fprintf(w, "\n%s\n", styledDivider("--- Capabilities ---"))
	fmt.Fprintf(w, "%s %v\n", styledLabel("Streaming:         "), card.Capabilities.Streaming)
	fmt.Fprintf(w, "%s %v\n", styledLabel("Push Notifications:"), card.Capabilities.PushNotifications)
	fmt.Fprintf(w, "%s %v\n", styledLabel("Extended Card:     "), card.Capabilities.ExtendedAgentCard)

	// Interfaces
	if len(card.SupportedInterfaces) > 0 {
		fmt.Fprintf(w, "\n%s\n", styledDivider("--- Interfaces ---"))
		for _, iface := range card.SupportedInterfaces {
			fmt.Fprintf(w, "  %s %s (v%s)\n", styledTag(fmt.Sprintf("[%s]", iface.ProtocolBinding)), iface.URL, iface.ProtocolVersion)
		}
	}

	// Default modes
	if len(card.DefaultInputModes) > 0 {
		fmt.Fprintf(w, "\n%s\n", styledDivider("--- Default Input Modes ---"))
		fmt.Fprintf(w, "  %s\n", strings.Join(card.DefaultInputModes, ", "))
	}
	if len(card.DefaultOutputModes) > 0 {
		fmt.Fprintf(w, "\n%s\n", styledDivider("--- Default Output Modes ---"))
		fmt.Fprintf(w, "  %s\n", strings.Join(card.DefaultOutputModes, ", "))
	}

	// Skills
	if len(card.Skills) > 0 {
		fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- Skills (%d) ---", len(card.Skills))))
		for i, skill := range card.Skills {
			fmt.Fprintf(w, "  %s %s (id: %s)\n", styledTag(fmt.Sprintf("[%d]", i+1)), skill.Name, skill.ID)
			if skill.Description != "" {
				fmt.Fprintf(w, "      %s %s\n", styledLabel("Description:"), skill.Description)
			}
			if len(skill.Tags) > 0 {
				fmt.Fprintf(w, "      %s %s\n", styledLabel("Tags:"), strings.Join(skill.Tags, ", "))
			}
			if len(skill.Examples) > 0 {
				fmt.Fprintf(w, "      %s\n", styledLabel("Examples:"))
				for _, ex := range skill.Examples {
					fmt.Fprintf(w, "        - %s\n", ex)
				}
			}
		}
	}

	return nil
}

// PrintValidation renders a "--- Validation ---" section describing the
// issues in result. Each issue is formatted as a multi-line block with
// level tag, code, message, optional field path, and optional hint.
//
// When result has no issues, this function writes nothing — the card
// display stays minimal for healthy cards. Callers should therefore call
// PrintValidation unconditionally after PrintAgentCard; the empty-result
// case is handled here, not by the caller.
func PrintValidation(w io.Writer, result cardcheck.Result) {
	if !result.HasIssues() {
		return
	}

	title := fmt.Sprintf("--- Validation (%s) ---", formatValidationCounts(result))
	fmt.Fprintf(w, "\n%s\n", styledDivider(title))

	for i, iss := range result.Issues {
		fmt.Fprintf(w, "  %s %s\n", styledIssueLevel(iss.Level), iss.Code)
		if iss.Message != "" {
			fmt.Fprintf(w, "          %s\n", iss.Message)
		}
		if iss.Field != "" {
			fmt.Fprintf(w, "          %s %s\n", styledLabel("field:"), iss.Field)
		}
		if iss.Hint != "" {
			fmt.Fprintf(w, "          %s  %s\n", styledLabel("hint:"), iss.Hint)
		}
		// Blank line between issues, but not after the last.
		if i < len(result.Issues)-1 {
			fmt.Fprintln(w)
		}
	}
}

// PrintValidationSummary writes a one-line-per-issue summary suitable for
// stderr when the card is being rendered as JSON to stdout. The format is
// `a2ahoy card: <level>: <CODE> <field>` for each issue. No output is
// produced for an empty Result, so callers can invoke it unconditionally.
func PrintValidationSummary(w io.Writer, result cardcheck.Result) {
	if !result.HasIssues() {
		return
	}
	for _, iss := range result.Issues {
		field := iss.Field
		if field == "" {
			field = "-"
		}
		fmt.Fprintf(w, "a2ahoy card: %s: %s %s\n", iss.Level, iss.Code, field)
	}
}

// styledIssueLevel returns the coloured bracket tag displayed at the
// start of each validation issue line. ERROR is red, WARNING is yellow,
// INFO uses the same blue as structural labels via styledTag. The fixed
// width helps the wrapped message text align consistently under each tag.
func styledIssueLevel(level cardcheck.Level) string {
	switch level {
	case cardcheck.LevelError:
		return styledError("[ERROR]")
	case cardcheck.LevelWarning:
		return styledWarning("[WARN] ")
	case cardcheck.LevelInfo:
		return styledTag("[INFO] ")
	default:
		return "[?]    "
	}
}

// formatValidationCounts returns a human-readable summary such as
// "1 warning" or "2 errors, 3 warnings". Only non-zero levels appear.
// The order is errors → warnings → infos so the most severe counts
// come first.
func formatValidationCounts(result cardcheck.Result) string {
	parts := make([]string, 0, 3)
	if n := result.Count(cardcheck.LevelError); n > 0 {
		parts = append(parts, pluralize(n, "error", "errors"))
	}
	if n := result.Count(cardcheck.LevelWarning); n > 0 {
		parts = append(parts, pluralize(n, "warning", "warnings"))
	}
	if n := result.Count(cardcheck.LevelInfo); n > 0 {
		parts = append(parts, pluralize(n, "info", "infos"))
	}
	return strings.Join(parts, ", ")
}

// pluralize formats a count with its singular or plural noun.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
