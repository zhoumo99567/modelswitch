# Model Switcher

一个 Windows / macOS 便携版配置工具，用于管理 OpenAI 兼容的本地模型服务，并切换官方 ChatGPT 应用里的 Codex 提供方或 pi agent 的默认模型。

双击 build/bin/model-switcher.exe，添加本地服务，填写 API Base URL（例如 http://127.0.0.1:1234/v1），点击“获取模型”，选择模型后点击“保存”保留连接，或点击右侧“应用到 ChatGPT / pi agent”保存并应用。左侧菜单用于选择要编辑的配置。

工具会从 /v1/models 读取模型列表，默认在程序同级的 `profiles.json` 读取和写入配置，在 Windows 使用 DPAPI、macOS 使用系统钥匙串安全保存 API Key，在用户目录的 `.codex/config.toml` 中写入本地 provider，切换前备份原始配置，并支持恢复切换前的 model 与 model_provider。

切换本地模型或“切回 OpenAI”时，工具会先关闭再重启官方 ChatGPT 应用。启动工具时会自动查找并显示当前 ChatGPT 路径。Windows 优先查找已安装应用和运行进程，并以开始菜单作为回退；macOS 通过 Spotlight 和常用应用目录查找 `ChatGPT.app`。点击路径可打开系统原生文件选择窗口（Windows 选择 `.exe` / `.lnk`，macOS 选择 `.app`），选择后自动保存，取消不会修改原设置。点击“自动查找”恢复自动模式；自动模式每次重新解析路径以适应应用更新。

界面底部可以打开 `.codex` 目录，或在工具内查看并编辑当前 `config.toml`。保存前会校验 TOML 并备份原文件；程序同级的 `backups` 目录保存配置备份，旧版 AppData 配置会自动迁移到程序目录。

界面采用左侧树形菜单、右侧详情页面。模型设置下列出保存的连接，技能管理下分为已安装、技能市场和回收区；右上角的应用选择同时决定模型和技能操作对象。左侧菜单可折叠，拖动分隔条调整宽度，也可用方向键微调、Home / End 到达上下限、双击恢复默认宽度。默认 256px，桌面最小 220px、最大 360px，并随窗口缩小保留主体空间；宽度保存在本机。

左下角显示当前服务、模型、应用状态和版本号，点击状态卡打开设置，可以切换中文 / English、浅色 / 深色主题，也可调整菜单栏宽度；选择会保存在本机。

窗口会记住上次正常关闭时的位置与大小。启动时按当前显示器的可用区域校验，窗口完整可见才恢复；显示器断开、分辨率变小或位置越界时，回到默认 1120 × 760 大小并居中（小屏幕会缩小到可用区域）。最大化、全屏或最小化时关闭，保留最近的普通窗口尺寸。窗口状态独立保存在用户配置目录的 `ModelSwitcher/window-state.json`，不跟随便携模型配置分发。

左侧“CLI 启动器”提供 Codex CLI 和 pi CLI 入口，显示检测到的命令路径。可以填写或选择工作目录，启动后在 macOS Terminal / Windows PowerShell 的独立窗口中运行对应 CLI；目录保存在本机。只启动已安装的 CLI，不自动安装。macOS 同时检查 PATH、常见安装目录、登录 shell 和 ChatGPT 附带的 Codex 命令。

左侧“高级设置”提供三组共享配置：记忆管理扫描 Codex 的 `~/.codex/memories` 和当前工作区的 `.codex/memories`，编辑、创建和删除都会保留备份；MCP 配置同时识别 Codex `config.toml` 中的 `[mcp_servers.*]` 与 pi agent 兼容版本的 `mcp.json`，展示服务器来源、命令、端点和启用状态；运行参数可以编辑 pi `settings.json` 或 Codex `config.toml`，写入前按 JSON/JSONC 或 TOML 校验并自动备份。当前 pi 版本如果没有启用 `mcp.json`，页面会显示文件未创建，不会替用户开启不受支持的配置。

设置中的“更新管理”默认跳过检查，因为当前没有配置更新源。部署 S3 后可通过环境变量 `MODELSWITCHER_UPDATE_URL` 指向清单，例如：`{"version":"0.2.0","windows":{"url":"https://.../ModelSwitcher.exe","sha256":"..."},"macos":{"url":"https://.../ModelSwitcher.app.zip","sha256":"..."}}`。下载后会校验 SHA-256；清单提供 `signature` 时，还会使用 `MODELSWITCHER_UPDATE_PUBLIC_KEY` 校验 Ed25519 签名。Windows 使用独立更新助手替换并重启 exe，macOS 替换并重新打开整个 app 包。

