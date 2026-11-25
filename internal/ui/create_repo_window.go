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

// CreateRepoApp is the state of the create-repository window.
type CreateRepoApp struct {
	client *svn.Client
	win    *mygo.Window

	path    string
	busy    bool
	err     string
	pending *toastMsg
}

// OpenCreateRepoWindow opens the create-local-repository dialog.
func OpenCreateRepoWindow(client *svn.Client) *mygo.Window {
	app := &CreateRepoApp{client: client}
	win := mygo.NewWindow(customWindow(mygo.WindowOptions{
		Title:     "新建仓库",
		Width:     520,
		Height:    280,
		MinWidth:  480,
		MinHeight: 240,
		Content:   ui.View(app.view),
	}))
	app.win = win
	return win
}

func (a *CreateRepoApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		titleBar(c).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
			Children(func() {
				ui.Icon(c, lucide.FolderPlus()).FontSize(16).TextColor(t.Accent)
				ui.Text(c, "新建仓库").FontSize(13).Bold()
				ui.Box(c).Grow(1)
			})

		ui.Column(c).Grow(1).Basis(0).Padding(24).Gap(16).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "仓库位置").FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					field.Input(c, &a.path, field.InputProps{
						Placeholder: "选择或输入新仓库目录",
						Variant:     field.InputOutlined,
					}).Grow(1)
					if action.Button(c, "浏览…", action.ButtonProps{Size: theme.Small}).Clicked() {
						a.browse()
					}
				})
				ui.Text(c, "将在指定目录创建一个新的 FSFS 格式 SVN 仓库").
					FontSize(11).TextColor(t.TextMuted)
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
					ui.Text(c, "创建中…").FontSize(12).TextColor(t.TextMuted).Grow(1)
				} else {
					ui.Box(c).Grow(1)
				}
				if action.Button(c, "取消", action.ButtonProps{}).Clicked() {
					a.win.Close()
				}
				if action.Button(c, "创建", action.ButtonProps{Variant: action.Primary}).Disabled(a.busy || a.path == "").Clicked() {
					a.run()
				}
			})
		toastHost(c)
	})
}

func (a *CreateRepoApp) browse() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.win,
			Title:     "选择仓库目录",
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

func (a *CreateRepoApp) run() {
	a.busy = true
	a.err = ""
	runAsyncResult(a.win, func() (string, error) {
		return a.client.CreateRepository(context.Background(), a.path)
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
		a.pending = &toastMsg{title: "仓库已创建", desc: abs, typ: feedback.ToastSuccess}
	})
}
