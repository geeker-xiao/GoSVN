# GoSVN

一款用 Go 编写的原生桌面 SVN 客户端。基于 [`egoist/mygo`](https://github.com/egoist/mygo) native 模式构建，**不依赖 WebView，不使用 CGO**，单二进制即可运行。

界面风格参考 [lobe-mygo](https://github.com/lobehub/lobe-mygo) 设计系统，支持浅色 / 深色 / 跟随系统主题。

## 功能

- **工作副本状态**：查看变更文件列表，按状态（`M` / `A` / `D` / `?` / `!` / `C`）分类标记
- **提交（Commit）**：勾选要提交的文件、填写提交说明、一键提交
- **更新 / 清理 / 还原**：Update、Cleanup、Revert 常用操作
- **版本历史（Log）**：浏览提交记录（版本号、作者、时间、说明）并查看对应修订的差异
- **差异查看（Diff）**：工作副本差异与指定修订区间差异
- **检出新副本（Checkout）**：从远程仓库检出到本地目录
- **新建本地仓库**：通过 `svnadmin create` 创建 FSFS 仓库
- **浏览远程仓库**：通过 `svn list` 查看远程目录内容
- **最近打开项目**：启动首页展示历史项目，一键打开
- **系统主题**：浅色 / 深色 / 跟随系统，可在设置中切换
- **右键集成**：Windows 资源管理器 / Ubuntu Nautilus 右键菜单（`install` / `uninstall`）

## 环境要求

- **Go** 1.27.3 或更高
- **Subversion 命令行工具**：`svn`（新建仓库还需要 `svnadmin`）
  - **Windows**：安装 [TortoiseSVN](https://tortoisesvn.net/)（安装时务必勾选 `command line client tools`）或 [Apache Subversion](https://subversion.apache.org/)
  - **Ubuntu**：`sudo apt install subversion`

> 找不到 `svn` 时，GoSVN 会自动扫描常见安装路径（TortoiseSVN / SlikSVN / VisualSVN / Apache Subversion 等），也可以在「设置」中手动指定路径或点击「自动检测」。

## 构建

```bash
git clone https://github.com/geeker-xiao/GoSVN.git
cd GoSVN
go build -o gosvn ./cmd/gosvn
```

Windows：

```powershell
go build -o gosvn.exe ./cmd/gosvn
```

## 使用

```text
gosvn [command] [path]
```

| 命令 | 说明 |
| --- | --- |
| *(无参数)* | 打开项目首页；`gosvn PATH` 直接打开主窗口 |
| `commit` | 打开提交窗口（别名：`status`、`st`） |
| `update` | 打开更新窗口 |
| `log` | 打开版本日志窗口 |
| `diff` | 打开差异窗口 |
| `checkout` | 打开检出仓库对话框 |
| `create-repo` | 打开新建本地仓库对话框 |
| `remote` | 打开远程仓库浏览器 |
| `settings` | 打开设置窗口 |
| `install` | 注册右键菜单集成 |
| `uninstall` | 移除右键菜单集成 |
| `help`, `-h` | 显示帮助 |
| `version` | 显示版本 |

`path` 省略时默认使用当前目录 `.`。

示例：

```bash
gosvn                         # 打开首页
gosvn F:\work\myrepo          # 直接打开某个工作副本
gosvn log F:\work\myrepo      # 直接查看版本日志
```

## 配置

配置文件位于用户配置目录下的 `gosvn/settings.json`：

- **Windows**：`%APPDATA%\gosvn\settings.json`
- **Linux**：`~/.config/gosvn/settings.json`

可配置项：

| 字段 | 说明 |
| --- | --- |
| `themeMode` | 主题：`system` / `light` / `dark` |
| `svnBin` | `svn` 可执行文件路径，留空则自动检测 / 使用 PATH |
| `timeout` | 单次命令默认超时（秒），`0` 表示不限制 |

也支持以下环境变量：

| 环境变量 | 说明 |
| --- | --- |
| `GOSVN_SVN_BIN` | 覆盖 `svn` 可执行文件路径 |
| `GOSVN_SVN_TIMEOUT` | 覆盖默认超时（Go duration，如 `30s`） |

## 项目结构

```text
.
├── cmd/
│   └── gosvn/              # 程序入口：解析子命令与目标路径
├── internal/
│   ├── svn/                # SVN CLI 封装、XML 解析、领域模型
│   ├── ui/                 # MyGo 原生窗口、视图组件、状态绑定
│   └── platform/           # 跨平台路径处理、右键菜单集成
├── testdata/               # 样例 svn --xml 输出，用于解析器单测
├── LICENSE
└── README.md
```

## 开发

```bash
go vet ./...
go test ./...
go build ./cmd/gosvn
```

设计约定：

- `internal/svn` 不依赖 `internal/ui`，UI 层只调用其暴露的接口，不直接拼接 `exec.Command`
- 所有外部命令失败返回带上下文的错误（命令、退出码、`stderr` 摘要）
- 新增 `svn --xml` 解析逻辑时，在 `internal/svn` 补充单测并使用 `testdata/` 夹具
- 平台相关逻辑放在 `internal/platform`，用 build tags（`//go:build windows` / `//go:build linux`）隔离

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可协议。
