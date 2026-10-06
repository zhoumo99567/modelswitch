# Model Switcher

一个 Windows / macOS 便携版配置工具，用于管理 OpenAI 兼容的本地模型服务，并切换官方 ChatGPT 应用里的 Codex 提供方或 pi agent 的默认模型。

双击 build/bin/model-switcher.exe，添加本地服务，填写 API Base URL（例如 http://127.0.0.1:1234/v1），点击“获取模型”，选择模型后点击“保存”保留连接，或点击右侧“应用到 ChatGPT / pi agent”保存并应用。左侧菜单用于选择要编辑的配置。

工具会从 /v1/models 读取模型列表，默认在程序同级的 `profiles.json` 读取和写入配置，API Key 直接保存在该文件中；界面默认用密码点隐藏，支持临时显示和复制。在用户目录的 `.codex/config.toml` 中写入本地 provider，切换前备份原始配置，并支持恢复切换前的 model 与 model_provider。

切换本地模型或“切回 OpenAI”时，工具会先关闭再重启官方 ChatGPT 应用。启动工具时会自动查找并显示当前 ChatGPT 路径。Windows 优先查找已安装应用和运行进程，并以开始菜单作为回退；macOS 通过 Spotlight 和常用应用目录查找 `ChatGPT.app`。点击路径可打开系统原生文件选择窗口（Windows 选择 `.exe` / `.lnk`，macOS 选择 `.app`），选择后自动保存，取消不会修改原设置。点击“自动查找”恢复自动模式；自动模式每次重新解析路径以适应应用更新。

界面底部可以打开 `.codex` 目录，或在工具内查看并编辑当前 `config.toml`。保存前会校验 TOML 并备份原文件；程序同级的 `backups` 目录保存配置备份，旧版 AppData 配置会自动迁移到程序目录。

界面采用左侧树形菜单、右侧详情页面。模型设置下列出保存的连接，技能管理下分为已安装、技能市场和回收区；右上角的应用选择同时决定模型和技能操作对象。左侧菜单可折叠，拖动分隔条调整宽度，也可用方向键微调、Home / End 到达上下限、双击恢复默认宽度。默认 256px，桌面最小 220px、最大 360px，并随窗口缩小保留主体空间；宽度保存在本机。

左下角显示当前服务、模型、应用状态和版本号，点击状态卡打开设置，可以切换中文 / English、浅色 / 深色主题，也可调整菜单栏宽度；选择会保存在本机。

窗口会记住上次正常关闭时的位置与大小。启动时按当前显示器的可用区域校验，窗口完整可见才恢复；显示器断开、分辨率变小或位置越界时，回到默认 1120 × 760 大小并居中（小屏幕会缩小到可用区域）。最大化、全屏或最小化时关闭，保留最近的普通窗口尺寸。窗口状态独立保存在用户配置目录的 `ModelSwitcher/window-state.json`，不跟随便携模型配置分发。

左侧“CLI 启动器”提供 Codex CLI 和 pi CLI 入口，显示检测到的命令路径。可以填写或选择工作目录，启动后在 macOS Terminal / Windows PowerShell 的独立窗口中运行对应 CLI；目录保存在本机。启动时会自动检测 Node.js、npm、pi agent 和 Codex CLI；缺失或版本不兼容时，可以在启动器的“环境检测与安装”区域点击安装或修复。Node.js/npm 按平台安装：Windows 优先使用 winget 的 `OpenJS.NodeJS.LTS`，macOS 优先使用已有 Homebrew 的 `node@24`；包管理器不存在、安装失败或安装后检测不通过时，回退到当前用户的 `ModelSwitcher/tools` 目录下载官方兼容 LTS 安装包，并按官方 SHA-256 清单校验。系统包管理器可能请求系统授权；用户目录回退不需要管理员权限，也不要求先安装 Homebrew 或 winget。npm 随 Node.js 一起安装；pi/Codex 的 npm 全局 prefix 位于当前用户目录。安装 pi/Codex 会先补齐 Node.js/npm，再从官方 npm registry 安装（pi 使用兼容当前内置对话的 1.x，Codex 使用最新稳定版）。安装有进度、失败原因和重试入口，完成后自动检测；从本工具启动 CLI 和内置 pi 对话会自动使用这些路径。用户目录回退及 npm prefix 不会写入系统 PATH；若在其他终端直接使用命令，需要自行把界面显示的 Node.js 与 npm bin 目录加入 PATH。macOS 同时检查 PATH、常见安装目录、登录 shell 和 ChatGPT 附带的 Codex 命令。

