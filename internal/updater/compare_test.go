package updater

import "testing"

func TestDecide(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		latest     string
		force      bool
		wantAction Action
	}{
		{
			name:       "force flag overrides everything",
			current:    "v1.2.3",
			latest:     "v1.2.3",
			force:      true,
			wantAction: ActionForceReinstall,
		},
		{
			name:       "force with dev version",
			current:    "dev",
			latest:     "v1.2.3",
			force:      true,
			wantAction: ActionForceReinstall,
		},
		{
			name:       "dev version always updates",
			current:    "dev",
			latest:     "v1.2.3",
			force:      false,
			wantAction: ActionDevelopment,
		},
		{
			name:       "older current means update",
			current:    "v1.2.2",
			latest:     "v1.2.3",
			force:      false,
			wantAction: ActionUpdate,
		},
		{
			name:       "matching versions are up-to-date",
			current:    "v1.2.3",
			latest:     "v1.2.3",
			force:      false,
			wantAction: ActionUpToDate,
		},
		{
			name:       "current ahead of latest",
			current:    "v1.2.4",
			latest:     "v1.2.3",
			force:      false,
			wantAction: ActionAhead,
		},
		{
			name:       "lexical ordering does not confuse comparison",
			current:    "v1.10.0",
			latest:     "v1.9.0",
			force:      false,
			wantAction: ActionAhead,
		},
		{
			name:       "missing v prefix is normalized for current",
			current:    "1.2.2",
			latest:     "v1.2.3",
			force:      false,
			wantAction: ActionUpdate,
		},
		{
			name:       "empty latest tag is invalid",
			current:    "v1.2.3",
			latest:     "",
			force:      false,
			wantAction: ActionInvalidLatest,
		},
		{
			name:       "garbage latest tag is invalid",
			current:    "v1.2.3",
			latest:     "snapshot-abc",
			force:      false,
			wantAction: ActionInvalidLatest,
		},
		{
			name:       "garbage current is treated as dev",
			current:    "snapshot-abc",
			latest:     "v1.2.3",
			force:      false,
			wantAction: ActionDevelopment,
		},
		{
			name:       "prerelease ordering: rc precedes release",
			current:    "v1.2.3-rc.1",
			latest:     "v1.2.3",
			force:      false,
			wantAction: ActionUpdate,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Decide(tt.current, tt.latest, tt.force)
			if d.Action != tt.wantAction {
				t.Errorf("Decide(%q, %q, %v).Action = %v, want %v",
					tt.current, tt.latest, tt.force, d.Action, tt.wantAction)
			}
			if d.Current != tt.current {
				t.Errorf("Decision.Current = %q, want %q", d.Current, tt.current)
			}
			if d.Latest != tt.latest {
				t.Errorf("Decision.Latest = %q, want %q", d.Latest, tt.latest)
			}
			if d.Reason == "" {
				t.Error("Decision.Reason should not be empty")
			}
		})
	}
}

func TestDecision_ShouldInstall(t *testing.T) {
	tests := []struct {
		action Action
		want   bool
	}{
		{ActionUpToDate, false},
		{ActionUpdate, true},
		{ActionDevelopment, true},
		{ActionAhead, false},
		{ActionForceReinstall, true},
		{ActionInvalidLatest, false},
	}
	for _, tt := range tests {
		t.Run(tt.action.String(), func(t *testing.T) {
			d := Decision{Action: tt.action}
			if got := d.ShouldInstall(); got != tt.want {
				t.Errorf("ShouldInstall() for %v = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestAction_String(t *testing.T) {
	tests := []struct {
		action Action
		want   string
	}{
		{ActionUpToDate, "up-to-date"},
		{ActionUpdate, "update"},
		{ActionDevelopment, "development"},
		{ActionAhead, "ahead"},
		{ActionForceReinstall, "force-reinstall"},
		{ActionInvalidLatest, "invalid-latest"},
		{Action(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.action.String(); got != tt.want {
				t.Errorf("Action.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeForCompare(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "v1.2.3"},
		{"1.2.3", "v1.2.3"},
		{"", ""},
		{"v", "v"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeForCompare(tt.input); got != tt.want {
				t.Errorf("normalizeForCompare(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
