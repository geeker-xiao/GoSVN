package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
	"github.com/zoo/gosvn/internal/svn"
)

// MainApp is the state of the main working-copy window. Its layout mirrors
// MyGit: a custom title bar, a left panel with a segmented Changes/History
// switch and a list, and a right detail/diff panel.
type MainApp struct {
	client *svn.Client
	win    *mygo.Window
	path   string

	info    *svn.Info
	entries []svn.StatusEntry
	logs    []svn.LogEntry

	infoErr   string
	statusErr string
	logErr    string
	loading   bool

	// UI layout state
	tab    int // 0 = Changes, 1 = History
	splitW float32
	chgSel int
	logSel int
	chgList ui.ListState
	logList ui.ListState

	// Commit state (Changes tab)
	checked      map[string]bool
	message      string
	committing   bool
	commitErr    string
	commitStatus string

	// Inline diff state (right panel)
	diffText    string
	diffErr     string
	diffLoading bool
	diffGen     int
	pathMsg     string // bound textarea for the selected commit's message

	askingRevert  bool
	askingCleanup bool
	pending       *toastMsg
}

// OpenMainWindow opens the main working-copy window for path.
func OpenMainWindow(client *svn.Client, path string) *mygo.Window {
	if path == "" {
		path = "."
	}
	abs, _ := filepath.Abs(path)
	if abs != "" {
		path = abs
	}
	// Only remember real working copies, so mistyped paths don't pollute the
	// recent list (the window still opens and shows an error banner).
	if st, err := os.Stat(filepath.Join(path, ".svn")); err == nil && st.IsDir() {
		addRecent(path)
	}

	app := &MainApp{
		client:  client,
		path:    path,
		splitW:  340,
		chgSel:  -1,
		logSel:  -1,
		checked: map[string]bool{},
	}
	app.chgList = ui.ListState{
		Selected: &app.chgSel,
		Key:      func(i int) any { return app.entries[i].Path },
		Label:    func(i int) string { return app.entries[i].Path },
	}
	app.logList = ui.ListState{
		Selected: &app.logSel,
		Key:      func(i int) any { return app.logs[i].Revision },
		Label:    func(i int) string { return fmt.Sprintf("r%d", app.logs[i].Revision) },
	}
	win := mygo.NewWindow(customWindow(mygo.WindowOptions{
		Title:     baseTitle("GoSVN", path),
		Width:     1180,
		Height:    740,
		MinWidth:  880,
		MinHeight: 560,
		Content:   ui.View(app.view),
	}))
	app.win = win
	app.loadAll()
	return win
}

func (a *MainApp) busy() bool {
	return a.loading || a.diffLoading || a.committing
}

func (a *MainApp) firstErr() string {
	switch {
	case a.statusErr != "":
		return a.statusErr
	case a.infoErr != "":
		return a.infoErr
	case a.logErr != "":
		return a.logErr
	}
	return ""
}

func (a *MainApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)

	ui.Column(c).Fill().Children(func() {
		a.toolbar(c)

		if e := a.firstErr(); e != "" {
			ui.Box(c).Padding(10, 12, 0, 12).Children(func() {
				errBanner(c, e)
			})
		}

		ui.Split(c, &a.splitW, func() { a.leftPanel(c) }, func() { a.detailPane(c) }).
			Grow(1).Basis(0)
		toastHost(c)
	})

	// Confirmation dialogs (kept from the previous design)
	switch ui.AlertDialog(c, &a.askingCleanup, "清理工作副本？",
		"移除此工作副本的锁并完成未完成的操作。", "取消", "清理") {
	case 1:
		a.doCleanup()
	}
	switch ui.AlertDialog(c, &a.askingRevert, "还原所有修改？",
		"这将丢弃此工作副本中的全部本地修改，且无法撤销。", "取消", "还原") {
	case 1:
		a.doRevert()
	}
}

// ---------- Toolbar (MyGit-style custom title bar) ----------

