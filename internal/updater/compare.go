package updater

import "golang.org/x/mod/semver"

// devVersion mirrors the sentinel used by internal/version. We duplicate the
// constant here (rather than importing the version package) so that updater
// has zero dependencies on application packages, which keeps it easy to test
// and reuse.
const devVersion = "dev"

// Action describes the outcome of a version comparison and indicates what
// the update command should do next.
type Action int

const (
	// ActionUpToDate means the running binary already matches the latest
	// release.
	ActionUpToDate Action = iota
	// ActionUpdate means a newer release is available and should be
	// installed.
	ActionUpdate
	// ActionDevelopment means the running binary is a development build
	// (Version == "dev"). The latest release should always be installed.
	ActionDevelopment
	// ActionAhead means the running binary's version is higher than the
	// latest published release. This typically happens when a developer
	// has rebuilt locally with a custom -ldflags injection. No action is
	// taken.
	ActionAhead
	// ActionForceReinstall means --force was set on the command line and
	// the latest release should be reinstalled regardless of the version
	// comparison.
	ActionForceReinstall
	// ActionInvalidLatest means the GitHub release tag could not be parsed
	// as a semantic version. The user must investigate manually.
	ActionInvalidLatest
)

// String returns a short identifier for the action, useful for logs and
// tests. The presenter package owns the user-facing description.
func (a Action) String() string {
	switch a {
	case ActionUpToDate:
		return "up-to-date"
	case ActionUpdate:
		return "update"
	case ActionDevelopment:
		return "development"
	case ActionAhead:
		return "ahead"
	case ActionForceReinstall:
		return "force-reinstall"
	case ActionInvalidLatest:
		return "invalid-latest"
	default:
		return "unknown"
	}
}

// Decision is the resolved outcome of comparing the current version to a
// remote release tag.
type Decision struct {
	Action  Action
	Current string
	Latest  string
	Reason  string
}

// ShouldInstall reports whether the decision implies that the binary should
// be replaced (assuming --check-only is not set).
func (d Decision) ShouldInstall() bool {
	switch d.Action {
	case ActionUpdate, ActionDevelopment, ActionForceReinstall:
		return true
	default:
		return false
	}
}

// Decide compares the running binary's version (currentVersion) to the
// latest release tag (latestTag) and returns the action that should be
// taken. The force argument short-circuits the comparison and always
// requests a reinstall.
//
// The function performs no I/O and is safe to call repeatedly.
func Decide(currentVersion, latestTag string, force bool) Decision {
	d := Decision{
		Current: currentVersion,
		Latest:  latestTag,
	}

	if force {
		d.Action = ActionForceReinstall
		d.Reason = "force flag set"
		return d
	}

	if currentVersion == devVersion {
		d.Action = ActionDevelopment
		d.Reason = "current is a development build"
		return d
	}

	if !semver.IsValid(latestTag) {
		d.Action = ActionInvalidLatest
		d.Reason = "latest release tag is not a valid semver"
		return d
	}

	cur := normalizeForCompare(currentVersion)
	if !semver.IsValid(cur) {
		// An unparseable current version is treated as a development
		// build so the user can recover by reinstalling.
		d.Action = ActionDevelopment
		d.Reason = "current version is not a valid semver"
		return d
	}

	switch cmp := semver.Compare(cur, latestTag); {
	case cmp < 0:
		d.Action = ActionUpdate
		d.Reason = "newer release available"
	case cmp == 0:
		d.Action = ActionUpToDate
		d.Reason = "current matches latest"
	default:
		d.Action = ActionAhead
		d.Reason = "current is ahead of latest release"
	}
	return d
}

// normalizeForCompare prepends a leading "v" if missing. The semver package
// from golang.org/x/mod requires the prefix.
func normalizeForCompare(v string) string {
	if len(v) > 0 && v[0] != 'v' {
		return "v" + v
	}
	return v
}
