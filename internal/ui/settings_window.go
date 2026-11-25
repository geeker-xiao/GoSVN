package gui

import (
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

// SettingsApp is the state of the preferences window.
type SettingsApp struct {
	win          *mygo.Window
	orig         Settings
	draft        Settings
	themeIdx     int // 0=system, 1=light, 2=dark; mirrors draft.ThemeMode
	detectedPath string
	err          string
	pending      *toastMsg
}

// OpenSettingsWindow opens the application preferences window.
func OpenSettingsWindow() *mygo.Window {
	s := GetSettings()
	app := &SettingsApp{orig: s, draft: s}
	switch s.ThemeMode {
	case "light":
		app.themeIdx = 1
	case "dark":
		app.themeIdx = 2
	}
	win := mygo.NewWindow(customWindow(mygo.WindowOptions{
		Title:     "设置",
		Width:     560,
		Height:    520,
		MinWidth:  480,
		MinHeight: 420,
		Content:   ui.View(app.view),
	}))
	app.win = win
	return win
}

func (a *SettingsApp) view(c *ui.Context) {
	applyTheme(c)
	flushToast(c, &a.pending)
	t := c.Theme()

	// Sync the segmented selection back into the draft every frame.
	switch a.themeIdx {
	case 1:
		a.draft.ThemeMode = "light"
	case 2:
		a.draft.ThemeMode = "dark"
	default:
		a.draft.ThemeMode = "system"
	}

	ui.Column(c).Fill().Children(func() {
		titleBar(c).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
			Children(func() {
				ui.Icon(c, lucide.Settings()).FontSize(16).TextColor(t.Accent)
				ui.Text(c, "设置").FontSize(13).Bold()
				ui.Box(c).Grow(1)
			})

		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(24).Gap(20).Children(func() {
				a.appearanceCard(c)
				a.svnCard(c)
				a.aboutCard(c)
			})
		})

		// Bottom bar
		ui.Row(c).Padding(12, 24).Gap(10).AlignItems(ui.Center).
			Background(t.Surface).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
			Children(func() {
				if a.err != "" {
					ui.Text(c, a.err).TextColor(t.Danger).FontSize(12).Grow(1).SingleLine()
				} else {
					ui.Text(c, a.dirtyHint()).TextColor(t.TextMuted).FontSize(11).Grow(1)
				}
				if action.Button(c, "取消", action.ButtonProps{}).Clicked() {
					a.win.Close()
				}
				if action.Button(c, "保存", action.ButtonProps{Variant: action.Primary}).Clicked() {
					a.save()
				}
			})
		toastHost(c)
	})
}

func (a *SettingsApp) appearanceCard(c *ui.Context) {
	card(c, "外观", func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, "主题模式").FontSize(12).TextColor(c.Theme().TextMuted)
			action.Segmented(c, &a.themeIdx, []string{"跟随系统", "浅色", "深色"}, action.SegmentedProps{Size: theme.Small})
		})
	})
}

func (a *SettingsApp) svnCard(c *ui.Context) {
	t := c.Theme()
	card(c, "SVN", func() {
		ui.Column(c).Gap(14).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "svn 可执行文件路径").FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					field.Input(c, &a.draft.SVNBin, field.InputProps{
						Placeholder: "留空则使用 PATH 中的 svn",
						Variant:     field.InputOutlined,
					}).Grow(1)
					if action.Button(c, "浏览…", action.ButtonProps{Size: theme.Small}).Clicked() {
						a.browseSVN()
					}
					if action.Button(c, "自动检测", action.ButtonProps{Size: theme.Small}).Clicked() {
						a.detectSVN()
					}
				})
				if a.detectedPath != "" {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, lucide.Check()).FontSize(12).TextColor(t.Success)
						ui.Text(c, "检测到: "+a.detectedPath).FontSize(11).TextColor(t.Success).SingleLine()
					})
				}
			})

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "默认超时（秒）").FontSize(12).TextColor(t.TextMuted)
				timeout := float64(a.draft.Timeout)
				field.InputNumber(c, &timeout, field.InputNumberProps{
					Min:         0,
					Max:         3600,
					Step:        10,
					Placeholder: "0 表示不限制",
					Variant:     field.InputOutlined,
				})
				a.draft.Timeout = int(timeout)
			})
		})
	})
}

func (a *SettingsApp) aboutCard(c *ui.Context) {
	t := c.Theme()
	card(c, "关于", func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "GoSVN v0.1.0").FontSize(14).Bold()
			ui.Text(c, "原生桌面 SVN 客户端，不依赖 WebView / cgo").
				FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "基于 github.com/egoist/mygo + lobe-mygo").
				FontSize(11).TextColor(t.TextMuted)
		})
	})
}

func (a *SettingsApp) dirtyHint() string {
	if a.draft == a.orig {
		return ""
	}
	return "有未保存的更改"
}

func (a *SettingsApp) browseSVN() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  "选择 svn 可执行文件",
		})
		a.win.Update(func() {
			if err != nil {
				a.err = fmt.Sprintf("选择文件失败：%v", err)
				return
			}
			if len(paths) > 0 {
				a.draft.SVNBin = paths[0]
				a.err = ""
			}
		})
	}()
}

func (a *SettingsApp) detectSVN() {
	a.detectedPath = ""
	if p := svn.Detect(); p != "" {
		a.detectedPath = p
		a.draft.SVNBin = p
		a.err = ""
	} else {
		a.err = "未检测到 svn。请安装 TortoiseSVN（勾选 command line client tools）或手动指定路径。"
	}
}

func (a *SettingsApp) save() {
	if err := saveSettings(a.draft); err != nil {
		a.err = fmt.Sprintf("保存设置失败：%v", err)
		return
	}
	a.orig = a.draft
	a.err = ""
	a.pending = &toastMsg{title: "设置已保存", desc: "新设置将在所有窗口生效。", typ: feedback.ToastSuccess}
}
