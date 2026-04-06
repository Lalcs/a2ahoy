package presenter

import (
	"fmt"
	"io"
	"strings"

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