左侧“高级设置”提供三组共享配置：记忆管理使用 `~/.agents/memory` 共享记忆库，全局记忆保存在 `global/`，项目记忆按工作区路径分目录保存在 `projects/`，`INDEX.md` 自动生成；Codex 的 `~/.codex/memories` 只读展示，可以提炼到共享记忆。编辑和删除共享记忆前都会保留备份。MCP 配置同时识别 Codex `config.toml` 中的 `[mcp_servers.*]` 与 pi agent 兼容版本的 `mcp.json`，展示服务器来源、命令、端点和启用状态；运行参数可以编辑 pi `settings.json` 或 Codex `config.toml`，写入前按 JSON/JSONC 或 TOML 校验并自动备份。当前 pi 版本如果没有启用 `mcp.json`，页面会显示文件未创建，不会替用户开启不受支持的配置。

创建、修改或删除共享记忆，以及打开高级设置或从启动器启动 pi CLI 时，工具会自动安装 `PI_CODING_AGENT_DIR/extensions/model-switcher-memory.js`（默认 `~/.pi/agent/extensions/`）。扩展在每次提问前读取全局记忆及 CLI 当前目录和父目录对应的项目记忆，优先加载 `soul.md` 中的名字和身份，不会混入其他项目的记忆；直接运行 `pi` 也适用。已有 Pi 会话首次接入后执行 `/reload`，之后记忆修改或删除在下次提问时生效。关闭扩展加载（例如 `--no-extensions`）会关闭此功能。记忆正文总读取量限于 64 KiB，单条最多读取 16 KiB，超出时提示代理按路径读取完整内容。工具保留用户已有的 `AGENTS.md`、`SYSTEM.md`、`APPEND_SYSTEM.md` 和 settings；Codex 仍需在全局指令中指向共享索引。

设置中的“更新管理”默认从 GitHub Releases 的固定入口 `https://github.com/zhoumo99567/modelswitch/releases/latest/download/latest.json` 检查最新正式版，预发布版本不进入这个更新入口。首次 Release 发布前，检查会提示无法读取清单。可以通过环境变量 `MODELSWITCHER_UPDATE_URL` 覆盖更新源，清单格式仍为 `{"version":"0.2.0","windows":{"url":"https://.../ModelSwitcher.exe","sha256":"..."},"macos":{"url":"https://.../ModelSwitcher.app.zip","sha256":"..."}}`。下载流式写入临时文件并校验 SHA-256；清单提供 `signature` 时，还会使用 `MODELSWITCHER_UPDATE_PUBLIC_KEY` 校验 Ed25519 签名。更新助手等待原程序退出后，在应用所在目录暂存新版，再替换并重启；替换或启动失败会恢复旧版，失败原因保存在程序或 app 同级的 `<程序路径>.update.log`。更新不会覆盖同级的 `profiles.json`。默认更新源为空的旧版本需要先手动安装一次新版。

