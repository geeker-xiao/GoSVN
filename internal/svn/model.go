// Package svn wraps the Subversion command-line client.
//
// It talks to the system "svn" binary through os/exec, preferring --xml
// output parsed with encoding/xml. It never links against libsvn (no cgo).
//
// The package is UI-agnostic: it exposes plain data types and error values
// and depends on nothing from internal/ui.
package svn

// ItemStatus is the working-copy status of a single item, taken directly from
// the "item" attribute of <wc-status> in svn status --xml output.
type ItemStatus string

// Working-copy item statuses reported by svn.
const (
	StatusNone        ItemStatus = "none"
	StatusNormal      ItemStatus = "normal"
	StatusAdded       ItemStatus = "added"
	StatusMissing     ItemStatus = "missing"
	StatusDeleted     ItemStatus = "deleted"
	StatusReplaced    ItemStatus = "replaced"
	StatusModified    ItemStatus = "modified"
	StatusConflict    ItemStatus = "conflicted"
	StatusIgnored     ItemStatus = "ignored"
	StatusObstructed  ItemStatus = "obstructed"
	StatusUnversioned ItemStatus = "unversioned"
	StatusIncomplete  ItemStatus = "incomplete"
	StatusMerged      ItemStatus = "merged"
)

// Code returns the single-letter column used by `svn status` and TortoiseSVN:
// M=modified, A=added, D=deleted, R=replaced, C=conflicted, !=missing,
// ?=unversioned, I=ignored, ~=obstructed, X=externals, blank=normal/clean.
func (s ItemStatus) Code() string {
	switch s {
	case StatusModified:
		return "M"
	case StatusAdded:
		return "A"
	case StatusDeleted:
		return "D"
	case StatusReplaced:
		return "R"
	case StatusConflict:
		return "C"
	case StatusMissing:
		return "!"
	case StatusUnversioned:
		return "?"
	case StatusIgnored:
		return "I"
	case StatusObstructed:
		return "~"
	case StatusIncomplete:
		return "!"
	case StatusMerged:
		return "G"
	default:
		return " "
	}
}

// Description returns a short human-readable label for the status.
func (s ItemStatus) Description() string {
	switch s {
	case StatusModified:
		return "modified"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusReplaced:
		return "replaced"
	case StatusConflict:
		return "conflicted"
	case StatusMissing:
		return "missing"
	case StatusUnversioned:
		return "unversioned"
	case StatusIgnored:
		return "ignored"
	case StatusObstructed:
		return "obstructed"
	case StatusIncomplete:
		return "incomplete"
	case StatusMerged:
		return "merged"
	case StatusNormal:
		return "normal"
	default:
		return string(s)
	}
}

// IsInteresting reports whether the entry represents a change worth showing
// in a commit dialog (anything not clean/normal/ignored).
func (s ItemStatus) IsInteresting() bool {
	switch s {
	case StatusNormal, StatusIgnored, StatusNone:
		return false
	default:
		return true
	}
}

// IsVersioned reports whether the item is under version control.
func (s ItemStatus) IsVersioned() bool {
	switch s {
	case StatusUnversioned, StatusIgnored, StatusNone:
		return false
	default:
		return true
	}
}

// CommitInfo describes a commit recorded for an entry or a log revision.
type CommitInfo struct {
	Revision int
	Author   string
	Date     string // ISO-8601 as reported by svn
}

// StatusEntry is one row of working-copy status.
type StatusEntry struct {
	Path     string // path relative to the status target
	Item     ItemStatus
	Props    string // "none" or "modified"
	Revision int    // working revision
	Commit   CommitInfo
}

// LogPath is one changed path inside a log entry (svn log -v).
type LogPath struct {
	Action   string // M, A, D, R
	Path     string
	Kind     string // "file" or "dir"
	TextMods bool
	PropMods bool
}

// LogEntry is one revision in the history.
type LogEntry struct {
	Revision int
	Author   string
	Date     string
	Message  string
	Paths    []LogPath
}

// Info is the result of `svn info --xml` for a working-copy target.
type Info struct {
	Path        string
	Kind        string // "dir" or "file"
	Revision    int
	URL         string
	RelativeURL string
	RepoRoot    string
	RepoUUID    string
	WCRoot      string
	Schedule    string
	Depth       string
	Commit      CommitInfo
}

// CommitResult is the outcome of `svn commit`.
type CommitResult struct {
	Revision int // 0 if not parsed from output
	Author   string
	Date     string
	Output   string // raw command output
}

// UpdateResult is the outcome of `svn update`.
type UpdateResult struct {
	Revision   int // 0 if not parsed
	Output     string
	Added      []string
	Updated    []string
	Deleted    []string
	Conflicted []string
	Merged     []string
}