func (a *MainApp) toolbar(c *ui.Context) {
	t := c.Theme()
	titleBar(c).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
		Children(func() {
			ui.Icon(c, lucide.Database()).FontSize(16).TextColor(t.Accent)
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, projectName(a.path)).FontSize(13).Bold().SingleLine()
				ui.Text(c, a.path).FontSize(10).TextColor(t.TextMuted).SingleLine()
			})
			if a.info != nil {
				accentPill(c, fmt.Sprintf("r%d", a.info.Revision))
			}
			ui.Box(c).Grow(1)
			if a.busy() {
				ui.Spinner(c).Size(14, 14)
			}
			if action.Button(c, "刷新", action.ButtonProps{Size: theme.Small}).Clicked() {
				a.loadAll()
			}
			if action.Button(c, "清理", action.ButtonProps{Size: theme.Small}).Clicked() {
				a.askingCleanup = true
			}
			if action.Button(c, "还原…", action.ButtonProps{Size: theme.Small}).Clicked() {
				a.askingRevert = true
			}
			if action.Button(c, "打开项目", action.ButtonProps{Size: theme.Small}).Clicked() {
				OpenWelcomeWindow(a.client, func(p string) {
					OpenMainWindow(a.client, p)
				})
			}
			if action.Button(c, "关闭项目", action.ButtonProps{Size: theme.Small}).Clicked() {
				OpenWelcomeWindow(a.client, func(p string) {
					OpenMainWindow(a.client, p)
				})
				a.win.Close()
			}
			if action.ActionIcon(c, lucide.Settings(), action.ActionIconProps{Size: theme.Small, Title: "设置"}).Clicked() {
				OpenSettingsWindow()
			}
		})
}

// ---------- Left panel (segmented Changes / History) ----------

func (a *MainApp) leftPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).
		BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		ui.Box(c).Padding(8, 10, 0, 10).Children(func() {
			action.Segmented(c, &a.tab, []string{"变更", "历史"}, action.SegmentedProps{Size: theme.Small, Block: true})
		})
		if a.tab == 0 {
			a.changesPanel(c)
		} else {
			a.historyPanel(c)
		}
	})
}

func (a *MainApp) changesPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).Basis(0).Children(func() {
		// List header
		ui.Row(c).Padding(6, 10).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 项变更", len(a.entries))).
				FontSize(12).Bold().Grow(1)
			if action.Button(c, "全选", action.ButtonProps{Size: theme.Small}).Clicked() {
				a.selectAll(true)
			}
			if action.Button(c, "全不选", action.ButtonProps{Size: theme.Small}).Clicked() {
				a.selectAll(false)
			}
		})

		// Change list
		el := ui.List(c, &a.chgList, len(a.entries), func(i int) {
			e := a.entries[i]
			checked := a.checked[e.Path]
			ui.Row(c).Gap(10).Padding(6, 10).AlignItems(ui.Center).Radius(6).Children(func() {
				ui.Checkbox(c, &checked, "").OnChange(func() {
					a.checked[e.Path] = checked
				})
				statusBadge(c, e.Item)
				ui.Text(c, e.Path).Grow(1).SingleLine()
				ui.Text(c, e.Item.Description()).FontSize(11).
					TextColor(t.TextMuted).Width(72)
			})
		}).Grow(1).Basis(0)
		el.Children(func() {
			a.changeListState(c)
		})
		if el.Changed() {
			a.loadDiffForChange()
		}
		if el.Submitted() && a.chgSel >= 0 && a.chgSel < len(a.entries) {
			OpenDiffWindow(a.client, a.entries[a.chgSel].Path, "")
		}

		// Commit area
		ui.Column(c).Padding(8, 10).Gap(6).
			BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			if a.commitErr != "" {
				feedback.Alert(c, a.commitErr, feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
			} else if a.commitStatus != "" {
				feedback.Alert(c, a.commitStatus, feedback.AlertProps{Type: feedback.Success, Variant: feedback.Soft})
			}
			ui.TextArea(c, &a.message).Height(80).Placeholder("输入提交说明…")
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				n := a.countChecked()
				ui.Text(c, fmt.Sprintf("已选 %d 项", n)).
					FontSize(11).TextColor(t.TextMuted).Grow(1)
				if action.Button(c, "提交", action.ButtonProps{Variant: action.Primary}).Disabled(a.committing || n == 0).Clicked() {
					a.doCommit()
				}
			})
		})
	})
}

func (a *MainApp) changeListState(c *ui.Context) {
	t := c.Theme()
	switch {
	case a.loading && len(a.entries) == 0 && a.statusErr == "":
		ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "加载中…").TextColor(t.TextMuted)
		})
	case a.statusErr != "":
		ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.TriangleAlert()).FontSize(14).TextColor(t.Danger)
			ui.Text(c, a.statusErr).TextColor(t.Danger).SingleLine()
		})
	case len(a.entries) == 0:
		ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.Check()).FontSize(16).TextColor(t.Success)
			ui.Text(c, "工作副本是干净的，没有待提交的变更。").TextColor(t.TextMuted)
		})
	}
}