左侧“技能管理 → 技能市场”支持在 OpenAI Skills、Anthropic Skills、Vercel Skills、OpenAI Plugins 和 pi-skills（badlogic/pi-skills）中按名称、描述、作者搜索，可以选择单个市场或全部市场。OpenAI Skills 使用 `.curated` 目录；OpenAI Plugins 仅展示官方 marketplace 中声明可供 Codex 安装的技能。市场目录缓存 10 分钟；全部市场并行加载，先显示已返回的结果，进度条分别显示每个市场的加载状态；市场请求不会锁住页面导航或应用选择。每个市场的完整请求限时 45 秒，并合并重复请求；某个市场不可用时会显示该市场的错误，其他结果仍可使用。列表每页显示 36 项，搜索和排序在已返回的数据上执行。可以按安装量、仓库 Stars、名称和市场排序。安装量来自 skills.sh 的公开搜索接口，并严格匹配官方仓库与技能名称；它是该平台记录的安装次数，不是全网下载量。GitHub Stars 代表整个仓库的热度，不是单个技能的评价。当前市场没有统一的用户评分，显示“未提供”，并提供仓库反馈和统计来源入口。真实零安装量显示 0，缺失值不会冒充 0；未知数据排在已知数据之后。统计并行加载并缓存一小时，不影响技能目录加载。可展开查看描述、作者、许可、仓库、版本和来源链接；缺失元数据会显示“未提供”。

技能操作跟随顶部的应用选择：ChatGPT / Codex 扫描 `CODEX_HOME/skills`（默认 `~/.codex/skills`），pi agent 扫描 `PI_CODING_AGENT_DIR/skills`（默认 `~/.pi/agent/skills`）。已安装和回收区直接读取本地目录，离线可用。可以安装单个技能、批量清理所选技能、查看回收区并恢复；同名目录不会被覆盖，`.system` 受到保护。清理将技能移入当前目录的 `.trash`，保留恢复能力。这里只管理这两个全局目录，pi 包、项目技能和共享的 `~/.agents/skills` 仍由各自工具管理。

安装会锁定仓库提交版本，在临时目录校验路径、符号链接、重复文件、`SKILL.md`、单文件和总大小，再移动到技能目录。安装均按目录下载单个技能；Anthropic 和 OpenAI Plugins 的市场目录也按文件读取，其他市场的目录通过仓库 ZIP 读取。安装后保存来源记录，不自动运行技能脚本。

顶部选择“pi agent”后，已保存的服务配置可直接切换为 pi 的默认模型。工具保留 `models.json` 中其他 provider 和 `settings.json` 中其他设置，写入 OpenAI Chat Completions 兼容 provider 以及 `defaultProvider` / `defaultModel`；两份配置先备份到 pi 目录中的 `.model-switcher-backups`。API Key 直接从本地配置读取，pi 通过工具的凭据命令读取。点击“恢复原配置”恢复切换前的 provider 和默认模型。可以在工具内编辑 `models.json`，支持 JSON/JSONC 校验并备份；写入时配置对象会规范化为 JSON，原始注释保存在备份中。视觉模型需在模型面板勾选“支持图像输入”、点击“应用到 pi agent”（会先保存修改）；该选项按模型保存，生成 `input: ["text", "image"]`，模型服务也必须接受图像请求。在现有 pi 会话中使用 `/model` 选择模型，新会话使用已保存的默认值；技能安装后使用 `/reload`。

用于 ChatGPT / Codex 的服务还需要兼容 /v1/responses；仅支持 /v1/chat/completions 时需要协议转换代理。pi agent 使用 /v1/chat/completions。

选择顶部的“pi agent”后，左侧显示“对话”。页面通过本机 pi CLI 的 `@earendil-works/pi-coding-agent` SDK 运行完整 AgentSession，内部使用 `pi-agent-core` 管理多轮上下文。采用类似 ChatGPT 的消息区与底部输入框，支持 Markdown 回复、实时流式显示、停止生成和新对话。先在模型设置中“应用到 pi agent”，聊天即可使用实际 `settings.json` / `models.json` 中的当前模型、服务地址和凭据；也可在聊天页直接切换已保存的配置，切换后自动开始新对话。当前支持 OpenAI Chat Completions 和 Responses 协议。Enter 发送，Shift + Enter 换行，中文输入法确认候选词不会误发送。对话使用内存会话，不写入 pi CLI 会话记录；凭据仅通过 Go 到 Node.js 的私有管道传递，不进入前端或命令行参数。内置对话需要已安装的 pi CLI 1.x npm 包和兼容的 Node.js（pi 1.x 要求 22.19 或更高版本）；可以在“CLI 启动器 → 环境检测与安装”中安装或修复这些依赖；启动时只检测，不会自动下载安装。

