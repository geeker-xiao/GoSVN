// Package gui implements the MyGo native UI windows for GoSVN.
//
// It depends only on the internal/svn package (for data and operations) and
// on github.com/egoist/mygo / mygo/ui (for the native widgets). It never
// shells out to svn directly.
package gui

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/theme"
	"github.com/zoo/gosvn/internal/svn"
)

// --- Design tokens ---------------------------------------------------------

// applyTheme installs the Lobe UI design system for the window. It respects
// the user's theme preference (system / light / dark) from Settings.
func applyTheme(c *ui.Context) {
	s := GetSettings()
	switch s.ThemeMode {
	case "light":
		c.SetTheme(theme.Default().Theme(false))
	case "dark":
		c.SetTheme(theme.Default().Theme(true))
	default:
		theme.Apply(c)
	}
}

// --- Shared building blocks ------------------------------------------------

// card renders a titled white surface with a hairline border and a soft
// shadow. body builds the card's content.
func card(c *ui.Context, title string, body func()) ui.Element {
	t := c.Theme()
	e := ui.Column(c).
		Padding(18).
		Gap(14).
		Radius(12).
		Background(t.Surface).
		Border(1, t.Border).
		Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.05))
	e.Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(15).Bold()
		}
		body()
	})
	return e
}

// statusTagColor maps a working-copy status code to a Lobe Tag color, so every
// status badge shares the design system's semantic palette.
func statusTagColor(item svn.ItemStatus) display.TagColor {
	switch item {
	case svn.StatusModified, svn.StatusReplaced:
		return display.TagWarning
	case svn.StatusAdded:
		return display.TagSuccess
	case svn.StatusDeleted, svn.StatusMissing, svn.StatusConflict, svn.StatusObstructed:
		return display.TagError
	case svn.StatusUnversioned, svn.StatusIgnored:
		return display.TagGray
	default:
		return ""
	}
}

// statusBadge draws a small pill showing the single-letter status code, using
// Lobe's Tag component so it matches the rest of the design system.
func statusBadge(c *ui.Context, item svn.ItemStatus) {
	display.Tag(c, item.Code(), display.TagProps{
		Color: statusTagColor(item),
		Size:  theme.Small,
		Round: true,
	})
}

// mutedText renders secondary text in the theme's muted color.
func mutedText(c *ui.Context, s string) {
	ui.Text(c, s).TextColor(c.Theme().TextMuted)
}

// accentPill draws a small highlighted pill with label, used for revision tags
// and other tokens (mirrors MyGit's rN badges). Built on Lobe's Tag so it
// follows the theme's accent color.
func accentPill(c *ui.Context, label string) {
	display.Tag(c, label, display.TagProps{
		Color:   display.TagProcessing,
		Variant: display.TagFilled,
		Size:    theme.Small,
		Round:   true,
	})
}

// errBanner renders an inline error banner using Lobe's Alert component.
func errBanner(c *ui.Context, msg string) {
	feedback.Alert(c, msg, feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
}

// --- Custom title bar (MyGit style: window draws its own title bar) --------

// titleBarHeight is the height of the title bars the windows draw themselves.
const titleBarHeight = 40

// customWindow hides the OS title bar but keeps the window controls, so the
// view can draw its own title bar under them — the look MyGit uses.
func customWindow(o mygo.WindowOptions) mygo.WindowOptions {
	o.TitleBarStyle = mygo.TitleBarHidden
	o.TitleBarHeight = titleBarHeight
	return o
}

// titleBar returns a row the height of the hidden title bar, clear of the
// window controls (traffic lights / caption buttons), that drags the window.
func titleBar(c *ui.Context) ui.Element {
	bar := c.TitleBar()
	left := float32(12)
	if bar.Left > 0 {
		left = bar.Left + 8
	}
	h := bar.Height
	if h < titleBarHeight {
		h = titleBarHeight
	}
	return ui.Row(c).Height(h).
		Padding(0, bar.Right+8, 0, left).
		Gap(8).AlignItems(ui.Center).DragWindow()
}

// --- Toast helpers ---------------------------------------------------------

// toastMsg is a toast queued by a background goroutine. The window's view
// flushes it on the next frame (where a ui.Context is available) through
// flushToast, because ShowToast needs the live context.
type toastMsg struct {
	title, desc string
	typ         feedback.ToastType
}

// flushToast renders and clears a pending toast. Call it once per view, before
// the ToastHost, so a queued toast surfaces exactly once.
func flushToast(c *ui.Context, p **toastMsg) {
	if *p == nil {
		return
	}
	m := *p
	feedback.ShowToast(c, feedback.ToastProps{Title: m.title, Description: m.desc, Type: m.typ})
	*p = nil
}

// toastHost mounts the Lobe toast stack in a view; pair it with flushToast.
func toastHost(c *ui.Context) {
	feedback.ToastHost(c, feedback.ToastHostProps{Placement: feedback.ToastBottomRight})
}

// --- Async helpers ---------------------------------------------------------

// runAsync runs work on a background goroutine and calls done on the window's
// main thread (via Window.Update) once it finishes, passing any error.
func runAsync(win *mygo.Window, work func() error, done func(error)) {
	go func() {
		err := work()
		win.Update(func() { done(err) })
	}()
}

// runAsyncResult is like runAsync but lets work return a value handed to done.
func runAsyncResult[T any](win *mygo.Window, work func() (T, error), done func(T, error)) {
	go func() {
		v, err := work()
		win.Update(func() { done(v, err) })
	}()
}

// --- Text / path helpers ---------------------------------------------------

// baseTitle returns a window title qualified with the target's base name.
func baseTitle(prefix, path string) string {
	base := filepath.Base(path)
	if base == "." || base == "" {
		base = path
	}
	return prefix + " - " + base
}

// formatDate turns an ISO-8601 svn date ("2024-05-02T10:00:00.000000Z") into
// a compact "2024-05-02 10:00" form.
func formatDate(s string) string {
	s = strings.Replace(s, "T", " ", 1)
	if i := strings.IndexAny(s, ".+Z"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 16 {
		s = s[:16]
	}
	return s
}

// firstLine returns the first line of a (possibly multi-line) string.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
