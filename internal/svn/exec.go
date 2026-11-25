package svn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// CmdError wraps a failed svn invocation with enough context for the UI to
// show a clear message: the command line, the exit code and a stderr summary.
type CmdError struct {
	Bin      string
	Args     []string
	ExitCode int
	Stderr   string
	Stdout   string
	Err      error // the underlying error, if any
}

func (e *CmdError) Error() string {
	// Binary not found: give a clear, actionable message instead of the raw
	// exec error, so users know they need to install SVN or set its path.
	if e.ExitCode == -1 && e.Err != nil {
		return fmt.Sprintf("未找到 svn 命令（%v）。请安装 SVN 命令行工具（如 TortoiseSVN，安装时勾选 command line client tools），或在设置中指定 svn 可执行文件路径。", e.Err)
	}
	summary := strings.TrimSpace(e.Stderr)
	if summary == "" {
		summary = strings.TrimSpace(e.Stdout)
	}
	if summary == "" && e.Err != nil {
		summary = e.Err.Error()
	}
	const max = 500
	if len(summary) > max {
		summary = summary[:max] + "..."
	}
	cmdline := strings.Join(e.Args, " ")
	if e.ExitCode != 0 {
		return fmt.Sprintf("svn %s failed (exit %d): %s", cmdline, e.ExitCode, summary)
	}
	return fmt.Sprintf("svn %s failed: %s", cmdline, summary)
}

// Unwrap returns the underlying error so errors.Is/errors.As keep working.
func (e *CmdError) Unwrap() error { return e.Err }

// Client runs svn commands. The zero value is usable and resolves the svn
// binary from PATH; set Bin to override it (e.g. when svn is not on PATH).
type Client struct {
	// Bin is the svn executable. Empty means "svn" (looked up on PATH).
	Bin string
	// Timeout is the default per-command timeout applied when the caller's
	// context has no deadline. Zero means no extra timeout.
	Timeout time.Duration
}

// DefaultClient returns a client configured from the environment and, when no
// explicit binary is given, auto-detected from common install locations.
// GOSVN_SVN_BIN overrides the binary path, GOSVN_SVN_TIMEOUT (Go duration)
// overrides the default timeout.
func DefaultClient() *Client {
	bin := os.Getenv("GOSVN_SVN_BIN")
	if bin == "" {
		bin = Detect()
	}
	c := &Client{Bin: bin}
	if d := os.Getenv("GOSVN_SVN_TIMEOUT"); d != "" {
		if dur, err := time.ParseDuration(d); err == nil {
			c.Timeout = dur
		}
	}
	return c
}

func (c *Client) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "svn"
}

// run executes svn with the given arguments and returns the decoded stdout and
// stderr as strings. ctx controls cancellation; if ctx has no deadline and
// c.Timeout is set, a deadline is derived from it.
func (c *Client) run(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	if c.Timeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.Timeout)
			defer cancel()
		}
	}

	cmd := exec.CommandContext(ctx, c.bin(), args...)
	// Encourage UTF-8 output on POSIX; on Windows the decodeOutput fallback
	// below takes care of local code pages such as GBK.
	env := os.Environ()
	env = append(env,
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
	)
	cmd.Env = env

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = decodeOutput(outBuf.Bytes())
	stderr = decodeOutput(errBuf.Bytes())

	if runErr != nil {
		var exitErr *exec.ExitError
		code := -1
		if errors.As(runErr, &exitErr) {
			code = exitErr.ExitCode()
		}
		return stdout, stderr, &CmdError{
			Bin:      c.bin(),
			Args:     args,
			ExitCode: code,
			Stderr:   stderr,
			Stdout:   stdout,
			Err:      runErr,
		}
	}
	if ctx.Err() != nil {
		return stdout, stderr, ctx.Err()
	}
	return stdout, stderr, nil
}

// runXML runs an svn command expected to emit XML on stdout and returns the
// raw decoded stdout bytes for parsing with encoding/xml.
func (c *Client) runXML(ctx context.Context, args ...string) ([]byte, error) {
	stdout, _, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return []byte(stdout), nil
}

// decodeOutput interprets command output bytes as text. SVN writes --xml in
// UTF-8, but plain output on Windows may be in the local code page (e.g. GBK
// on Chinese Windows). We accept valid UTF-8 as-is and fall back to GBK.
func decodeOutput(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) {
		return string(b)
	}
	dec := simplifiedchinese.GBK.NewDecoder()
	out, _, err := transform.Bytes(dec, b)
	if err == nil {
		return string(out)
	}
	// Last resort: keep the bytes as Latin-1 so nothing is lost.
	runes := make([]rune, len(b))
	for i, v := range b {
		runes[i] = rune(v)
	}
	return string(runes)
}

// isWindows reports the build target, used for platform-specific quoting hints.
func isWindows() bool { return runtime.GOOS == "windows" }
