package cmd

import (
	"fmt"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/spf13/cobra"
)

// buildSendConfig returns a SendMessageConfig populated from the given
// parameters, or nil if nothing is specified. Returning nil ensures the
// "configuration" key is omitted from the JSON-RPC request when no flags
// are set, matching the current zero-flag behaviour.
func buildSendConfig(acceptedOutputModes []string, returnImmediately bool, historyLength *int, pushConfig *a2a.PushConfig) *a2a.SendMessageConfig {
	if len(acceptedOutputModes) == 0 && !returnImmediately && historyLength == nil && pushConfig == nil {
		return nil
	}
	return &a2a.SendMessageConfig{
		AcceptedOutputModes: acceptedOutputModes,
		ReturnImmediately:   returnImmediately,
		HistoryLength:       historyLength,
		PushConfig:          pushConfig,
	}
}

// buildPushConfig returns a PushConfig from the given URL and token,
// or nil when pushURL is empty. This enables the inline push
// notification configuration on the send command.
func buildPushConfig(pushURL, pushToken string) *a2a.PushConfig {
	if pushURL == "" {
		return nil
	}
	return &a2a.PushConfig{
		URL:   pushURL,
		Token: pushToken,
	}
}

// getHistoryLength returns the --history-length flag value as a pointer,
// or nil when the flag was not explicitly set. This distinguishes "flag
// omitted" (server default) from "explicit --history-length=0".
func getHistoryLength(cmd *cobra.Command) *int {
	if !cmd.Flags().Changed(flagNameHistoryLength) {
		return nil
	}
	h, _ := cmd.Flags().GetInt(flagNameHistoryLength)
	return &h
}

// parseMetadata converts a slice of "KEY=VALUE" strings into a
// map[string]any suitable for request Metadata fields. Returns an error
// if any entry is missing the "=" separator.
func parseMetadata(ss []string) (map[string]any, error) {
	if len(ss) == 0 {
		return nil, nil
	}
	m := make(map[string]any, len(ss))
	for _, s := range ss {
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --metadata format %q: expected KEY=VALUE", s)
		}
		m[k] = v
	}
	return m, nil
}

// toTaskIDs converts a string slice to a2a.TaskID slice.
// Returns nil when the input is empty so the field is omitted from JSON.
func toTaskIDs(ss []string) []a2a.TaskID {
	if len(ss) == 0 {
		return nil
	}
	ids := make([]a2a.TaskID, len(ss))
	for i, s := range ss {
		ids[i] = a2a.TaskID(s)
	}
	return ids
}
