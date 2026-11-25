package gui

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
	"github.com/zoo/gosvn/internal/svn"
)

// CheckoutApp is the state of the checkout window.
type CheckoutApp struct {
	client *svn.Client
	win    *mygo.Window
	onOpen func(path string)

	url     string
	path    string
	busy    bool
	err     string
	pending *toastMsg
}

// OpenCheckoutWindow opens the repository checkout dialog.
func OpenCheckoutWindow(client *svn.Client, onOpen func(path string)) *mygo.Window {
	app := &CheckoutApp{client: client, onOpen: onOpen}
	win := mygo.NewWindow(customWindow(mygo.WindowOptions{
		Title:     "检出仓库",
		Width:     520,
		Height:    320,
		MinWidth:  480,
		MinHeight: 280,
		Content:   ui.View(app.view),
	}))
	app.win = win
	return win
}

func (a *CheckoutApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		titleBar(c).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
			Children(func() {
				ui.Icon(c, lucide.Download()).FontSize(16).TextColor(t.Accent)
				ui.Text(c, "检出仓库").FontSize(13).Bold()
				ui.Box(c).Grow(1)
			})

		ui.Column(c).Grow(1).Basis(0).Padding(24).Gap(16).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "仓库地址 (URL)").FontSize(12).TextColor(t.TextMuted)
				field.Input(c, &a.url, field.InputProps{
					Placeholder: "https://svn.example.com/svn/repo/trunk",
					Variant:     field.InputOutlined,
				})
			})

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "本地目录").FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					field.Input(c, &a.path, field.InputProps{
						Placeholder: "选择或输入本地目录",
						Variant:     field.InputOutlined,
					}).Grow(1)
					if action.Button(c, "浏览…", action.ButtonProps{Size: theme.Small}).Clicked() {
						a.browse()
					}
				})
			})

			if a.err != "" {
				feedback.Alert(c, a.err, feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
			}
		})

		ui.Row(c).Padding(12, 24).Gap(10).AlignItems(ui.Center).
			Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
			Children(func() {
				if a.busy {
					ui.Spinner(c)
					ui.Text(c, "检出中…").FontSize(12).TextColor(t.TextMuted).Grow(1)
				} else {
					ui.Box(c).Grow(1)
				}
				if action.Button(c, "取消", action.ButtonProps{}).Clicked() {
					a.win.Close()
				}
				if action.Button(c, "检出", action.ButtonProps{Variant: action.Primary}).Disabled(a.busy || a.url == "" || a.path == "").Clicked() {
					a.run()
				}
			})
		toastHost(c)
	})
}

func (a *CheckoutApp) browse() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.win,
			Title:     "选择本地目录",
			Directory: true,
		})
		a.win.Update(func() {
			if err != nil {
				a.err = fmt.Sprintf("选择目录失败：%v", err)
				return
			}
			if len(paths) > 0 {
				a.path = paths[0]
				a.err = ""
			}
		})
	}()
}

func (a *CheckoutApp) run() {
	a.busy = true
	a.err = ""
	runAsyncResult(a.win, func() (string, error) {
		return a.client.Checkout(context.Background(), a.url, a.path)
	}, func(out string, err error) {
		a.busy = false
		if err != nil {
			a.err = err.Error()
			return
		}
		abs, _ := filepath.Abs(a.path)
		if abs == "" {
			abs = a.path
		}
		a.pending = &toastMsg{title: "检出完成", desc: abs, typ: feedback.ToastSuccess}
		if a.onOpen != nil {
			a.onOpen(abs)
		}
		a.win.Close()
	})
}
