# Model Switcher

一个 Windows / macOS 便携版配置工具，用于管理 OpenAI 兼容的本地模型服务，并切换官方 ChatGPT 应用里的 Codex 提供方。

双击 build/bin/model-switcher.exe，添加本地服务，填写 API Base URL（例如 http://127.0.0.1:1234/v1），点击“获取模型”，选择模型后在左侧点击“保存”。已保存的配置可以直接在左侧菜单点击“切换”。

工具会从 /v1/models 读取模型列表，默认在程序同级的 `profiles.json` 读取和写入配置，在 Windows 使用 DPAPI、macOS 使用系统钥匙串安全保存 API Key，在用户目录的 `.codex/config.toml` 中写入本地 provider，切换前备份原始配置，并支持恢复切换前的 model 与 model_provider。

切换本地模型或“切回 OpenAI”时，工具会先关闭再重启官方 ChatGPT 应用。启动工具时会自动查找并显示当前 ChatGPT 路径。Windows 优先查找已安装应用和运行进程，并以开始菜单作为回退；macOS 通过 Spotlight 和常用应用目录查找 `ChatGPT.app`。点击路径可打开系统原生文件选择窗口（Windows 选择 `.exe` / `.lnk`，macOS 选择 `.app`），选择后自动保存，取消不会修改原设置。点击“自动查找”恢复自动模式；自动模式每次重新解析路径以适应应用更新。

界面底部可以打开 `.codex` 目录，或在工具内查看并编辑当前 `config.toml`。保存前会校验 TOML 并备份原文件；程序同级的 `backups` 目录保存配置备份，旧版 AppData 配置会自动迁移到程序目录。

左下角“设置”可以切换中文 / English，以及浅色 / 深色主题；选择会保存在本机。

设置中的“更新管理”默认跳过检查，因为当前没有配置更新源。部署 S3 后可通过环境变量 `MODELSWITCHER_UPDATE_URL` 指向清单，例如：`{"version":"0.2.0","windows":{"url":"https://.../ModelSwitcher.exe","sha256":"..."},"macos":{"url":"https://.../ModelSwitcher.app.zip","sha256":"..."}}`。下载后会校验 SHA-256；清单提供 `signature` 时，还会使用 `MODELSWITCHER_UPDATE_PUBLIC_KEY` 校验 Ed25519 签名。Windows 使用独立更新助手替换并重启 exe，macOS 替换并重新打开整个 app 包。

设置中的“技能管理”会扫描 `CODEX_HOME/skills`（未设置时为 `~/.codex/skills`），展示技能目录、文件数量、大小和是否包含脚本。`.system` 受到保护，删除的用户技能会先移入 `skills/.trash`。官方目录使用 OpenAI Plugins 的官方 marketplace，只允许安装官方仓库中声明为 Codex 可用的项目；安装会先下载到临时目录，校验路径、符号链接、`SKILL.md` 和文件大小，再移动到技能目录，不会自动执行技能脚本。技能内容仍可能包含提示注入或外部命令，安装后应先审阅 `SKILL.md` 和 `scripts`。

本地服务还需要兼容 Codex 使用的 /v1/responses。只兼容 /v1/chat/completions 的服务需要使用协议转换代理。

开发命令：npm --prefix frontend install；wails dev

Windows 发布命令：`pwsh ./scripts/release.ps1 -Version 0.2.0`。脚本会使用 `-X main.AppVersion=...` 注入版本号，生成 `release/0.2.0/ModelSwitcher-0.2.0-windows-amd64.exe` 和 `latest.json`。

macOS 发布命令（需要在 macOS 主机安装 Xcode / WebKit 环境执行）：`./scripts/release-macos.sh 0.2.0`。它会生成 `.app.zip`、SHA-256 和合并后的 `latest.json`。

也可以在 macOS 上运行仓库内的 `./build-macos.sh 0.2.0`。

有 S3 地址后设置 `MODELSWITCHER_UPDATE_BASE_URL`（公开下载地址前缀）和 `MODELSWITCHER_S3_URI`（例如 `s3://bucket/model-switcher`），再执行 `pwsh ./scripts/release.ps1 -Version 0.2.0 -Publish`。脚本使用本机 AWS CLI 配置或环境凭据，不把密钥写入仓库；macOS 端设置 `PUBLISH=1` 后运行 `./scripts/release-macos.sh 0.2.0`。

Windows 发布文件：`build/bin/model-switcher.exe`，是便携版单文件 exe，不需要安装 Go、Node.js 或 Wails。Windows 10/11 通常已包含 WebView2。

macOS 发布文件：`build/bin/model-switcher.app`。macOS 版本使用系统钥匙串保存 API Key；分发给其他用户时建议在 macOS 上签名并公证。

