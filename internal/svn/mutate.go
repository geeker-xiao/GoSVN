package svn

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// committedRevRe matches "Committed revision 43."
var committedRevRe = regexp.MustCompile(`(?m)Committed revision (\d+)\.`)

// updatedRevRe matches "Updated to revision 43." and "At revision 43."
var updatedRevRe = regexp.MustCompile(`(?m)(?:Updated to|At) revision (\d+)\.`)

// updateLineRe matches an update status line such as "A    path" or "U    path".
var updateLineRe = regexp.MustCompile(`^([A-Z])\s+(.+)$`)

// Commit commits the given paths with message. Use paths=nil to commit the
// current directory / whole working copy. It returns the new revision when
// svn reports one.
func (c *Client) Commit(ctx context.Context, paths []string, message string) (*CommitResult, error) {
	args := []string{"commit"}
	args = append(args, paths...)
	if message == "" {
		message = "(no message)"
	}
	args = append(args, "-m", message)
	stdout, _, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseCommitOutput(stdout), nil
}

// parseCommitOutput extracts the committed revision from svn commit output.
func parseCommitOutput(stdout string) *CommitResult {
	res := &CommitResult{Output: stdout}
	if m := committedRevRe.FindStringSubmatch(stdout); m != nil {
		if rev, err := strconv.Atoi(m[1]); err == nil {
			res.Revision = rev
		}
	}
	return res
}

// Update updates path to the latest revision and parses the per-file action
// summary from svn's output.
func (c *Client) Update(ctx context.Context, path string) (*UpdateResult, error) {
	args := []string{"update", path}
	stdout, _, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseUpdateOutput(stdout), nil
}

// parseUpdateOutput extracts the target revision and per-file action summary
// from svn update output.
func parseUpdateOutput(stdout string) *UpdateResult {
	res := &UpdateResult{Output: stdout}
	if m := updatedRevRe.FindStringSubmatch(stdout); m != nil {
		if rev, err := strconv.Atoi(m[1]); err == nil {
			res.Revision = rev
		}
	}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		m := updateLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		action, p := m[1], strings.TrimSpace(m[2])
		switch action {
		case "A":
			res.Added = append(res.Added, p)
		case "U":
			res.Updated = append(res.Updated, p)
		case "D":
			res.Deleted = append(res.Deleted, p)
		case "C":
			res.Conflicted = append(res.Conflicted, p)
		case "G":
			res.Merged = append(res.Merged, p)
		}
	}
	return res
}

// Diff returns the unified diff text for path. If rev is non-empty it is
// passed as `-r rev` (e.g. "42" or "41:42"); otherwise the working diff is
// shown.
func (c *Client) Diff(ctx context.Context, path, rev string) (string, error) {
	args := []string{"diff"}
	if rev != "" {
		args = append(args, "-r", rev)
	}
	args = append(args, path)
	stdout, _, err := c.run(ctx, args...)
	return stdout, err
}

// Revert restores paths to their pristine state. recursive enables `-R`.
func (c *Client) Revert(ctx context.Context, paths []string, recursive bool) (string, error) {
	args := []string{"revert"}
	if recursive {
		args = append(args, "-R")
	}
	args = append(args, paths...)
	stdout, _, err := c.run(ctx, args...)
	return stdout, err
}

// Cleanup runs `svn cleanup` on path, removing locks and finishing unfinished
// operations.
func (c *Client) Cleanup(ctx context.Context, path string) (string, error) {
	stdout, _, err := c.run(ctx, "cleanup", path)
	return stdout, err
}

// Add schedules unversioned paths for addition.
func (c *Client) Add(ctx context.Context, paths []string) (string, error) {
	args := append([]string{"add"}, paths...)
	stdout, _, err := c.run(ctx, args...)
	return stdout, err
}

// Checkout checks out a remote repository URL into a local directory.
func (c *Client) Checkout(ctx context.Context, url, path string) (string, error) {
	stdout, _, err := c.run(ctx, "checkout", url, path)
	return stdout, err
}

// RemoteEntry is one item returned by svn list.
type RemoteEntry struct {
	Name string
	Kind string // "dir" or "file"
	Size int64
}

// ListRemote lists the contents of a remote repository URL.
func (c *Client) ListRemote(ctx context.Context, url string) ([]RemoteEntry, error) {
	stdout, _, err := c.run(ctx, "list", url)
	if err != nil {
		return nil, err
	}
	var entries []RemoteEntry
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		// svn list output: "name/" for dirs, "name" for files (verbose off).
		if strings.HasSuffix(line, "/") {
			entries = append(entries, RemoteEntry{Name: strings.TrimSuffix(line, "/"), Kind: "dir"})
		} else {
			entries = append(entries, RemoteEntry{Name: line, Kind: "file"})
		}
	}
	return entries, nil
}

// CreateRepository creates a new FSFS repository at path using svnadmin.
// svnadmin is expected next to the configured svn binary.
func (c *Client) CreateRepository(ctx context.Context, path string) (string, error) {
	bin := c.bin()
	// Resolve svnadmin beside svn.
	dir := filepath.Dir(bin)
	admin := filepath.Join(dir, "svnadmin")
	if runtime.GOOS == "windows" {
		admin += ".exe"
	}
	if _, err := os.Stat(admin); err != nil {
		return "", fmt.Errorf("未找到 svnadmin（%s）。请确认已完整安装 SVN 命令行工具", admin)
	}
	cmd := exec.CommandContext(ctx, admin, "create", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("创建仓库失败：%v\n%s", err, out.String())
	}
	return out.String(), nil
}

// Default is a package-level client configured from the environment
// (GOSVN_SVN_BIN, GOSVN_SVN_TIMEOUT). UIs may use it directly or construct
// their own *Client when they need a custom binary path or timeout.
var Default = DefaultClient()
