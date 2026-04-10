package presenter

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Lalcs/a2ahoy/internal/cardcheck"
	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintAgentCard writes a formatted display of an AgentCard.
func PrintAgentCard(w io.Writer, card *a2a.AgentCard) error {
	_, _ = fmt.Fprintf(w, "%s\n", styledHeader("=== Agent Card ==="))
	_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("Name:       "), styledSuccess(card.Name))
	_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("Description:"), card.Description)
	_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("Version:    "), card.Version)

	if card.Provider != nil {
		_, _ = fmt.Fprintf(w, "%s %s (%s)\n", styledLabel("Provider:   "), card.Provider.Org, card.Provider.URL)
	}

	if card.DocumentationURL != "" {
		_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("Docs:       "), card.DocumentationURL)
	}

	if card.IconURL != "" {
		_, _ = fmt.Fprintf(w, "%s %s\n", styledLabel("Icon:       "), card.IconURL)
	}

	// Capabilities
	_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider("--- Capabilities ---"))
	_, _ = fmt.Fprintf(w, "%s %v\n", styledLabel("Streaming:         "), card.Capabilities.Streaming)
	_, _ = fmt.Fprintf(w, "%s %v\n", styledLabel("Push Notifications:"), card.Capabilities.PushNotifications)
	_, _ = fmt.Fprintf(w, "%s %v\n", styledLabel("Extended Card:     "), card.Capabilities.ExtendedAgentCard)

	// Interfaces
	if len(card.SupportedInterfaces) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider("--- Interfaces ---"))
		for _, iface := range card.SupportedInterfaces {
			_, _ = fmt.Fprintf(w, "  %s %s (v%s)\n", styledTag(fmt.Sprintf("[%s]", iface.ProtocolBinding)), iface.URL, iface.ProtocolVersion)
		}
	}

	// Default modes
	if len(card.DefaultInputModes) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider("--- Default Input Modes ---"))
		_, _ = fmt.Fprintf(w, "  %s\n", strings.Join(card.DefaultInputModes, ", "))
	}
	if len(card.DefaultOutputModes) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider("--- Default Output Modes ---"))
		_, _ = fmt.Fprintf(w, "  %s\n", strings.Join(card.DefaultOutputModes, ", "))
	}

	// Skills
	if len(card.Skills) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- Skills (%d) ---", len(card.Skills))))
		for i, skill := range card.Skills {
			_, _ = fmt.Fprintf(w, "  %s %s (id: %s)\n", styledTag(fmt.Sprintf("[%d]", i+1)), skill.Name, skill.ID)
			if skill.Description != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Description:"), skill.Description)
			}
			if len(skill.Tags) > 0 {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Tags:"), strings.Join(skill.Tags, ", "))
			}
			if len(skill.Examples) > 0 {
				_, _ = fmt.Fprintf(w, "      %s\n", styledLabel("Examples:"))
				for _, ex := range skill.Examples {
					_, _ = fmt.Fprintf(w, "        - %s\n", ex)
				}
			}
		}
	}

	// Security Schemes
	if len(card.SecuritySchemes) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- Security Schemes (%d) ---", len(card.SecuritySchemes))))
		printSecuritySchemes(w, card.SecuritySchemes)
	}

	// Security Requirements
	if len(card.SecurityRequirements) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider("--- Security Requirements ---"))
		for _, req := range card.SecurityRequirements {
			parts := make([]string, 0, len(req))
			for name, scopes := range req {
				if len(scopes) > 0 {
					parts = append(parts, fmt.Sprintf("%s(%s)", name, strings.Join(scopes, ", ")))
				} else {
					parts = append(parts, string(name))
				}
			}
			_, _ = fmt.Fprintf(w, "  %s\n", strings.Join(parts, " AND "))
		}
	}

	// Signatures
	if len(card.Signatures) > 0 {
		_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider(fmt.Sprintf("--- Signatures (%d) ---", len(card.Signatures))))
		for i, sig := range card.Signatures {
			_, _ = fmt.Fprintf(w, "  %s\n", styledTag(fmt.Sprintf("[%d]", i+1)))
			_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Protected:"), sig.Protected)
			_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Signature:"), sig.Signature)
			if len(sig.Header) > 0 {
				headerParts := make([]string, 0, len(sig.Header))
				for k, v := range sig.Header {
					headerParts = append(headerParts, fmt.Sprintf("%s=%v", k, v))
				}
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Header:   "), strings.Join(headerParts, ", "))
			}
		}
	}

	return nil
}

