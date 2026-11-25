package gui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
	"github.com/zoo/gosvn/internal/svn"
)

// LogApp is the state of the log window.
type LogApp struct {
	client  *svn.Client
	path    string
	win     *mygo.Window
	entries []svn.LogEntry
	err     string
	busy    bool

	revList     ui.ListState
	revSelected int

	pathList     ui.ListState
	pathSelected int
	pathMsg      string // non-pointer holder bound to TextArea

	splitSize float32
	pending   *toastMsg
}

// OpenLogWindow opens the revision log window for path.
func OpenLogWindow(client *svn.Client, path string) *mygo.Window {
	app := &LogApp{
		client:    client,
		path:      path,
		splitSize: 280,
	}
	app.revSelected = -1
	app.pathSelected = -1
	app.revList.Selected = &app.revSelected
	app.revList.Key = func(i int) any {
		if i < len(app.entries) {
			return app.entries[i].Revision
		}
		return i
	}
	app.pathList.Selected = &app.pathSelected
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:   baseTitle("Log", path),
		Width:   980,
		Height:  640,
		Content: ui.View(app.view),
	})
	app.win = win
	app.load()
	return win
}

func (a *LogApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(14).Children(func() {
		// Header
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.History()).FontSize(20).TextColor(t.Accent)
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, "版本历史").FontSize(18).Bold()
				ui.Text(c, a.path).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
			if a.busy {
				ui.Spinner(c)
			}
			if action.Button(c, "刷新", action.ButtonProps{Size: theme.Small}).Clicked() {
				a.load()
			}
		})

		if a.err != "" {
			feedback.Alert(c, a.err, feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
		}

		ui.Split(c, &a.splitSize, func() { a.revisionPane(c) }, func() { a.detailPane(c) }).Grow(1)
		toastHost(c)
	})
}

func (a *LogApp) revisionPane(c *ui.Context) {
	t := c.Theme()
	card(c, "修订记录", func() {
		el := ui.List(c, &a.revList, len(a.entries), func(i int) {
			e := a.entries[i]
			ui.Row(c).Gap(10).Padding(8, 10).AlignItems(ui.Center).Radius(6).Children(func() {
				accentPill(c, fmt.Sprintf("r%d", e.Revision))
				ui.Text(c, e.Author).TextColor(t.TextMuted).Width(100).SingleLine()
				ui.Text(c, formatDate(e.Date)).TextColor(t.TextMuted).Width(110).SingleLine()
				ui.Text(c, firstLine(e.Message)).Grow(1).SingleLine()
			})
		}).Grow(1)
		el.Children(func() {
			if len(a.entries) == 0 && !a.busy {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8).Children(func() {
					ui.Icon(c, lucide.Info()).FontSize(16).TextColor(t.TextMuted)
					ui.Text(c, "没有日志记录。").TextColor(t.TextMuted)
				})
			}
		})
	}).Grow(1)
}

func (a *LogApp) detailPane(c *ui.Context) {
	t := c.Theme()
	card(c, "修订详情", func() {
		if a.revSelected < 0 || a.revSelected >= len(a.entries) {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8).Children(func() {
				ui.Icon(c, lucide.Info()).FontSize(16).TextColor(t.TextMuted)
				ui.Text(c, "选择左侧的修订以查看详情。").TextColor(t.TextMuted)
			})
			return
		}
		e := a.entries[a.revSelected]
		a.pathMsg = e.Message

		// Revision header
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("r%d", e.Revision)).
				TextColor(t.Accent).Bold().FontSize(22)
			ui.Text(c, "·").TextColor(t.TextMuted)
			ui.Text(c, e.Author).TextColor(t.Text)
			ui.Text(c, "·").TextColor(t.TextMuted)
			ui.Text(c, formatDate(e.Date)).TextColor(t.TextMuted)
		})

		// Commit message
		ui.TextArea(c, &a.pathMsg).ReadOnly(true).Height(100)

		// Changed paths
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("变更路径  ·  %d", len(e.Paths))).
				FontSize(12).Bold().TextColor(t.TextMuted)
		})
		a.renderPaths(c)
	}).Grow(1)
}

func (a *LogApp) renderPaths(c *ui.Context) {
	t := c.Theme()
	if a.revSelected < 0 || a.revSelected >= len(a.entries) {
		return
	}
	paths := a.entries[a.revSelected].Paths
	el := ui.List(c, &a.pathList, len(paths), func(i int) {
		p := paths[i]
		ui.Row(c).Gap(10).Padding(7, 6).AlignItems(ui.Center).Radius(6).Children(func() {
			statusBadge(c, logActionStatus(p.Action))
			ui.Text(c, p.Path).Grow(1).SingleLine()
			ui.Text(c, p.Kind).FontSize(11).TextColor(t.TextMuted).Width(44)
		})
	}).Grow(1)
	if el.Submitted() && a.pathSelected >= 0 && a.pathSelected < len(paths) {
		rev := a.entries[a.revSelected].Revision
		from := rev - 1
		if from < 1 {
			from = 1
		}
		OpenDiffWindow(a.client, paths[a.pathSelected].Path, fmt.Sprintf("%d:%d", from, rev))
	}
}

// logActionStatus maps an `svn log -v` action letter (M/A/D/R) back to an
// ItemStatus so it can reuse the shared status badge.
func logActionStatus(action string) svn.ItemStatus {
	switch action {
	case "M":
		return svn.StatusModified
	case "A":
		return svn.StatusAdded
	case "D":
		return svn.StatusDeleted
	case "R":
		return svn.StatusReplaced
	default:
		return svn.ItemStatus(action)
	}
}

func (a *LogApp) load() {
	if a.busy {
		return
	}
	a.busy = true
	a.err = ""
	runAsyncResult(a.win, func() ([]svn.LogEntry, error) {
		return a.client.Log(context.Background(), a.path, 200, true)
	}, func(entries []svn.LogEntry, err error) {
		a.busy = false
		if err != nil {
			a.err = err.Error()
			a.entries = nil
			return
		}
		a.entries = entries
		if len(entries) > 0 {
			a.revSelected = 0
			a.pathMsg = entries[0].Message
		}
	})
}