进入对话时默认使用 Pi 的 `DefaultResourceLoader` 加载 `PI_CODING_AGENT_DIR`（默认 `~/.pi/agent/`）中的全局 skills、extensions、提示词及 settings 中配置的资源和 packages，遵循 Pi 的启用、排除和冲突规则。工作目录默认为用户主目录。扩展的 `session_start`、`before_agent_start`、输入处理、工具调用和其他生命周期钩子实际执行，注册的工具可由模型调用；CLI 内置的 MCP、codemode 和 tool_search 扩展也按当前 Pi SDK 和全局设置加载。已配置共享记忆库时自动加载记忆桥，记忆修改在下次提问前读取。页面底部可查看加载的 skills、extensions、工具及错误，工具调用显示执行状态。支持 `/skill:名称`、扩展命令及 `/reload`；新增或修改资源可通过 `/reload` 重新加载。支持扩展的确认、输入、选择、多行编辑、通知、状态和文本小组件；终端专用 TUI 组件和 CLI 历史会话切换、分支操作需在 pi CLI 中使用。

输入框支持粘贴截图、图片及文件，也可以拖入文件或点击附件按钮选择。Windows 资源管理器复制的文件通过系统剪贴板读取。发送前显示附件预览并可逐个移除；只发送附件也可以，失败或停止后保留草稿方便重试。PNG、JPEG、WebP、GIF 使用模型原生图像输入，需要在模型设置中开启“支持图像输入”并应用；文本和代码文件、PDF、DOCX 在本机提取正文后随消息发送。不支持扫描 PDF 的 OCR 或其他二进制文件，扫描文档可粘贴页面截图。每条消息最多 8 个附件，单个不超过 8 MB，合计不超过 12 MB；每个文件最多提取 120,000 字符，PDF 最多读取前 100 页，截取时会在预览中提示。对话请求（含历史图片）总大小限制为 32 MB，超出时可减少附件或开启新对话。

模型生成中仍可输入、粘贴附件并发送，后续消息默认加入队列，显示在输入框上方；前端保留可编辑队列，等待 Pi AgentSession 完成后按顺序逐条提交，每条输入均经过扩展输入处理和提问钩子。点击某条消息的“引导”会中断当前生成，保留已有上下文和部分回答，并优先处理该条消息，其余消息继续排队。队列支持删除、“更多 → 编辑消息”和移到队首；编辑时暂时暂停队列，原输入框草稿及附件保持不变。停止生成或请求失败后队列暂停，可点击“继续发送”；失败的当前消息恢复为草稿，已有新草稿时则放回暂停队列。配置变化后保留未完成及待发送消息并暂停，确认新配置后可继续；“新对话”清空队列。最多保留 20 条待发送消息。每次交接会等待 Pi 的运行或停止完成，避免连续发送或立即引导时发生重叠请求。

开发命令：`npm --prefix frontend install`；`wails dev`。当前 Wails CLI 与 Go 1.27 的绑定生成存在兼容问题时，使用 `GOTOOLCHAIN=go1.23.12 wails dev`。

验证：`go test -race ./...`、`npm --prefix frontend test`、`npm --prefix frontend run build`。前端测试覆盖附件解析、pi-agent-core 队列/引导和 Wails 传输交接。可选真实依赖安装测试：`MODELSWITCHER_LIVE_DEPENDENCY_TEST=1 go test -run TestLiveDependencyInstallation -v`，在临时用户目录下载、校验并安装 Node.js/npm、pi 和 Codex，验证可运行后清理。可选真实市场测试：`MODELSWITCHER_LIVE_TEST=1 go test -run TestLiveSkillMarkets -v`；同时设置 `MODELSWITCHER_LIVE_INSTALL_TEST=1` 可在临时目录验证实际下载与安装。

