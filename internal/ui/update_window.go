package gui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/zoo/gosvn/internal/svn"
)

// UpdateApp is the state of the update window.
type UpdateApp struct {
	client  *svn.Client
	path    string
	win     *mygo.Window
	output  string
	summary string
	err     string
	busy    bool
	pending *toastMsg
}

// OpenUpdateWindow opens the update window for path and starts updating.
func OpenUpdateWindow(client *svn.Client, path string) *mygo.Window {
	app := &UpdateApp{client: client, path: path}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:   baseTitle("Update", path),
		Width:   820,
		Height:  560,
		Content: ui.View(app.view),
	})
	app.win = win
	app.run()
	return win
}

func (a *UpdateApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(14).Children(func() {
		// Header
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.ArrowDownToLine()).FontSize(20).TextColor(t.Accent)
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, "更新工作副本").FontSize(18).Bold()
				ui.Text(c, a.path).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
			if a.busy {
				ui.Spinner(c)
			}
			if action.Button(c, "更新", action.ButtonProps{Variant: action.Primary}).Disabled(a.busy).Clicked() {
				a.run()
			}
		})

		if a.err != "" {
			feedback.Alert(c, a.err, feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
		}

		// Summary
		if a.summary != "" {
			typ := feedback.Success
			if strings.Contains(a.summary, "conflict") || strings.Contains(a.summary, "CONFLICT") {
				typ = feedback.Warning
			}
			feedback.Alert(c, a.summary, feedback.AlertProps{Type: typ, Variant: feedback.Soft})
		}

		// Output log card (grows)
		card(c, "输出日志", func() {
			ui.TextArea(c, &a.output).ReadOnly(true).Grow(1)
		}).Grow(1)
		toastHost(c)
	})
}

func (a *UpdateApp) run() {
	if a.busy {
		return
	}
	a.busy = true
	a.err = ""
	a.summary = "Updating..."
	a.output = ""
	runAsyncResult(a.win, func() (*svn.UpdateResult, error) {
		return a.client.Update(context.Background(), a.path)
	}, func(res *svn.UpdateResult, err error) {
		a.busy = false
		if err != nil {
			a.err = err.Error()
			a.summary = "Update failed."
			return
		}
		a.output = res.Output
		parts := []string{}
		if res.Revision > 0 {
			parts = append(parts, fmt.Sprintf("Updated to revision %d.", res.Revision))
		}
		if len(res.Added) > 0 {
			parts = append(parts, fmt.Sprintf("%d added", len(res.Added)))
		}
		if len(res.Updated) > 0 {
			parts = append(parts, fmt.Sprintf("%d updated", len(res.Updated)))
		}
		if len(res.Deleted) > 0 {
			parts = append(parts, fmt.Sprintf("%d deleted", len(res.Deleted)))
		}
		if len(res.Merged) > 0 {
			parts = append(parts, fmt.Sprintf("%d merged", len(res.Merged)))
		}
		if len(res.Conflicted) > 0 {
			parts = append(parts, fmt.Sprintf("%d CONFLICT", len(res.Conflicted)))
		}
		if len(parts) == 0 {
			parts = append(parts, "Already up to date.")
		}
		a.summary = strings.Join(parts, " · ")
		a.pending = &toastMsg{title: "更新完成", desc: a.summary, typ: feedback.ToastSuccess}
	})
}
