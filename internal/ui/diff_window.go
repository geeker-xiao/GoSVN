package gui

import (
	"context"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
	"github.com/zoo/gosvn/internal/svn"
)

// DiffApp is the state of the diff viewer window.
type DiffApp struct {
	client *svn.Client
	path   string
	rev    string // empty = working diff; otherwise passed to svn diff -r
	win    *mygo.Window
	text    string
	err     string
	busy    bool
	pending *toastMsg
}

// OpenDiffWindow opens a window showing the diff for path. If rev is empty the
// working-copy diff is shown; otherwise rev is passed to `svn diff -r`.
func OpenDiffWindow(client *svn.Client, path, rev string) *mygo.Window {
	app := &DiffApp{client: client, path: path, rev: rev}
	title := "Diff - " + path
	if rev != "" {
		title = "Diff " + rev + " - " + path
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:   title,
		Width:   900,
		Height:  640,
		Content: ui.View(app.view),
	})
	app.win = win
	app.load()
	return win
}

func (a *DiffApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(14).Children(func() {
		// Header
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, lucide.GitCompare()).FontSize(20).TextColor(t.Accent)
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, "差异对比").FontSize(18).Bold()
				sub := a.path
				if a.rev != "" {
					sub += "  ·  版本 " + a.rev
				}
				ui.Text(c, sub).FontSize(11).TextColor(t.TextMuted).SingleLine()
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

		// Diff content card (fills the window)
		card(c, "Diff", func() {
			ui.TextArea(c, &a.text).ReadOnly(true).Grow(1)
		}).Grow(1)
		toastHost(c)
	})
}

func (a *DiffApp) load() {
	if a.busy {
		return
	}
	a.busy = true
	a.err = ""
	runAsyncResult(a.win, func() (string, error) {
		return a.client.Diff(context.Background(), a.path, a.rev)
	}, func(text string, err error) {
		a.busy = false
		if err != nil {
			a.err = err.Error()
			a.text = ""
			return
		}
		a.text = text
		if text == "" {
			a.err = "No differences."
		}
	})
}