GitHub 自动发布由 `.github/workflows/release.yml` 管理。先修改根目录 `VERSION` 并提交，推送与其一致的标签（例如 `VERSION` 为 `0.2.0` 时执行 `git tag v0.2.0`、`git push origin v0.2.0`）。Actions 使用 Go 1.23.12、Node.js 22.22.2 和 Wails CLI 2.12.0，运行前端测试、前端构建和 Go race 测试，再分别构建 Windows amd64 与 macOS universal。两边成功后统一生成 `latest.json` 和 `SHA256SUMS.txt`，把两个安装包及清单上传到草稿 Release，最后发布。正式版由 GitHub 自动决定 Latest；预发布标签（如 `v0.2.0-rc.1`）会明确标为 prerelease，且不会成为 Latest。已发布版本禁止覆盖，失败留下的草稿可以重跑。

在 GitHub Actions 页手动运行 Build and release，默认只构建完整下载包，不创建 Release；勾选 `publish` 时必须选择已存在、且与 `VERSION` 一致的版本标签。完整包在工作流的 `release-bundle` artifact 中保留 7 天，正式 Release 附件用于长期下载与应用更新。发布使用 Actions 自带的 `GITHUB_TOKEN` 和 `contents: write` 权限，不需要 S3 或额外的 GitHub 个人令牌。面向普通用户的更新入口需要公开仓库；源码私有时可使用另一个公开分发仓库。macOS 当前产物未配置开发者签名或公证，首次安装仍需按系统提示允许打开。

Windows 本地发布命令：`pwsh ./scripts/release.ps1`（默认读取 `VERSION`），也可显式传入 `-Version 0.2.0`。脚本会使用 `-X main.AppVersion=...` 注入版本号，并通过程序的 `--version` 校验实际版本，生成 `release/<version>/ModelSwitcher-<version>-windows-amd64.exe`、`latest.json` 和 `SHA256SUMS.txt`。没有下载地址配置时只生成本地清单，清单的 URL 留空，不上传。

macOS 本地发布命令（需要在 macOS 主机安装 Xcode / WebKit 环境执行）：`zsh ./scripts/release-macos.sh`，也可传入 `0.2.0`。它会生成 `release/<version>/ModelSwitcher-<version>-macos-universal.app.zip`、`SHA256SUMS.txt` 和合并后的 `latest.json`。两个本地脚本共用 `scripts/release-manifest.mjs`，执行 `node --test scripts/release-manifest.test.mjs` 可验证标签、清单与校验文件。Actions 在独立目录构建并汇总，避免两个平台各自覆盖更新清单。

手动汇总 GitHub 产物时，可将 `MODELSWITCHER_RELEASE_DOWNLOAD_BASE_URL` 设为完整版本目录（如 `https://github.com/zhoumo99567/modelswitch/releases/download/v0.2.0`），然后执行 `node scripts/release-manifest.mjs manifest 0.2.0 release/0.2.0 --require-all`；缺少任一平台安装包会停止生成。Actions 构建时还通过 `MODELSWITCHER_BUILD_UPDATE_URL` 注入当前仓库的默认清单地址，fork 的产物会使用 fork 自身的 Release。

也可以在 macOS 上运行仓库内的 `./build-macos.sh 0.2.0`。

有 S3 地址后设置 `MODELSWITCHER_UPDATE_BASE_URL`（公开下载地址前缀）和 `MODELSWITCHER_S3_URI`（例如 `s3://bucket/model-switcher`），再执行 `pwsh ./scripts/release.ps1 -Version 0.2.0 -Publish`。脚本使用本机 AWS CLI 配置或环境凭据，不把密钥写入仓库；macOS 端设置 `PUBLISH=1` 后运行 `./scripts/release-macos.sh 0.2.0`。

Windows 发布文件：`build/bin/model-switcher.exe`，是便携版单文件 exe，不需要安装 Go、Node.js 或 Wails。Windows 10/11 通常已包含 WebView2。

macOS 发布文件：`build/bin/ModelSwitcher.app`。API Key 会随程序同级 `profiles.json` 保存，分发或备份该文件时请一并考虑其中的凭据内容。
