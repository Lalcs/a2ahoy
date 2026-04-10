package cmd

import "github.com/a2aproject/a2a-go/v2/a2a"

// buildSendConfig returns a SendMessageConfig populated from the given
// accepted output modes, or nil if no modes are specified. Returning nil
// ensures the "configuration" key is omitted from the JSON-RPC request
// when no flags are set, matching the current zero-flag behaviour.
func buildSendConfig(acceptedOutputModes []string) *a2a.SendMessageConfig {
	if len(acceptedOutputModes) == 0 {
		return nil
	}
	return &a2a.SendMessageConfig{
		AcceptedOutputModes: acceptedOutputModes,
	}
}
