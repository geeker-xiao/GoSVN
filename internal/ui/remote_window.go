package gui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
	"github.com/zoo/gosvn/internal/svn"
)

// RemoteApp is the state of the remote repository browser.
type RemoteApp struct {
	client *svn.Client
	win    *mygo.Window

	url     string
	entries []svn.RemoteEntry
	err     string
	busy    bool
	list    ui.ListState
	pending *toastMsg
}

// OpenRemoteWindow opens the remote repository browser.
func OpenRemoteWindow(client *svn.Client) *mygo.Window {
	app := &RemoteApp{client: client}
	win := mygo.NewWindow(customWindow(mygo.WindowOptions{
		Title:     "查看远程仓库",
		Width:     640,
		Height:    480,
		MinWidth:  480,
		MinHeight: 360,
		Content:   ui.View(app.view),
	}))
	app.win = win
	return win
}

func (a *RemoteApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		titleBar(c).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
			Children(func() {
				ui.Icon(c, lucide.Globe()).FontSize(16).TextColor(t.Accent)
				ui.Text(c, "查看远程仓库").FontSize(13).Bold()
				ui.Box(c).Grow(1)
			})

		ui.Column(c).Grow(1).Basis(0).Padding(24).Gap(16).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "仓库地址 (URL)").FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					field.Input(c, &a.url, field.InputProps{
						Placeholder: "https://svn.example.com/svn/repo",
						Variant:     field.InputOutlined,
					}).Grow(1)
					if action.Button(c, "查看", action.ButtonProps{Variant: action.Primary, Size: theme.Small}).Disabled(a.busy || a.url == "").Clicked() {
						a.load()
					}
				})
			})

			if a.err != "" {
				feedback.Alert(c, a.err, feedback.AlertProps{Type: feedback.Error, Variant: feedback.Soft})
			}

			// Result list
			ui.Column(c).Grow(1).Basis(0).Radius(8).Border(1, t.Border).
				Background(t.Surface).Children(func() {
				switch {
				case a.busy:
					ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Spinner(c)
						ui.Text(c, "加载中…").TextColor(t.TextMuted)
					})
				case a.entries == nil && a.err == "" && a.url == "":
					ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Icon(c, lucide.Globe()).FontSize(34).TextColor(t.TextMuted)
						ui.Text(c, "输入仓库地址并点击查看").TextColor(t.TextMuted).Padding(10)
					})
				case len(a.entries) == 0:
					ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Text(c, "仓库为空或无法访问").TextColor(t.TextMuted)
					})
				default:
					ui.List(c, &a.list, len(a.entries), func(i int) {
						e := a.entries[i]
						icon := lucide.File()
						if e.Kind == "dir" {
							icon = lucide.Folder()
						}
						ui.Row(c).Gap(10).Padding(8, 12).AlignItems(ui.Center).
							BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
							ui.Icon(c, icon).FontSize(15).TextColor(t.TextMuted)
							ui.Text(c, e.Name).Grow(1).SingleLine()
							if e.Kind == "dir" {
								ui.Text(c, "目录").FontSize(11).TextColor(t.TextMuted)
							} else {
								ui.Text(c, "文件").FontSize(11).TextColor(t.TextMuted)
							}
						})
					}).Grow(1).Basis(0)
				}
			})
		})
		toastHost(c)
	})
}

func (a *RemoteApp) load() {
	a.busy = true
	a.err = ""
	a.entries = nil
	runAsyncResult(a.win, func() ([]svn.RemoteEntry, error) {
		return a.client.ListRemote(context.Background(), a.url)
	}, func(entries []svn.RemoteEntry, err error) {
		a.busy = false
		if err != nil {
			a.err = err.Error()
			return
		}
		a.entries = entries
		a.pending = &toastMsg{title: "加载完成", desc: fmt.Sprintf("共 %d 项", len(entries)), typ: feedback.ToastSuccess}
	})
}
