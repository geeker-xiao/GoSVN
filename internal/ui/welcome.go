package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"
	"github.com/zoo/gosvn/internal/svn"
)

// WelcomeApp is the project picker shown when GoSVN launches without a path.
// It mirrors MyGit's welcome window: a left drop/action column and a right
// list of recently opened working copies.
type WelcomeApp struct {
	client *svn.Client
	win    *mygo.Window
	onOpen func(path string)

	recent   []string
	errMsg   string
	dragging bool
	pending  *toastMsg
}

// OpenWelcomeWindow shows the project picker. onOpen is invoked (on the UI
// thread) with a valid working-copy path when the user picks one.
func OpenWelcomeWindow(client *svn.Client, onOpen func(path string)) *mygo.Window {
	w := &WelcomeApp{
		client: client,
		onOpen: onOpen,
		recent: loadRecent(),
	}
	win := mygo.NewWindow(customWindow(mygo.WindowOptions{
		Title:     "GoSVN",
		Width:     880,
		Height:    600,
		MinWidth:  720,
		MinHeight: 480,
		Content:   ui.View(w.view),
	}))
	w.win = win
	return win
}

func (w *WelcomeApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &w.pending)
	t := c.Theme()

	root := ui.Column(c).Fill().Children(func() {
		// Custom title bar
		titleBar(c).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
			Children(func() {
				ui.Icon(c, lucide.Database()).FontSize(16).TextColor(t.Accent)
				ui.Text(c, "GoSVN").FontSize(13).Bold()
				ui.Box(c).Grow(1)
				if action.ActionIcon(c, lucide.Settings(), action.ActionIconProps{Size: theme.Small, Title: "设置"}).Clicked() {
					OpenSettingsWindow()
				}
			})

		// Two-column body. AlignItems(Stretch) makes the right panel fill the
		// full height instead of being vertically centered.
		ui.Row(c).Grow(1).Basis(0).AlignItems(ui.Stretch).Children(func() {
			// Left: brand + drop zone + actions
			ui.Column(c).Grow(1).Basis(0).Padding(40).Gap(16).
				AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(c, lucide.Database()).FontSize(60).TextColor(t.Accent)
				ui.Text(c, "GoSVN").FontSize(28).Bold()

				dragBg := t.Surface
				if w.dragging {
					dragBg = t.Accent.Alpha(0.08)
				}
				dz := ui.ButtonBase(c).Padding(24).Border(1, t.Border).BorderStyle(ui.BorderDashed).
					Background(dragBg).Radius(12).Height(150).Width(320).
					AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Column(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, lucide.Folder()).FontSize(28).TextColor(t.TextMuted)
						ui.Text(c, "拖放文件夹到此处").FontSize(14).Bold()
						ui.Text(c, "或点击选择一个 SVN 工作副本").
							FontSize(11).TextColor(t.TextMuted)
					})
				})
				if dz.Clicked() {
					w.pick()
				}

				// Four action buttons in two rows, matching MyGit's welcome.
				ui.Row(c).Gap(8).Children(func() {
					if action.Button(c, "检出仓库…", action.ButtonProps{}).Clicked() {
						OpenCheckoutWindow(w.client, w.onOpen)
					}
					if action.Button(c, "新建仓库…", action.ButtonProps{}).Clicked() {
						OpenCreateRepoWindow(w.client)
					}
				})
				ui.Row(c).Gap(8).Children(func() {
					if action.Button(c, "查看远程仓库…", action.ButtonProps{}).Clicked() {
						OpenRemoteWindow(w.client)
					}
					if action.Button(c, "打开文件夹", action.ButtonProps{Variant: action.Primary}).Clicked() {
						w.pick()
					}
				})

				if w.errMsg != "" {
					errBanner(c, w.errMsg)
				}
			})

			// Right: recent projects — full-height panel starting at the top.
			ui.Column(c).Width(280).Gap(0).
				Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
				Children(func() {
				ui.Row(c).Padding(16, 16, 12, 16).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "最近打开").FontSize(13).Bold().Grow(1)
					if len(w.recent) > 0 && action.Button(c, "清除", action.ButtonProps{Size: theme.Small}).Clicked() {
						w.clearRecent()
					}
				})
				if len(w.recent) == 0 {
					ui.Column(c).Padding(16).Gap(6).Children(func() {
						ui.Text(c, "还没有打开过项目。").FontSize(12).TextColor(t.TextMuted)
						ui.Text(c, "拖入或选择一个 SVN 工作副本即可开始。").
							FontSize(11).TextColor(t.TextMuted)
					})
				} else {
					ui.Scroll(c).Grow(1).Basis(0).Children(func() {
						for i := range w.recent {
							w.recentRow(c, i)
						}
					})
				}
			})
		})
	})
	toastHost(c)

	w.dragging = root.FileDragOver()
	if paths := root.DroppedFiles(); len(paths) > 0 {
		w.open(paths[0])
	}
}

func (w *WelcomeApp) recentRow(c *ui.Context, i int) {
	t := c.Theme()
	p := w.recent[i]
	name := filepath.Base(p)
	if name == "" || name == "." {
		name = p
	}
	row := ui.Row(c).Padding(10, 12).Gap(10).AlignItems(ui.Center).Radius(8).Children(func() {
		ui.Icon(c, lucide.Folder()).FontSize(15).TextColor(t.TextMuted)
		ui.Column(c).Gap(1).Grow(1).Basis(0).Children(func() {
			ui.Text(c, name).FontSize(12).Bold().SingleLine()
			ui.Text(c, p).FontSize(10).TextColor(t.TextMuted).SingleLine()
		})
		if action.ActionIcon(c, lucide.X(), action.ActionIconProps{Size: theme.Small, Title: "移除"}).Clicked() {
			removeRecent(p)
			w.recent = loadRecent()
		}
	})
	if row.Clicked() {
		w.open(p)
	}
}

// pick opens the native folder picker.
func (w *WelcomeApp) pick() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    w.win,
			Title:     "选择 SVN 工作副本",
			Directory: true,
		})
		w.win.Update(func() {
			if err != nil {
				w.errMsg = fmt.Sprintf("无法选择文件夹：%v", err)
				return
			}
			if len(paths) > 0 {
				w.open(paths[0])
			}
		})
	}()
}

// open validates dir is a working copy, remembers it, and hands it to onOpen.
func (w *WelcomeApp) open(dir string) {
	dir = filepath.Clean(dir)
	if !w.isWorkingCopy(dir) {
		w.errMsg = fmt.Sprintf("%q 不是一个有效的 SVN 工作副本（未找到 .svn，且 svn info 失败）。", dir)
		return
	}
	w.errMsg = ""
	addRecent(dir)
	w.recent = loadRecent()
	if w.onOpen != nil {
		w.onOpen(dir)
	}
	w.win.Close()
}

func (w *WelcomeApp) isWorkingCopy(dir string) bool {
	if st, err := os.Stat(filepath.Join(dir, ".svn")); err == nil && st.IsDir() {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := w.client.Info(ctx, dir)
	return err == nil
}

func (w *WelcomeApp) clearRecent() {
	for _, p := range w.recent {
		removeRecent(p)
	}
	w.recent = nil
}
