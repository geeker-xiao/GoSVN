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

// CommitApp is the state of the Status / Commit window.
type CommitApp struct {
	client  *svn.Client
	path    string
	win     *mygo.Window
	entries []svn.StatusEntry
	checked map[string]bool
	message string
	status  string
	err     string
	busy    bool

	list     ui.ListState
	selected int
	pending  *toastMsg
}

// OpenCommitWindow opens the working-copy status / commit window for path.
func OpenCommitWindow(client *svn.Client, path string) *mygo.Window {
	app := &CommitApp{
		client:  client,
		path:    path,
		checked: make(map[string]bool),
	}
	app.selected = -1
	app.list.Selected = &app.selected
	app.list.Key = func(i int) any {
		if i < len(app.entries) {
			return app.entries[i].Path
		}
		return i
	}
	app.list.Label = func(i int) string {
		if i < len(app.entries) {
			return app.entries[i].Path
		}
		return ""
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:   baseTitle("Commit", path),
		Width:   920,
		Height:  640,
		Content: ui.View(app.view),
	})
	app.win = win
	app.refresh()
	return win
}

func (a *CommitApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(14).Children(func() {
		// Header
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.GitCommit()).FontSize(20).TextColor(t.Accent)
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, "提交变更").FontSize(18).Bold()
				ui.Text(c, a.path).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
			if action.Button(c, "刷新", action.ButtonProps{Size: theme.Small}).Clicked() {
				a.refresh()
			}
		})

		if a.err != "" {
			feedback.Alert(c, a.err, feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
		}

		// File list card (grows to fill the middle)
		card(c, fmt.Sprintf("变更文件  ·  %d 项", len(a.entries)), func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if action.Button(c, "全选", action.ButtonProps{Size: theme.Small}).Clicked() {
					a.selectAll(true)
				}
				if action.Button(c, "全不选", action.ButtonProps{Size: theme.Small}).Clicked() {
					a.selectAll(false)
				}
				ui.Text(c, "勾选要提交的文件，双击行查看差异").
					FontSize(11).TextColor(t.TextMuted).Grow(1)
			})
			a.renderList(c)
		}).Grow(1)

		// Commit message card
		card(c, "提交信息", func() {
			ui.TextArea(c, &a.message).Height(120).Placeholder("输入提交说明…")
		})

		// Bottom bar
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if a.busy {
				ui.Spinner(c)
			}
			ui.Text(c, a.status).TextColor(t.TextMuted).Grow(1).SingleLine().FontSize(12)
			label := "提交"
			if n := a.countChecked(); n > 0 {
				label = fmt.Sprintf("提交 %d 个文件", n)
			}
			if action.Button(c, label, action.ButtonProps{Variant: action.Primary}).Disabled(a.busy || a.countChecked() == 0).Clicked() {
				a.doCommit()
			}
		})
		toastHost(c)
	})
}

func (a *CommitApp) renderList(c *ui.Context) {
	t := c.Theme()
	el := ui.List(c, &a.list, len(a.entries), func(i int) {
		e := a.entries[i]
		checked := a.checked[e.Path]
		ui.Row(c).Gap(10).Padding(7, 6).AlignItems(ui.Center).Radius(6).Children(func() {
			ui.Checkbox(c, &checked, "").OnChange(func() {
				a.checked[e.Path] = checked
			})
			statusBadge(c, e.Item)
			ui.Text(c, e.Path).Grow(1).SingleLine()
			ui.Text(c, e.Item.Description()).
				FontSize(11).TextColor(t.TextMuted).Width(84)
		})
	}).Grow(1)
	el.Children(func() {
		if len(a.entries) == 0 && !a.busy {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8).Children(func() {
				ui.Icon(c, lucide.Check()).FontSize(16).TextColor(t.Success)
				ui.Text(c, "工作副本是干净的，没有变更。").TextColor(t.TextMuted)
			})
		}
	})
	if el.Submitted() && a.selected >= 0 && a.selected < len(a.entries) {
		OpenDiffWindow(a.client, a.entries[a.selected].Path, "")
	}
}

func (a *CommitApp) refresh() {
	if a.busy {
		return
	}
	a.busy = true
	a.err = ""
	a.status = "Loading status..."
	runAsyncResult(a.win, func() ([]svn.StatusEntry, error) {
		return a.client.Status(context.Background(), a.path)
	}, func(entries []svn.StatusEntry, err error) {
		a.busy = false
		if err != nil {
			a.err = err.Error()
			a.entries = nil
			return
		}
		a.entries = entries
		for _, e := range entries {
			if _, ok := a.checked[e.Path]; !ok {
				// Pre-select versioned, changed items (not unversioned).
				a.checked[e.Path] = e.Item.IsVersioned() && e.Item.IsInteresting()
			}
		}
		a.status = fmt.Sprintf("%d changed item(s)", len(entries))
	})
}

func (a *CommitApp) doCommit() {
	if a.busy {
		return
	}
	var paths []string
	for _, e := range a.entries {
		if a.checked[e.Path] {
			paths = append(paths, e.Path)
		}
	}
	if len(paths) == 0 {
		return
	}
	a.busy = true
	a.err = ""
	a.status = "Committing..."
	runAsyncResult(a.win, func() (*svn.CommitResult, error) {
		return a.client.Commit(context.Background(), paths, a.message)
	}, func(res *svn.CommitResult, err error) {
		a.busy = false
		if err != nil {
			a.err = err.Error()
			return
		}
		if res.Revision > 0 {
			a.status = fmt.Sprintf("Committed revision %d.", res.Revision)
			a.message = ""
		} else {
			a.status = "Commit finished."
		}
		a.pending = &toastMsg{title: "提交成功", desc: a.status, typ: feedback.ToastSuccess}
		a.refresh()
	})
}

func (a *CommitApp) selectAll(v bool) {
	for _, e := range a.entries {
		a.checked[e.Path] = v
	}
}

func (a *CommitApp) countChecked() int {
	n := 0
	for _, e := range a.entries {
		if a.checked[e.Path] {
			n++
		}
	}
	return n
}
