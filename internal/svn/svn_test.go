package svn

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// readFixture loads a file from the repository-root testdata directory.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseStatusXML(t *testing.T) {
	entries, err := parseStatusXML(readFixture(t, "status.xml"))
	if err != nil {
		t.Fatalf("parseStatusXML: %v", err)
	}
	want := 7
	if len(entries) != want {
		t.Fatalf("got %d entries, want %d", len(entries), want)
	}

	cases := []struct {
		path     string
		item     ItemStatus
		code     string
		revision int
		commit   int
		props    string
	}{
		{"src/main.go", StatusModified, "M", 42, 41, "none"},
		{"src/new_file.go", StatusAdded, "A", 42, 41, "none"},
		{"docs/old.txt", StatusDeleted, "D", 42, 0, "none"},
		{"untracked.go", StatusUnversioned, "?", 0, 0, "none"},
		{"src/missing.go", StatusMissing, "!", 42, 0, "none"},
		{"src/conflict.go", StatusConflict, "C", 42, 0, "none"},
		{"src/中文文件.go", StatusModified, "M", 42, 0, "modified"},
	}
	byPath := map[string]StatusEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}
	for _, c := range cases {
		e, ok := byPath[c.path]
		if !ok {
			t.Errorf("missing entry for %q", c.path)
			continue
		}
		if e.Item != c.item {
			t.Errorf("%q item = %q, want %q", c.path, e.Item, c.item)
		}
		if e.Item.Code() != c.code {
			t.Errorf("%q code = %q, want %q", c.path, e.Item.Code(), c.code)
		}
		if e.Revision != c.revision {
			t.Errorf("%q revision = %d, want %d", c.path, e.Revision, c.revision)
		}
		if e.Commit.Revision != c.commit {
			t.Errorf("%q commit revision = %d, want %d", c.path, e.Commit.Revision, c.commit)
		}
		if e.Props != c.props {
			t.Errorf("%q props = %q, want %q", c.path, e.Props, c.props)
		}
	}
}

