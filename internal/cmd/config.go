package cmd

import "github.com/a2aproject/a2a-go/v2/a2a"

// buildSendConfig returns a SendMessageConfig populated from the given
// accepted output modes and return-immediately flag, or nil if neither is
// specified. Returning nil ensures the "configuration" key is omitted
// from the JSON-RPC request when no flags are set, matching the current
// zero-flag behaviour.
func buildSendConfig(acceptedOutputModes []string, returnImmediately bool) *a2a.SendMessageConfig {
	if len(acceptedOutputModes) == 0 && !returnImmediately {
		return nil
	}
	return &a2a.SendMessageConfig{
		AcceptedOutputModes: acceptedOutputModes,
		ReturnImmediately:   returnImmediately,
	}
}
