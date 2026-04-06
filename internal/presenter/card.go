package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintAgentCard writes a formatted display of an AgentCard.
func PrintAgentCard(w io.Writer, card *a2a.AgentCard) error {
	fmt.Fprintf(w, "=== Agent Card ===\n")
	fmt.Fprintf(w, "Name:        %s\n", card.Name)
	fmt.Fprintf(w, "Description: %s\n", card.Description)
	fmt.Fprintf(w, "Version:     %s\n", card.Version)

	if card.Provider != nil {
		fmt.Fprintf(w, "Provider:    %s (%s)\n", card.Provider.Org, card.Provider.URL)
	}

	if card.DocumentationURL != "" {
		fmt.Fprintf(w, "Docs:        %s\n", card.DocumentationURL)
	}

	// Capabilities
	fmt.Fprintf(w, "\n--- Capabilities ---\n")
	fmt.Fprintf(w, "Streaming:          %v\n", card.Capabilities.Streaming)
	fmt.Fprintf(w, "Push Notifications: %v\n", card.Capabilities.PushNotifications)
	fmt.Fprintf(w, "Extended Card:      %v\n", card.Capabilities.ExtendedAgentCard)

	// Interfaces
	if len(card.SupportedInterfaces) > 0 {
		fmt.Fprintf(w, "\n--- Interfaces ---\n")
		for _, iface := range card.SupportedInterfaces {
			fmt.Fprintf(w, "  [%s] %s (v%s)\n", iface.ProtocolBinding, iface.URL, iface.ProtocolVersion)
		}
	}

	// Default modes
	if len(card.DefaultInputModes) > 0 {
		fmt.Fprintf(w, "\n--- Default Input Modes ---\n")
		fmt.Fprintf(w, "  %s\n", strings.Join(card.DefaultInputModes, ", "))
	}
	if len(card.DefaultOutputModes) > 0 {
		fmt.Fprintf(w, "\n--- Default Output Modes ---\n")
		fmt.Fprintf(w, "  %s\n", strings.Join(card.DefaultOutputModes, ", "))
	}

	// Skills
	if len(card.Skills) > 0 {
		fmt.Fprintf(w, "\n--- Skills (%d) ---\n", len(card.Skills))
		for i, skill := range card.Skills {
			fmt.Fprintf(w, "  [%d] %s (id: %s)\n", i+1, skill.Name, skill.ID)
			if skill.Description != "" {
				fmt.Fprintf(w, "      Description: %s\n", skill.Description)
			}
			if len(skill.Tags) > 0 {
				fmt.Fprintf(w, "      Tags: %s\n", strings.Join(skill.Tags, ", "))
			}
			if len(skill.Examples) > 0 {
				fmt.Fprintf(w, "      Examples:\n")
				for _, ex := range skill.Examples {
					fmt.Fprintf(w, "        - %s\n", ex)
				}
			}
		}
	}

	return nil
}