func TestParseStatusEmpty(t *testing.T) {
	entries, err := parseStatusXML(readFixture(t, "status_empty.xml"))
	if err != nil {
		t.Fatalf("parseStatusXML empty: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestParseLogXML(t *testing.T) {
	entries, err := parseLogXML(readFixture(t, "log.xml"))
	if err != nil {
		t.Fatalf("parseLogXML: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d log entries, want 2", len(entries))
	}
	first := entries[0]
	if first.Revision != 42 || first.Author != "bob" {
		t.Errorf("first entry = %+v", first)
	}
	if !strings.Contains(first.Message, "Implement feature X") ||
		!strings.Contains(first.Message, "Multi-line message") {
		t.Errorf("multi-line message not preserved: %q", first.Message)
	}
	if len(first.Paths) != 3 {
		t.Fatalf("first entry paths = %d, want 3", len(first.Paths))
	}
	wantActions := map[string]string{
		"/trunk/src/main.go":      "M",
		"/trunk/src/new_file.go":  "A",
		"/trunk/docs/old.txt":     "D",
	}
	for _, p := range first.Paths {
		if wantActions[p.Path] != p.Action {
			t.Errorf("path %q action = %q, want %q", p.Path, p.Action, wantActions[p.Path])
		}
	}
	if first.Paths[0].Kind != "file" || !first.Paths[0].TextMods {
		t.Errorf("first path kind/textmods = %+v", first.Paths[0])
	}
	// Second entry has no <paths>; must not panic and Paths stays nil/empty.
	if entries[1].Revision != 41 || entries[1].Message != "Initial commit" {
		t.Errorf("second entry = %+v", entries[1])
	}
	if len(entries[1].Paths) != 0 {
		t.Errorf("second entry should have no paths, got %d", len(entries[1].Paths))
	}
}

func TestParseLogEmpty(t *testing.T) {
	entries, err := parseLogXML(readFixture(t, "log_empty.xml"))
	if err != nil {
		t.Fatalf("parseLogXML empty: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestParseInfoXML(t *testing.T) {
	info, err := parseInfoXML(readFixture(t, "info.xml"))
	if err != nil {
		t.Fatalf("parseInfoXML: %v", err)
	}
	if info.Revision != 42 || info.Kind != "dir" {
		t.Errorf("revision/kind = %d/%q", info.Revision, info.Kind)
	}
	if info.URL != "https://svn.example.com/repo/trunk" {
		t.Errorf("url = %q", info.URL)
	}
	if info.RelativeURL != "^/trunk" {
		t.Errorf("relative-url = %q", info.RelativeURL)
	}
	if info.RepoRoot != "https://svn.example.com/repo" || info.RepoUUID == "" {
		t.Errorf("repo = %+v", info)
	}
	if info.WCRoot != "C:/work/repo" || info.Schedule != "normal" || info.Depth != "infinity" {
		t.Errorf("wc-info = %+v", info)
	}
	if info.Commit.Revision != 41 || info.Commit.Author != "alice" {
		t.Errorf("commit = %+v", info.Commit)
	}
}

func TestItemStatusCodeMapping(t *testing.T) {
	cases := map[ItemStatus]string{
		StatusModified:    "M",
		StatusAdded:       "A",
		StatusDeleted:     "D",
		StatusReplaced:    "R",
		StatusConflict:    "C",
		StatusMissing:     "!",
		StatusUnversioned: "?",
		StatusIgnored:     "I",
		StatusObstructed:  "~",
		StatusNormal:      " ",
	}
	for item, want := range cases {
		if got := item.Code(); got != want {
			t.Errorf("%v.Code() = %q, want %q", item, got, want)
		}
	}
}

func TestItemStatusFlags(t *testing.T) {
	if !StatusModified.IsInteresting() || !StatusUnversioned.IsInteresting() {
		t.Error("modified/unversioned should be interesting")
	}
	if StatusNormal.IsInteresting() || StatusIgnored.IsInteresting() {
		t.Error("normal/ignored should not be interesting")
	}
	if !StatusModified.IsVersioned() || StatusUnversioned.IsVersioned() {
		t.Error("versioned flag wrong")
	}
}

func TestDecodeOutput(t *testing.T) {
	// Valid UTF-8 passes through unchanged, including Chinese.
	utf8 := []byte("hello 中文")
	if got := decodeOutput(utf8); got != "hello 中文" {
		t.Errorf("utf8 passthrough = %q", got)
	}
	// GBK-encoded Chinese should decode back.
	want := "提交日志"
	enc := simplifiedchinese.GBK.NewEncoder()
	gbk, err := enc.Bytes([]byte(want))
	if err != nil {
		t.Fatalf("encode gbk: %v", err)
	}
	if got := decodeOutput(gbk); got != want {
		t.Errorf("gbk decode = %q, want %q", got, got)
	}
	// Empty input returns empty string.
	if got := decodeOutput(nil); got != "" {
		t.Errorf("nil = %q", got)
	}
}

func TestParseCommitOutput(t *testing.T) {
	out := "Sending        src/main.go\nAdding         src/new_file.go\nTransmitting file data .....\nCommitted revision 43.\n"
	res := parseCommitOutput(out)
	if res.Revision != 43 {
		t.Errorf("revision = %d, want 43", res.Revision)
	}
	if res.Output != out {
		t.Error("output not preserved")
	}
	// No revision line -> 0.
	res2 := parseCommitOutput("nothing here")
	if res2.Revision != 0 {
		t.Errorf("revision = %d, want 0", res2.Revision)
	}
}

func TestParseUpdateOutput(t *testing.T) {
	out := "Updating '.':\r\nA    src/added.go\r\nU    src/updated.go\r\nD    docs/old.txt\r\nC    src/conflict.go\r\nG    src/merged.go\r\nUpdated to revision 43.\r\n"
	res := parseUpdateOutput(out)
	if res.Revision != 43 {
		t.Errorf("revision = %d, want 43", res.Revision)
	}
	check := func(name string, got, want []string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("Added", res.Added, []string{"src/added.go"})
	check("Updated", res.Updated, []string{"src/updated.go"})
	check("Deleted", res.Deleted, []string{"docs/old.txt"})
	check("Conflicted", res.Conflicted, []string{"src/conflict.go"})
	check("Merged", res.Merged, []string{"src/merged.go"})

	// Already up to date: "At revision N."
	res2 := parseUpdateOutput("At revision 42.\n")
	if res2.Revision != 42 {
		t.Errorf("at revision = %d, want 42", res2.Revision)
	}
}

func TestCmdError(t *testing.T) {
	ce := &CmdError{
		Args:     []string{"status", "x"},
		ExitCode: 1,
		Stderr:   "svn: E155007: not a working copy",
	}
	msg := ce.Error()
	if !strings.Contains(msg, "exit 1") || !strings.Contains(msg, "E155007") {
		t.Errorf("CmdError.Error() = %q", msg)
	}

	// When stderr/stdout are empty (e.g. binary not found), the underlying
	// error must surface so the UI has something to show.
	notFound := &CmdError{
		Args:     []string{"status"},
		ExitCode: -1,
		Err:      errors.New("exec: \"svn\": executable file not found in %PATH%"),
	}
	m2 := notFound.Error()
	if !strings.Contains(m2, "not found") {
		t.Errorf("CmdError fallback = %q", m2)
	}
}
