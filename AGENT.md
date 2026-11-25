

## 1. 项目目标与定位

使用 **Go** 和 **`github.com/egoist/mygo`**（优先采用 `native` 模式）开发一款面向 **Ubuntu** 和 **Windows** 的桌面端 SVN 客户端，参考 **TortoiseSVN** 的高频交互习惯（工作副本状态查看、提交 Commit、更新 Update、版本日志 Log、基础差异 Diff），支持通过命令行参数直接唤起对应功能窗口。

---

## 2. 技术栈与核心约束

- **语言**：Go（使用 `go test ./...` 验证逻辑与解析器）。
- **GUI 框架**：`github.com/egoist/mygo`（`native` 模式为主）。
  - 避免引入重量级 WebView 作为主架构；保持单二进制构建与启动速度。
  - 若 MyGo native 缺少现成复杂表格或逐行 Diff 控件，优先基于现有布局/列表/文本组件封装复用，或提供导出/外部 Diff 工具调用兜底，不要过度发明复杂渲染引擎。
- **SVN 底层交互**：通过 `os/exec` 调用系统/内置 `svn` 命令行工具，优先使用 `--xml` 输出并通过 `encoding/xml` 解析。
  - **禁止**使用 CGO 绑定 `libsvn` C 库。
- **目标平台**：
  - Linux：**Ubuntu**（桌面环境以 GNOME / Nautilus 为主）。
  - Windows：**Windows 10 / 11**（文件资源管理器 Explorer）。

---

## 3. 目录结构约定

在现有 MyGo 工程基础上，按以下职责划分目录：

```text
.
├── AGENTS.md
├── cmd/
│   └── gosvn/              # 程序入口：解析子命令（status/commit/update/log/diff）与目标路径
├── internal/
│   ├── svn/                # SVN CLI 封装、命令执行器、XML 解析器、领域模型
│   ├── ui/                 # MyGo native 窗口、视图组件、状态绑定
│   └── platform/           # Ubuntu / Windows 路径处理、编码/环境适配、右键集成辅助
└── testdata/               # 样例 svn status --xml / svn log --xml / svn diff 输出

```

---

## 4. 核心功能里程碑（按优先级实现）

### M1：SVN CLI 封装与 XML 解析（先跑通无 UI 逻辑）

* 封装核心命令：
* `status`（支持 `--xml`）
* `commit`（支持指定文件列表 `-m` / `--targets`）
* `update`
* `log`（支持 `--xml`、限制条数 `--limit`、详细路径 `-v`）
* `diff`
* `info` / `revert` / `cleanup`


* 处理跨平台路径规范化与中文编码（UTF-8 / Windows 本地编码兼容），确保中文路径、中文文件名和中文提交日志不乱码。
* 所有 `svn` 子进程调用必须支持超时与取消（`context.Context`），并完整捕获 `stdout` / `stderr` 错误信息。

### M2：核心桌面窗口（MyGo Native）

* **Status / Commit 窗口**：
* 展示工作副本变更文件列表（状态如 `M` / `A` / `D` / `?` / `!`、路径）。
* 支持勾选要提交的文件、输入 Commit Message、触发提交并展示结果。


* **Update 窗口**：
* 展示更新进度与变更文件输出，支持错误提示。


* **Log 窗口**：
* 展示提交历史列表（版本号、作者、时间、提交说明），选中某条记录时展示关联变更文件与差异内容。



### M3：双平台系统集成与命令行入口

* 支持通过子命令直接打开对应窗口，例如：
* `gosvn commit <path>`
* `gosvn update <path>`
* `gosvn log <path>`
* 未传子命令时默认打开工作副本/主操作窗口。


* **Ubuntu（Nautilus）**：提供脚本/安装辅助，将选中的文件或目录路径传给 `gosvn`。
* **Windows（Explorer）**：提供注册表注册/卸载辅助（写入目录与文件右键菜单调用 `gosvn.exe`），注意路径含空格或中文时的引号转义。

---

## 5. 代码与工程规范

1. **关注点分离**：`internal/svn` 不得依赖 `internal/ui`；UI 层只调用 `internal/svn` 暴露的接口与结构体，不直接在视图代码中拼接 `exec.Command`。
2. **错误处理**：所有外部命令失败必须返回带上下文的错误（包含命令、退出码与 `stderr` 摘要），UI 层需能清晰展示给用户，禁止静默吞掉错误。
3. **测试优先**：每新增一种 `svn --xml` 输出解析或状态映射逻辑，必须在 `internal/svn` 中补充对应的单元测试，使用 `testdata/` 中的 XML 夹具验证。
4. **平台差异隔离**：平台专属逻辑（注册表、Nautilus 脚本、路径分隔符或编码处理）放在 `internal/platform` 并使用 Go build tags（如 `//go:build windows`、`//go:build linux`）隔离。

---

## 6. 交付与自检标准

每次完成功能或阶段性修改后，Agent 应自检以下内容：

1. `go vet ./...` 与 `go test ./...` 全部通过。
2. 构建产物在目标平台可正常编译（如 `go build ./cmd/gosvn`）。
3. 说明已修改的文件列表、新增的子命令/交互行为、仍存在的限制或待办项，以及在 Ubuntu / Windows 上的手动验证步骤。