左侧“技能管理 → 技能市场”支持在 OpenAI Skills、Anthropic Skills、Vercel Skills、OpenAI Plugins 和 pi-skills（badlogic/pi-skills）中按名称、描述、作者搜索，可以选择单个市场或全部市场。OpenAI Skills 使用 `.curated` 目录；OpenAI Plugins 仅展示官方 marketplace 中声明可供 Codex 安装的技能。市场目录缓存 10 分钟；全部市场并行加载，先显示已返回的结果，进度条分别显示每个市场的加载状态；市场请求不会锁住页面导航或应用选择。每个市场的完整请求限时 45 秒，并合并重复请求；某个市场不可用时会显示该市场的错误，其他结果仍可使用。列表每页显示 36 项，搜索和排序在已返回的数据上执行。可以按安装量、仓库 Stars、名称和市场排序。安装量来自 skills.sh 的公开搜索接口，并严格匹配官方仓库与技能名称；它是该平台记录的安装次数，不是全网下载量。GitHub Stars 代表整个仓库的热度，不是单个技能的评价。当前市场没有统一的用户评分，显示“未提供”，并提供仓库反馈和统计来源入口。真实零安装量显示 0，缺失值不会冒充 0；未知数据排在已知数据之后。统计并行加载并缓存一小时，不影响技能目录加载。可展开查看描述、作者、许可、仓库、版本和来源链接；缺失元数据会显示“未提供”。

技能操作跟随顶部的应用选择：ChatGPT / Codex 扫描 `CODEX_HOME/skills`（默认 `~/.codex/skills`），pi agent 扫描 `PI_CODING_AGENT_DIR/skills`（默认 `~/.pi/agent/skills`）。已安装和回收区直接读取本地目录，离线可用。可以安装单个技能、批量清理所选技能、查看回收区并恢复；同名目录不会被覆盖，`.system` 受到保护。清理将技能移入当前目录的 `.trash`，保留恢复能力。这里只管理这两个全局目录，pi 包、项目技能和共享的 `~/.agents/skills` 仍由各自工具管理。

安装会锁定仓库提交版本，在临时目录校验路径、符号链接、重复文件、`SKILL.md`、单文件和总大小，再移动到技能目录。安装均按目录下载单个技能；Anthropic 和 OpenAI Plugins 的市场目录也按文件读取，其他市场的目录通过仓库 ZIP 读取。安装后保存来源记录，不自动运行技能脚本。

顶部选择“pi agent”后，已保存的服务配置可直接切换为 pi 的默认模型。工具保留 `models.json` 中其他 provider 和 `settings.json` 中其他设置，写入 OpenAI Chat Completions 兼容 provider 以及 `defaultProvider` / `defaultModel`；两份配置先备份到 pi 目录中的 `.model-switcher-backups`。API Key 仍使用系统安全存储，pi 通过工具的凭据命令读取。点击“恢复原配置”恢复切换前的 provider 和默认模型。可以在工具内编辑 `models.json`，支持 JSON/JSONC 校验并备份；写入时配置对象会规范化为 JSON，原始注释保存在备份中。视觉模型需在模型面板勾选“支持图像输入”、点击“应用到 pi agent”（会先保存修改）；该选项按模型保存，生成 `input: ["text", "image"]`，模型服务也必须接受图像请求。在现有 pi 会话中使用 `/model` 选择模型，新会话使用已保存的默认值；技能安装后使用 `/reload`。

用于 ChatGPT / Codex 的服务还需要兼容 /v1/responses；仅支持 /v1/chat/completions 时需要协议转换代理。pi agent 使用 /v1/chat/completions。

开发命令：`npm --prefix frontend install`；`wails dev`。当前 Wails CLI 与 Go 1.27 的绑定生成存在兼容问题时，使用 `GOTOOLCHAIN=go1.23.12 wails dev`。

验证：`go test -race ./...`、`npm --prefix frontend run build`。可选真实市场测试：`MODELSWITCHER_LIVE_TEST=1 go test -run TestLiveSkillMarkets -v`；同时设置 `MODELSWITCHER_LIVE_INSTALL_TEST=1` 可在临时目录验证实际下载与安装。

Windows 发布命令：`pwsh ./scripts/release.ps1 -Version 0.2.0`。脚本会使用 `-X main.AppVersion=...` 注入版本号，生成 `release/0.2.0/ModelSwitcher-0.2.0-windows-amd64.exe` 和 `latest.json`。

macOS 发布命令（需要在 macOS 主机安装 Xcode / WebKit 环境执行）：`./scripts/release-macos.sh 0.2.0`。它会生成 `.app.zip`、SHA-256 和合并后的 `latest.json`。

也可以在 macOS 上运行仓库内的 `./build-macos.sh 0.2.0`。

有 S3 地址后设置 `MODELSWITCHER_UPDATE_BASE_URL`（公开下载地址前缀）和 `MODELSWITCHER_S3_URI`（例如 `s3://bucket/model-switcher`），再执行 `pwsh ./scripts/release.ps1 -Version 0.2.0 -Publish`。脚本使用本机 AWS CLI 配置或环境凭据，不把密钥写入仓库；macOS 端设置 `PUBLISH=1` 后运行 `./scripts/release-macos.sh 0.2.0`。

Windows 发布文件：`build/bin/model-switcher.exe`，是便携版单文件 exe，不需要安装 Go、Node.js 或 Wails。Windows 10/11 通常已包含 WebView2。

macOS 发布文件：`build/bin/ModelSwitcher.app`。macOS 版本使用系统钥匙串保存 API Key；分发给其他用户时建议在 macOS 上签名并公证。

