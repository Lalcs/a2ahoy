package presenter

import (
	"fmt"
	"io"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// PrintTaskPushConfig writes a formatted display of a single TaskPushConfig.
func PrintTaskPushConfig(w io.Writer, config *a2a.TaskPushConfig) error {
	fmt.Fprintf(w, "%s\n", styledHeader("=== Push Notification Config ==="))
	fmt.Fprintf(w, "%s %s\n", styledLabel("Task ID:  "), string(config.TaskID))

	printPushConfig(w, &config.Config)

	if config.Tenant != "" {
		fmt.Fprintf(w, "%s %s\n", styledLabel("Tenant:   "), config.Tenant)
	}

	return nil
}

// PrintTaskPushConfigs writes a formatted display of a list of TaskPushConfig entries.
func PrintTaskPushConfigs(w io.Writer, configs []*a2a.TaskPushConfig) error {
	if len(configs) == 0 {
		fmt.Fprintln(w, "No push notification configs found.")
		return nil
	}

	fmt.Fprintf(w, "%s\n", styledHeader(fmt.Sprintf("=== Push Notification Configs (%d) ===", len(configs))))

	for _, cfg := range configs {
		id := cfg.Config.ID
		if id == "" {
			id = "(none)"
		}
		fmt.Fprintf(w, "  %s %-20s %s %-40s %s %s\n",
			styledLabel("ID:"), id,
			styledLabel("URL:"), cfg.Config.URL,
			styledLabel("Task:"), string(cfg.TaskID))
	}

	return nil
}

// PrintTaskPushConfigDeleted writes a success message for a deleted push
// notification configuration.
func PrintTaskPushConfigDeleted(w io.Writer, taskID, configID string) error {
	fmt.Fprintf(w, "%s Push notification config %q deleted from task %q.\n",
		styledSuccess("OK"), configID, taskID)
	return nil
}

func printPushConfig(w io.Writer, cfg *a2a.PushConfig) {
	if cfg.ID != "" {
		fmt.Fprintf(w, "%s %s\n", styledLabel("Config ID:"), cfg.ID)
	}
	fmt.Fprintf(w, "%s %s\n", styledLabel("URL:      "), cfg.URL)
	if cfg.Token != "" {
		fmt.Fprintf(w, "%s %s\n", styledLabel("Token:    "), cfg.Token)
	}
	if cfg.Auth != nil {
		fmt.Fprintf(w, "%s %s %s\n", styledLabel("Auth:     "), cfg.Auth.Scheme, cfg.Auth.Credentials)
	}
}