// printSecuritySchemes renders the SecuritySchemes map in a deterministic
// order by sorting scheme names alphabetically.
func printSecuritySchemes(w io.Writer, schemes a2a.NamedSecuritySchemes) {
	// Sort scheme names for deterministic output.
	names := make([]string, 0, len(schemes))
	for name := range schemes {
		names = append(names, string(name))
	}
	sort.Strings(names)

	for _, name := range names {
		scheme := schemes[a2a.SecuritySchemeName(name)]
		switch s := scheme.(type) {
		case a2a.APIKeySecurityScheme:
			_, _ = fmt.Fprintf(w, "  %s %s\n", styledTag(fmt.Sprintf("[%s]", name)), "API Key")
			_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Name:    "), s.Name)
			_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Location:"), s.Location)
			if s.Description != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Desc:    "), s.Description)
			}
		case a2a.HTTPAuthSecurityScheme:
			_, _ = fmt.Fprintf(w, "  %s %s\n", styledTag(fmt.Sprintf("[%s]", name)), "HTTP Auth")
			_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Scheme:      "), s.Scheme)
			if s.BearerFormat != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Bearer Format:"), s.BearerFormat)
			}
			if s.Description != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Desc:        "), s.Description)
			}
		case a2a.OpenIDConnectSecurityScheme:
			_, _ = fmt.Fprintf(w, "  %s %s\n", styledTag(fmt.Sprintf("[%s]", name)), "OpenID Connect")
			_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("URL: "), s.OpenIDConnectURL)
			if s.Description != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Desc:"), s.Description)
			}
		case a2a.OAuth2SecurityScheme:
			_, _ = fmt.Fprintf(w, "  %s %s\n", styledTag(fmt.Sprintf("[%s]", name)), "OAuth2")
			if s.Description != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Desc:        "), s.Description)
			}
			if s.Oauth2MetadataURL != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Metadata URL:"), s.Oauth2MetadataURL)
			}
			printOAuth2Flow(w, s.Flows)
		case a2a.MutualTLSSecurityScheme:
			_, _ = fmt.Fprintf(w, "  %s %s\n", styledTag(fmt.Sprintf("[%s]", name)), "Mutual TLS")
			if s.Description != "" {
				_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Desc:"), s.Description)
			}
		default:
			_, _ = fmt.Fprintf(w, "  %s %s\n", styledTag(fmt.Sprintf("[%s]", name)), "Unknown")
		}
	}
}

// printOAuth2Flow renders the OAuth2 flow details indented under the scheme.
func printOAuth2Flow(w io.Writer, flow a2a.OAuthFlows) {
	switch f := flow.(type) {
	case a2a.AuthorizationCodeOAuthFlow:
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Flow:        "), "Authorization Code")
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Auth URL:    "), f.AuthorizationURL)
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Token URL:   "), f.TokenURL)
	case a2a.ClientCredentialsOAuthFlow:
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Flow:        "), "Client Credentials")
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Token URL:   "), f.TokenURL)
	case a2a.DeviceCodeOAuthFlow:
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Flow:        "), "Device Code")
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Device URL:  "), f.DeviceAuthorizationURL)
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Token URL:   "), f.TokenURL)
	case a2a.ImplicitOAuthFlow:
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Flow:        "), "Implicit")
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Auth URL:    "), f.AuthorizationURL)
	case a2a.PasswordOAuthFlow:
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Flow:        "), "Password")
		_, _ = fmt.Fprintf(w, "      %s %s\n", styledLabel("Token URL:   "), f.TokenURL)
	}
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
	_, _ = fmt.Fprintf(w, "\n%s\n", styledDivider(title))

	for i, iss := range result.Issues {
		_, _ = fmt.Fprintf(w, "  %s %s\n", styledIssueLevel(iss.Level), iss.Code)
		if iss.Message != "" {
			_, _ = fmt.Fprintf(w, "          %s\n", iss.Message)
		}
		if iss.Field != "" {
			_, _ = fmt.Fprintf(w, "          %s %s\n", styledLabel("field:"), iss.Field)
		}
		if iss.Hint != "" {
			_, _ = fmt.Fprintf(w, "          %s  %s\n", styledLabel("hint:"), iss.Hint)
		}
		// Blank line between issues, but not after the last.
		if i < len(result.Issues)-1 {
			_, _ = fmt.Fprintln(w)
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
		_, _ = fmt.Fprintf(w, "a2ahoy card: %s: %s %s\n", iss.Level, iss.Code, field)
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