func (a *MainApp) historyPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Grow(1).Basis(0).Children(func() {
		ui.Row(c).Padding(6, 10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 条提交", len(a.logs))).
				FontSize(12).Bold().Grow(1)
		})
		el := ui.List(c, &a.logList, len(a.logs), func(i int) {
			l := a.logs[i]
			ui.Row(c).Gap(10).Padding(8, 10).AlignItems(ui.Center).Radius(6).Children(func() {
				accentPill(c, fmt.Sprintf("r%d", l.Revision))
				ui.Text(c, l.Author).TextColor(t.TextMuted).Width(90).SingleLine()
				ui.Text(c, formatDate(l.Date)).FontSize(11).
					TextColor(t.TextMuted).Width(100)
				ui.Text(c, firstLine(l.Message)).Grow(1).SingleLine()
			})
		}).Grow(1).Basis(0)
		el.Children(func() {
			a.historyListState(c)
		})
		if el.Changed() {
			a.loadDiffForLog()
		}
	})
}

func (a *MainApp) historyListState(c *ui.Context) {
	t := c.Theme()
	switch {
	case a.loading && len(a.logs) == 0 && a.logErr == "":
		ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "加载中…").TextColor(t.TextMuted)
		})
	case a.logErr != "":
		ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.TriangleAlert()).FontSize(14).TextColor(t.Danger)
			ui.Text(c, a.logErr).TextColor(t.Danger).SingleLine()
		})
	case len(a.logs) == 0:
		ui.Text(c, "暂无提交记录。").TextColor(t.TextMuted).Padding(12)
	}
}

// ---------- Right panel (detail + inline diff) ----------

func (a *MainApp) detailPane(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(12).Gap(10).Children(func() {
		a.detailHeader(c)

		// Diff viewer
		ui.Column(c).Grow(1).Basis(0).Radius(8).
			Border(1, t.Border).Background(t.Surface).Children(func() {
			switch {
			case a.diffLoading:
				ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c)
					ui.Text(c, "加载差异…").TextColor(t.TextMuted)
				})
			case a.diffErr != "":
				ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, lucide.TriangleAlert()).FontSize(14).TextColor(t.Danger)
					ui.Text(c, a.diffErr).TextColor(t.Danger)
				})
			case a.diffText == "":
				ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Icon(c, lucide.GitCompare()).FontSize(34).TextColor(t.TextMuted)
					ui.Text(c, a.diffHint()).TextColor(t.TextMuted).Padding(10)
				})
			default:
				ui.TextArea(c, &a.diffText).ReadOnly(true).Grow(1)
			}
		})
	})
}

func (a *MainApp) diffHint() string {
	if a.tab == 0 {
		return "选择左侧文件查看差异"
	}
	return "选择左侧修订查看提交内容"
}

func (a *MainApp) detailHeader(c *ui.Context) {
	t := c.Theme()
	if a.tab == 0 {
		if a.chgSel < 0 || a.chgSel >= len(a.entries) {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, lucide.List()).FontSize(15).TextColor(t.TextMuted)
				ui.Text(c, "选择一个文件查看差异").TextColor(t.TextMuted)
			})
			return
		}
		e := a.entries[a.chgSel]
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			statusBadge(c, e.Item)
			ui.Text(c, e.Path).Bold().Grow(1).SingleLine()
			ui.Text(c, e.Item.Description()).FontSize(11).TextColor(t.TextMuted)
		})
		return
	}

	// History tab
	if a.logSel < 0 || a.logSel >= len(a.logs) {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.History()).FontSize(15).TextColor(t.TextMuted)
			ui.Text(c, "选择一个修订查看提交内容").TextColor(t.TextMuted)
		})
		return
	}
	l := a.logs[a.logSel]
	a.pathMsg = l.Message
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			accentPill(c, fmt.Sprintf("r%d", l.Revision))
			ui.Text(c, l.Author)
			ui.Text(c, "·").TextColor(t.TextMuted)
			ui.Text(c, formatDate(l.Date)).TextColor(t.TextMuted)
		})
		ui.TextArea(c, &a.pathMsg).ReadOnly(true).Height(60)
		ui.Text(c, fmt.Sprintf("变更路径 %d", len(l.Paths))).
			FontSize(11).TextColor(t.TextMuted)
	})
}

// ---------- Diff loaders ----------

func (a *MainApp) clearDiff() {
	a.diffLoading = false
	a.diffErr = ""
	a.diffText = ""
}

func (a *MainApp) loadDiffForChange() {
	if a.chgSel < 0 || a.chgSel >= len(a.entries) {
		a.clearDiff()
		return
	}
	e := a.entries[a.chgSel]
	a.diffGen++
	gen := a.diffGen
	a.diffLoading = true
	a.diffErr = ""
	a.diffText = ""
	runAsyncResult(a.win, func() (string, error) {
		return a.client.Diff(context.Background(), e.Path, "")
	}, func(text string, err error) {
		if gen != a.diffGen {
			return
		}
		a.diffLoading = false
		if err != nil {
			a.diffErr = err.Error()
			return
		}
		a.diffText = text
		if text == "" {
			a.diffErr = "该文件无差异。"
		}
	})
}

func (a *MainApp) loadDiffForLog() {
	if a.logSel < 0 || a.logSel >= len(a.logs) {
		a.clearDiff()
		return
	}
	rev := a.logs[a.logSel].Revision
	from := rev - 1
	if from < 1 {
		from = 1
	}
	a.diffGen++
	gen := a.diffGen
	a.diffLoading = true
	a.diffErr = ""
	a.diffText = ""
	runAsyncResult(a.win, func() (string, error) {
		return a.client.Diff(context.Background(), a.path, fmt.Sprintf("%d:%d", from, rev))
	}, func(text string, err error) {
		if gen != a.diffGen {
			return
		}
		a.diffLoading = false
		if err != nil {
			a.diffErr = err.Error()
			return
		}
		a.diffText = text
		if text == "" {
			a.diffErr = "该修订无差异。"
		}
	})
}

// ---------- Commit ----------

func (a *MainApp) selectAll(v bool) {
	for _, e := range a.entries {
		a.checked[e.Path] = v
	}
}

func (a *MainApp) countChecked() int {
	n := 0
	for _, e := range a.entries {
		if a.checked[e.Path] {
			n++
		}
	}
	return n
}

func (a *MainApp) doCommit() {
	if a.committing {
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
	a.committing = true
	a.commitErr = ""
	a.commitStatus = ""
	runAsyncResult(a.win, func() (*svn.CommitResult, error) {
		return a.client.Commit(context.Background(), paths, a.message)
	}, func(res *svn.CommitResult, err error) {
		a.committing = false
		if err != nil {
			a.commitErr = err.Error()
			return
		}
		if res.Revision > 0 {
			a.commitStatus = fmt.Sprintf("已提交至 r%d。", res.Revision)
			a.message = ""
		} else {
			a.commitStatus = "提交完成。"
		}
		a.pending = &toastMsg{title: "提交成功", desc: a.commitStatus, typ: feedback.ToastSuccess}
		a.loadAll()
	})
}

// ---------- Data loaders (unchanged logic) ----------

// loadAll fetches info, status and recent log in parallel.
func (a *MainApp) loadAll() {
	if a.loading {
		return
	}
	a.loading = true
	a.infoErr, a.statusErr, a.logErr = "", "", ""
	a.commitErr = ""

	runAsyncResult(a.win, func() (*svn.Info, error) {
		return a.client.Info(context.Background(), a.path)
	}, func(info *svn.Info, err error) {
		if err != nil {
			a.infoErr = err.Error()
		} else {
			a.info = info
		}
		a.checkDone()
	})
	runAsyncResult(a.win, func() ([]svn.StatusEntry, error) {
		return a.client.Status(context.Background(), a.path)
	}, func(entries []svn.StatusEntry, err error) {
		if err != nil {
			a.statusErr = err.Error()
		} else {
			a.entries = entries
			for _, e := range entries {
				if _, ok := a.checked[e.Path]; !ok {
					a.checked[e.Path] = e.Item.IsVersioned() && e.Item.IsInteresting()
				}
			}
		}
		a.checkDone()
	})
	runAsyncResult(a.win, func() ([]svn.LogEntry, error) {
		return a.client.Log(context.Background(), a.path, 20, true)
	}, func(logs []svn.LogEntry, err error) {
		if err != nil {
			a.logErr = err.Error()
		} else {
			a.logs = logs
		}
		a.checkDone()
	})
}

// checkDone clears the loading flag once all three loads have settled.
func (a *MainApp) checkDone() {
	if a.info == nil && a.infoErr == "" {
		return
	}
	if a.entries == nil && a.statusErr == "" {
		return
	}
	if a.logs == nil && a.logErr == "" {
		return
	}
	a.loading = false
}

func (a *MainApp) doCleanup() {
	a.loading = true
	runAsyncResult(a.win, func() (string, error) {
		return a.client.Cleanup(context.Background(), a.path)
	}, func(_ string, err error) {
		a.loading = false
		if err != nil {
			a.statusErr = err.Error()
			return
		}
		a.pending = &toastMsg{title: "清理完成", desc: "工作副本的锁已清除。", typ: feedback.ToastSuccess}
		a.loadAll()
	})
}

func (a *MainApp) doRevert() {
	a.loading = true
	runAsyncResult(a.win, func() (string, error) {
		return a.client.Revert(context.Background(), []string{a.path}, true)
	}, func(_ string, err error) {
		a.loading = false
		if err != nil {
			a.statusErr = err.Error()
			return
		}
		a.pending = &toastMsg{title: "还原完成", desc: "本地修改已还原。", typ: feedback.ToastSuccess}
		a.loadAll()
	})
}

func projectName(p string) string {
	b := filepath.Base(p)
	if b == "" || b == "." || b == string(filepath.Separator) {
		return p
	}
	return b
}
