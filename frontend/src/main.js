import './style.css';
import './app.css';
import { renderResponse as markdownHTML } from './response-format.mjs';
import { createPiChat } from './pi-chat';
import { createEnvironmentManager } from './environment-ui.mjs';
import { serializeProfile, parseProfile } from './profile-transfer.mjs';
import { readClipboardText, writeClipboardText } from './clipboard.mjs';
import { DetectEnvironment, StartDependencyInstall, GetDependencyInstallState } from '../wailsjs/go/main/App';
import { BrowserOpenURL } from '../wailsjs/runtime/runtime';
import { LoadCLIState, LaunchCLI, ChooseCLIDirectory, LoadSkillMarketMetrics, ActivateOpenAI, ActivateProfile, ChooseChatGPTPath, DeleteProfile, FetchModels, TestModel, LoadState, LoadLocalSkillsState, SearchSkills, CleanSkills, RestoreSkill, SetTarget, InstallSkill, InstallSkillTo, DeleteSkill, CheckForUpdate, StartUpdate, OpenCodexDirectory, ReadConfigText, SaveProfile, SetChatGPTPath, WriteConfigText, LoadWorkspaceState, ChooseWorkspaceDirectory, SaveWorkspace, SelectWorkspace, DeleteWorkspace, LoadAgentConfig, ReadAgentDocument, WriteAgentDocument, LoadSharedSkillsState, OpenWorkspaceDirectory, OpenSharedSkillsDirectory, LoadAdvancedState, ReadRuntimeDocument, WriteRuntimeDocument, ReadMemory, WriteMemory, CreateMemory, DeleteMemory, OpenAdvancedDirectory } from '../wailsjs/go/main/App';

const paths = {
 chat: '<path d="M21 11.5a8.4 8.4 0 0 1-9 8.5 9.3 9.3 0 0 1-4-.9L3 21l1.9-5A9.3 9.3 0 0 1 4 12a8.4 8.4 0 0 1 8.5-9H13a8.4 8.4 0 0 1 8 8Z"/>',
 arrowUp: '<path d="M12 19V5m-6 6 6-6 6 6"/>',
 paperclip: '<path d="m21 11-8.5 8.5a6 6 0 0 1-8.5-8.5l9-9a4 4 0 0 1 5.7 5.7l-9 9a2 2 0 0 1-2.9-2.9l8.5-8.5"/>',
 steer: '<path d="M5 4v5a4 4 0 0 0 4 4h11m-5-5 5 5-5 5"/>',
 more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
 stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
 route: '<circle cx="5" cy="7" r="2"/><circle cx="19" cy="5" r="2"/><circle cx="19" cy="19" r="2"/><path d="M7 7h3a4 4 0 0 1 4 4v4a4 4 0 0 0 3 4M10 7h1a4 4 0 0 0 4-2h2"/>',
 terminal: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7 9 3 3-3 3M13 15h4"/>',
 star: '<path d="m12 3 2.8 5.7 6.3.9-4.6 4.4 1.1 6.3L12 17.3l-5.6 3 1.1-6.3L3 9.6l6.2-.9Z"/>',
 chevronDown: '<path d="m6 9 6 6 6-6"/>',
 chevronRight: '<path d="m9 6 6 6-6 6"/>',
 minus: '<path d="M5 12h14"/>',
 spark: '<path d="m12 3-1.3 5.7L5 10l5.7 1.3L12 17l1.3-5.7L19 10l-5.7-1.3L12 3Z"/><path d="m5 16-.5 2.5L2 19l2.5.5L5 22l.5-2.5L8 19l-2.5-.5L5 16Z"/>',
 server: '<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01M11 7h8M11 17h8"/>',
 refresh: '<path d="M20 11A8 8 0 0 0 6 6L3 9M3 3v6h6M4 13a8 8 0 0 0 14 5l3-3M21 21v-6h-6"/>',
 check: '<path d="m5 12 4 4L19 6"/>',
 cloud: '<path d="M17.5 19H9a6 6 0 1 1 5.7-8H16a4 4 0 1 1 1.5 8Z"/>',
 play: '<path d="m8 5 11 7-11 7V5Z"/>',
 folder: '<path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v9H3V7Z"/>',
 plus: '<path d="M12 5v14M5 12h14"/>',
 trash: '<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>',
 briefcase: '<rect x="3" y="7" width="18" height="13" rx="2"/><path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M3 12h18M10 12v2h4v-2"/>',
 file: '<path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M9 13h6M9 17h6"/>',
 users: '<path d="M16 20v-1.5a3.5 3.5 0 0 0-3.5-3.5h-3A3.5 3.5 0 0 0 6 18.5V20M11 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM18 9a2.5 2.5 0 0 0-1.5-2.3M18 14h1a2 2 0 0 1 2 2v1"/>',
 brain: '<path d="M9.5 4.5a3 3 0 0 0-5 2.2A3.2 3.2 0 0 0 5 13a3.2 3.2 0 0 0-.5 6.3A3 3 0 0 0 9.5 21M9.5 4.5a3 3 0 0 1 5 2.2A3.2 3.2 0 0 1 14 13a3.2 3.2 0 0 1 .5 6.3A3 3 0 0 1 9.5 21M9.5 4.5v17M14.5 4.5v17M5 9h4M15 9h4M5 16h4M15 16h4"/>',
 plug: '<path d="M9 7V3M15 7V3M6 7h12v3a6 6 0 0 1-12 0V7ZM12 16v5M8 21h8"/>',
 sliders: '<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="9" cy="6" r="2"/><circle cx="15" cy="12" r="2"/><circle cx="11" cy="18" r="2"/>',
 settings: '<path d="M12 15.2a3.2 3.2 0 1 0 0-6.4 3.2 3.2 0 0 0 0 6.4Z"/><path d="m19.4 15 .1.1a1.8 1.8 0 1 1-2.5 2.5l-.1-.1a1.8 1.8 0 0 0-3.1 1.3v.2a1.8 1.8 0 1 1-3.6 0v-.2a1.8 1.8 0 0 0-3.1-1.3l-.1.1a1.8 1.8 0 1 1-2.5-2.5l.1-.1a1.8 1.8 0 0 0-1.3-3.1h-.2a1.8 1.8 0 1 1 0-3.6h.2a1.8 1.8 0 0 0 1.3-3.1l-.1-.1a1.8 1.8 0 1 1 2.5-2.5l.1.1a1.8 1.8 0 0 0 3.1-1.3v-.2a1.8 1.8 0 1 1 3.6 0v.2a1.8 1.8 0 0 0 3.1 1.3l.1-.1a1.8 1.8 0 1 1 2.5 2.5l-.1.1a1.8 1.8 0 0 0 1.3 3.1h.2a1.8 1.8 0 1 1 0 3.6h-.2a1.8 1.8 0 0 0-1.3 3.1Z"/>',
 close: '<path d="m6 6 12 12M18 6 6 18"/>',
 save: '<path d="M5 4h11l3 3v13H5V4Z"/><path d="M8 4v6h8V4M8 20v-6h8v6"/>',
 search: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/>',
 edit: '<path d="m4 16-.8 4.8L8 20l10.8-10.8a2.8 2.8 0 0 0-4-4L4 16Z"/><path d="m13.5 6.5 4 4"/>',
 download: '<path d="M12 3v12M7 10l5 5 5-5"/><path d="M5 21h14"/>',
 shield: '<path d="M12 3 20 6v5c0 5-3.4 8.5-8 10-4.6-1.5-8-5-8-10V6l8-3Z"/><path d="m9 12 2 2 4-4"/>',
 eye: '<path d="M2.5 12s3.5-5 9.5-5 9.5 5 9.5 5-3.5 5-9.5 5-9.5-5-9.5-5Z"/><circle cx="12" cy="12" r="2.5"/>',
 eyeOff: '<path d="m3 3 18 18M10.6 6.2A10.8 10.8 0 0 1 12 6c6 0 9.5 6 9.5 6a17 17 0 0 1-3.4 3.8M6.2 6.8C3.9 8.1 2.5 12 2.5 12s3.5 6 9.5 6c1.1 0 2.1-.2 3-.6"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
 copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
 paste: '<rect x="8" y="2" width="8" height="4" rx="1"/><path d="M8 4H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h9M16 4h3a2 2 0 0 1 2 2v5M12 15h9m-3-3 3 3-3 3"/>',
};
const icon = (name, size = 18) => '<svg width="' + size + '" height="' + size + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + (paths[name] || '') + '</svg>';
const esc = (value = '') => String(value).replace(/[&<>'"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#039;', '"': '&quot;' }[c]));
const DEFAULT_CONTEXT_WINDOW = 256 * 1024;
const contextWindowValue = (value) => Number.isFinite(Number(value)) && Number(value) > 0 ? Math.round(Number(value)) : DEFAULT_CONTEXT_WINDOW;
const formatContextWindow = (value) => {
 const tokens = contextWindowValue(value);
 return tokens >= 1024 * 1024 ? `${(tokens / (1024 * 1024)).toFixed(tokens % (1024 * 1024) ? 1 : 0)}M` : `${Math.round(tokens / 1024)}K`;
};
const normalizeDraftModels = (models = []) => models.map((model) => ({ ...model, contextWindow: contextWindowValue(model.contextWindow) }));
const emptyDraft = () => ({ id: '', name: '', baseUrl: '', apiKey: '', selectedModel: '', models: [], clearApiKey: false, hasApiKey: false });
let state = { version: '0.2.24', profiles: [], activeProvider: 'openai', activeModel: '', activeProfileId: '', configPath: '', profilePath: '', loadError: '', canRestore: false, chatGptRunning: false, chatGptTarget: '' };
let draft = emptyDraft();
let busy = '';
let restoreFocusID = '';
let error = '';
let message = '';
let modelTestInput = '';
let modelTestReply = '';
let modelTestModel = '';
let modelTestProtocol = '';
let dirty = false;
let loaded = false;
let configPreview = null;
let configDirty = false;
let settingsOpen = false;
let skillsState = { root: '', skills: [], trash: [], catalog: [], markets: [], errors: [] };
let skillQuery = '';
let skillMarket = 'all';
let selectedSkills = new Set();
let showTrash = false;
let page = 'models';
let workspaceState = { projects: [], activeId: '' };
let agentConfig = { workspace: {}, documents: [], codexHome: '', piHome: '', sharedSkillsRoot: '', sharedSkills: [] };
let workspaceForm = { id: '', name: '', path: '' };
let agentPreview = null;
let agentDirty = false;
let advancedState = { target: 'chatgpt', codexHome: '', piHome: '', memoryVault: '', currentProjectId: '', codexMemoryRoot: '', documents: [], memories: [], codexMemories: [], mcpServers: [], diagnostics: [] };
let advancedSection = 'memory';
let advancedPreview = null;
let advancedDirty = false;
let keyVisible = false;
let skillSection = 'installed';
let treeExpanded = { models: true, skills: true, cli: true, advanced: true };
try { treeExpanded = { ...treeExpanded, ...JSON.parse(localStorage.getItem('model-switcher-navigation') || '{}') }; } catch {}
let sidebarWidth = Number(localStorage.getItem('model-switcher-sidebar-width') || 256);
let resetPageScroll = false;
const isPi = () => state.target === 'pi';
const configName = () => isPi() ? 'models.json' : 'config.toml';
let updateState = { status: 'unconfigured', message: '更新源未配置，已跳过检查。', currentVersion: '0.2.24', updateAvailable: false };
let language = localStorage.getItem('model-switcher-language') || 'zh';
let theme = localStorage.getItem('model-switcher-theme') || 'light';
const app = document.querySelector('#app');
const piChat = createPiChat({ redraw: render, getState: () => state, getLanguage: () => language, icon, esc, markdown: markdownHTML });
const environment = createEnvironmentManager({ detect: DetectEnvironment, startInstall: StartDependencyInstall, getInstallState: GetDependencyInstallState, redraw: render, getLanguage: () => language, icon, esc, onInstalled: async () => { if (page === 'cli') { await refreshCLI(false); render(); } } });

const zhToEn = {
 '复制配置': 'Copy profile', '粘贴配置': 'Paste profile', '正在复制…': 'Copying…', '正在粘贴…': 'Pasting…',
 '复制连接信息、API Key 和模型设置': 'Copy the connection, API key, and model settings', '从剪贴板新建配置': 'Create a profile from the clipboard',
 '配置已复制到剪贴板。': 'Profile copied to the clipboard.', '已粘贴为新配置，保存后即可使用。': 'Pasted as a new profile. Save it to use it.',
 '无法写入剪贴板，请重试。': 'Could not write to the clipboard. Please retry.', '无法读取剪贴板，请允许剪贴板访问后重试。': 'Could not read the clipboard. Allow clipboard access and retry.',
 '剪贴板为空，请先复制一个配置': 'The clipboard is empty. Copy a profile first.', '配置内容必须是文本': 'Profile content must be text.',
 '配置不是有效的 JSON，请检查复制内容': 'The profile is not valid JSON. Check the copied content.', '请粘贴单个配置，不支持配置列表或 profiles.json': 'Paste a single profile, rather than a profile list or profiles.json.',
 '配置必须是 JSON 对象': 'The profile must be a JSON object.', 'JSON 中没有模型配置字段，请复制一个模型配置': 'No model profile fields were found. Copy a model profile first.', 'models 必须是模型数组': 'models must be an array of models.',
 '对话': 'Conversation', '与当前 pi 模型进行多轮对话，实时查看回复。': 'Chat with the current pi model and watch replies as they arrive.',
 'pi agent 使用': 'pi agent uses', '模型与代理工作台': 'Models & agents workspace', '配置位置': 'Configuration location', '模型连接': 'Model connection',
 'CLI 启动器': 'CLI launcher', '展开或折叠 CLI 启动器': 'Expand or collapse CLI launcher', '选择工作目录，启动对应的命令行代理。': 'Choose a working folder and launch a command-line agent.', '启动 CLI': 'Launch CLI', '在系统终端中打开 Codex 或 pi agent。': 'Open Codex or pi agent in your system terminal.', '重新检测': 'Detect again', '工作目录': 'Working folder', '选择项目所在文件夹': 'Choose your project folder', 'CLI 会在这个目录中启动。': 'The CLI starts in this folder.', '选择目录': 'Choose folder', '已检测到': 'Detected', '未安装或未找到': 'Not installed or not found', '正在检测 CLI…': 'Detecting CLIs…',
 '安装量': 'Installs', '仓库 Stars': 'Repository stars', '用户评分': 'User rating', '查看反馈': 'View feedback', '统计来源': 'Stats source', '获取中…': 'Loading…', '排序': 'Sort', '安装量优先': 'Most installed', '仓库热度优先': 'Most repository stars', '名称 A–Z': 'Name A–Z', '按市场': 'Market', '用户评分（未提供）': 'User rating (unavailable)', '正在加载市场': 'Loading markets', '市场加载完成': 'Markets loaded', '上一页': 'Previous', '下一页': 'Next', '技能分页': 'Skill pages', '描述': 'Description',
 'skills.sh 记录的安装次数，非全网下载数': 'Install counts recorded by skills.sh, not global downloads', '整个仓库的 Stars，不是单个技能的评分': 'Stars for the entire repository, not a skill rating', '安装量来自 skills.sh；仓库 Stars 仅代表仓库热度。当前市场未提供用户评分。': 'Installs come from skills.sh. Repository stars reflect repository popularity. These markets do not provide user ratings.',
 '选择适合当前环境的显示模式。': 'Choose the appearance for your environment.',
 '使用的模型': 'Active model', '上下文大小': 'Context window', '默认 256K；应用后由当前模型直接使用。': 'Default 256K; applied directly to the selected model.', '尚未选择模型': 'No model selected', '本地模型': 'Local model', '正在读取…': 'Loading…', '关闭配置编辑': 'Close config editor',
 '主导航': 'Main navigation', '模型设置': 'Model settings', '当前应用': 'Current app', '连接与模型': 'Connections and models',
 '连接模型服务，选择模型并应用配置。': 'Connect a service, choose a model, and apply your configuration.', '搜索、安装和整理当前应用的技能。': 'Find, install, and organize skills for the current app.',
 '当前使用': 'Currently using', '使用默认模型': 'Default model', '编辑连接信息并应用到当前应用。': 'Edit this connection and apply it to the current app.', '添加一个 OpenAI 兼容的模型服务。': 'Add an OpenAI-compatible model service.',
 '应用到 pi agent': 'Apply to pi agent', '应用到 ChatGPT': 'Apply to ChatGPT', '图像': 'Image', '菜单栏宽度': 'Sidebar width', '拖动分隔条或用方向键调整。': 'Drag the divider or use the arrow keys.',
 '缩窄菜单栏': 'Narrow sidebar', '加宽菜单栏': 'Widen sidebar', '调整菜单栏宽度': 'Resize sidebar', '拖动调整宽度；方向键微调，双击恢复默认': 'Drag to resize; use arrow keys to fine-tune, double-click to reset',
 '展开或折叠模型设置': 'Expand or collapse model settings', '展开或折叠技能管理': 'Expand or collapse skills', '打开设置': 'Open settings', '刷新当前状态': 'Refresh current state',
 '检查新版本并在下载校验后自动重启。': 'Check for updates and restart after verification.', '从支持的市场安装技能。': 'Install skills from supported markets.', '管理当前应用的本地技能。': 'Manage local skills for the current app.',
 '应用': 'App', '重新应用': 'Reapply', '支持图像输入': 'Supports image input', '仅在模型和 API 支持图像时启用。': 'Enable when both the model and API support images.', 'pi agent 配置': 'pi agent config', 'pi 默认模型': 'pi default model', '恢复原配置': 'Restore previous config',
 '技能市场': 'Skill market', '全部市场': 'All markets', '搜索技能': 'Search skills', '名称、描述或作者': 'Name, description, or author', '搜索': 'Search',
 '市场技能': 'Market skills', '元数据': 'Metadata', '来源': 'Source', '作者': 'Author', '许可': 'License', '版本': 'Version', '修改时间': 'Modified', '查看来源': 'View source',
 '未提供': 'Not provided', '本地技能': 'Local skill', '回收区': 'Trash', '恢复': 'Restore', '清理所选': 'Clean selected', '回收区为空': 'Trash is empty',
 '包含脚本': 'Contains scripts', '同名已存在': 'Name occupied', '正在处理…': 'Working…', '正在读取技能市场…': 'Loading skill market…',
 '没有匹配的本地技能，试试其他关键词。': 'No local skills match. Try another keyword.', '没有匹配的技能，试试其他关键词或市场。': 'No skills match. Try another keyword or market.',
 '市场加载失败，请刷新重试。': 'Market could not be loaded. Refresh to retry.', '打开 pi 配置目录': 'Open pi config folder', '查看 models.json': 'View models.json', '编辑 models.json': 'Edit models.json', '保存 models.json': 'Save models.json',
 '切换后在 pi 中使用 /model 选择模型；新会话使用已保存的默认模型。技能安装后使用 /reload。': 'Use /model in pi to select the model. New sessions use the saved default. Use /reload after installing skills.',
 'pi 默认模型已保存；在 pi 中使用 /model 选择模型或开启新会话。': 'Pi default model saved. Select it with /model or start a new session.',
 '已恢复 pi 切换前的配置。': 'Previous pi configuration restored.', '技能已安装。': 'Skill installed.', '技能已移入回收区，可恢复。': 'Skill moved to trash. You can restore it.', '所选技能已移入回收区，可恢复。': 'Selected skills moved to trash. You can restore them.', '技能已恢复。': 'Skill restored.',
 '保存前会校验 JSON/JSONC，并自动备份原文件。': 'JSON/JSONC is validated and the original file is backed up before saving.',

 '让模型切换更简单': 'Switch models simply', '工作空间': 'Workspace', '添加本地配置': 'Add local profile',
 '只属于你的配置': 'Your profiles', 'API Key 使用系统安全存储。': 'API keys use secure system storage.',
 'Codex 配置': 'Codex config', 'OpenAI 云端': 'OpenAI Cloud', '模型切换': 'Model Switcher', '模型配置': 'Model Profiles',
 '一个地方，管理你的本地模型连接。': 'Manage local model connections in one place.', 'ChatGPT 运行中': 'ChatGPT running',
 'ChatGPT 未运行': 'ChatGPT not running', '服务连接': 'Service connection', '编辑配置': 'Edit profile', '新建配置': 'New profile',
 '配置名称': 'Profile name', 'API 地址': 'API address', '支持本机、局域网和远程 OpenAI 兼容服务。': 'Local, LAN, and remote OpenAI-compatible services are supported.', '测试模型': 'Test model', '测试对话': 'Test conversation', '发送给当前模型': 'Send to current model', '发送测试': 'Send test', '正在发送…': 'Sending…', '模型回答': 'Model reply', '当前模型：': 'Current model: ', '输入一段话并点击“发送测试”，模型的完整回答会显示在这里。': 'Enter a message and click “Send test” to see the complete reply here.', '输入要发送给模型的内容，例如：你好，请介绍一下你自己。': 'Enter a message, for example: Hello, please introduce yourself.', '正在测试…': 'Testing…',
 '可选': 'optional', 'Key 已加密保存；留空保持原值': 'Key is encrypted; leave blank to keep it', '服务无需认证时可留空': 'Leave blank if the service needs no authentication',
 '移除已保存的 Key': 'Remove saved key', '保存': 'Save', '正在保存…': 'Saving…', '获取模型': 'Fetch models', '正在获取…': 'Fetching…',
 '选择模型': 'Choose model', '选择一个模型': 'Choose a model', '等待获取模型列表': 'Waiting for model list', '先连接你的本地服务': 'Connect your local service first',
 '填写 API 地址，然后点击「获取模型」。': 'Enter an API address, then click “Fetch models”.', '接入 Codex 需要兼容': 'Codex requires a compatible',
 '有未保存的修改。': 'You have unsaved changes.', '配置保存在此电脑，切换前自动备份 Codex 配置。': 'Profiles are saved on this computer; Codex config is backed up before switching.',
 'ChatGPT 路径': 'ChatGPT path', '手动选择': 'Manual', '自动查找': 'Auto-detect', '未找到 ChatGPT，点击选择应用': 'ChatGPT not found; click to choose an app',
 '正在查找 ChatGPT…': 'Finding ChatGPT…', '打开 .codex 目录': 'Open .codex folder', '查看 config.toml': 'View config.toml', '删除配置': 'Delete profile',
 '切回 OpenAI': 'Switch to OpenAI', '切换': 'Switch', '当前': 'Current', '已应用': 'Applied', '配置已保存。': 'Profile saved.', '设置': 'Settings', '关闭设置': 'Close settings',
 '语言': 'Language', '主题': 'Theme', '浅色': 'Light', '深色': 'Dark', '界面语言和外观会保存在本机。': 'Language and appearance are saved on this computer.',
 '未保存': 'Unsaved', '代码编辑器': 'Code editor', 'CODEX CONFIG': 'CODEX CONFIG', '编辑 config.toml': 'Edit config.toml',
 '保存 config.toml': 'Save config.toml', '保存前会校验 TOML，并自动备份原文件。': 'TOML is validated and the original file is backed up before saving.',
 '技能管理': 'Skills', '管理本机 .codex/skills 中的技能，也可从官方目录安装。': 'Manage skills in local .codex/skills or install from the official catalog.',
 '已安装': 'Installed', '官方目录': 'Official catalog', '刷新技能': 'Refresh skills', '目录': 'Folder', '系统技能': 'System skill',
 '包含脚本，请先审阅': 'Contains scripts; review before use', '受保护': 'Protected', '删除': 'Delete', '安装': 'Install',
 '官方 OpenAI 技能目录': 'Official OpenAI skill catalog', '无法加载官方目录': 'Official catalog unavailable', '暂无技能': 'No skills found',
 '更新管理': 'Updates', '检查更新': 'Check for updates', '更新源未配置，已跳过检查。': 'No update source configured; skipped.', '当前已经是最新版本。': 'You are up to date.',
 '工作区': 'Workspaces', '选择项目工作区': 'Choose a project workspace', '项目文件夹': 'Project folder', '添加工作区': 'Add workspace', '保存工作区': 'Save workspace', '选择文件夹': 'Choose folder', '当前工作区': 'Active workspace', '没有工作区': 'No workspace selected', '项目指令': 'Project instructions', '全局指令': 'Global instructions', '共享技能': 'Shared skills', '共用的代理上下文': 'Shared agent context', '请选择一个工作区开始管理指令和共享技能。': 'Choose a workspace to manage instructions and shared skills.', '没有发现指令文件。': 'No instruction files found.', '新建文件': 'Create file', '读取': 'Read', '编辑': 'Edit', '已生效': 'Active', '被覆盖': 'Shadowed', '不存在': 'Missing', '保存指令': 'Save instructions', '关闭': 'Close', '文件内容': 'File content', '选择文件': 'Select a file', 'Codex 全局目录': 'Codex home', 'pi 全局目录': 'pi home', '共享技能目录': 'Shared skills folder', '共享技能会从 ~/.agents/skills 被两个代理发现。': 'Both agents discover shared skills from ~/.agents/skills.', '没有共享技能。': 'No shared skills.', '删除工作区': 'Remove workspace', '工作区名称': 'Workspace name', '代理指令文件': 'Agent instruction files', '作用链': 'Instruction chain', '工作区已保存。': 'Workspace saved.', '工作区已删除。': 'Workspace removed.', '指令文件已保存。': 'Instruction file saved.', '请先选择项目工作区': 'Choose a workspace first.',
 '高级设置': 'Advanced settings', '记忆管理': 'Memory', 'MCP 配置': 'MCP configuration', '运行参数': 'Runtime settings', '管理代理记忆、MCP 服务器和高级运行参数。': 'Manage agent memories, MCP servers, and advanced runtime settings.', '没有记忆文件。': 'No memory files.', '新建记忆': 'New memory', '记忆根目录': 'Memory folder', '全局记忆': 'Global memory', '项目记忆': 'Project memory', '编辑记忆': 'Edit memory', '保存记忆': 'Save memory', '记忆名称': 'Memory name', '记忆内容': 'Memory content', '删除记忆': 'Delete memory', '记忆已保存。': 'Memory saved.', '记忆已删除，可从备份恢复。': 'Memory deleted; it can be restored from backup.', 'MCP 服务器': 'MCP servers', '没有检测到 MCP 服务器。': 'No MCP servers detected.', '配置文件': 'Configuration files', '编辑配置': 'Edit configuration', '打开目录': 'Open folder', '未创建': 'Not created', '已创建': 'Created', '已启用': 'Enabled', '已禁用': 'Disabled', '命令': 'Command', '端点': 'Endpoint', '来源文件': 'Source file', '当前应用支持的设置': 'Settings supported by the current app', 'pi settings 支持启动模型、思考级别、压缩、重试、图像、代理和终端显示设置。': 'Pi settings cover the startup model, thinking, compaction, retry, images, proxy, and terminal display.', 'Codex 高级参数保存在 config.toml 中，可直接编辑并自动备份。': 'Codex advanced parameters live in config.toml and are backed up before editing.', '格式校验失败，未写入。': 'Format validation failed; nothing was written.', '诊断': 'Diagnostics', '没有诊断信息。': 'No diagnostics.', '新建代理配置': 'Create agent config',
 '更新包已校验，程序即将重启。': 'Update verified; the app will restart.', '当前版本': 'Current version', '发现新版本': 'New version available', '更新': 'Update',
 '共享记忆库': 'Shared memory vault', 'Codex 原生记忆': 'Codex native memory', '只读': 'Read-only', '所有项目共用': 'Shared by every project', '仅在该项目使用': 'This project only', '其他项目记忆': 'Other projects', '当前项目记忆': 'Current project', '共享记忆怎么用': 'How the shared vault works',
 'Codex 和 pi agent 共用这一个记忆库，新建和编辑都写在这里，索引会自动更新。': 'Codex and pi agent share this one vault. New notes are written here and the index refreshes automatically.',
 'Codex 自己写入的记忆，本应用只读。打开一条可以复制内容，再存成共享记忆。': 'Memories Codex writes itself. This app reads them only; open one, copy it, then save it as a shared memory.',
 '还没有共享记忆，点击「新建记忆」写下第一条。': 'No shared memories yet. Click New memory to write the first one.',
 'Codex 还没有生成记忆。': 'Codex has not created any memories yet.',
 'Codex 记忆（只读）': 'Codex memory (read-only)', '提炼到共享记忆': 'Copy into shared memory', '保存位置': 'Stored in', '全局记忆（所有项目共用）': 'Global memory (shared by every project)',
 '这是代理自己生成的记忆，本应用只读；可复制内容后新建共享记忆。': 'The agent generated this memory, so it is read-only here. Copy the content into a new shared memory instead.',
 '全局记忆所有项目共用，项目记忆按项目分目录保存，两个代理都会读到同一份内容。': 'Global notes apply to every project; project notes live in a folder per project. Both agents read the same files.',
 'pi CLI 会在每次提问前自动加载全局记忆和当前项目记忆，soul.md 可用于设定名字和身份。': 'Pi CLI loads global and current project memories before each prompt. Use soul.md to define its name and identity.',
 '已打开的 pi 会话请先执行 /reload，之后记忆修改会在下次提问时生效。': 'Run /reload once in an existing Pi session. Later memory edits take effect on the next prompt.',
 'Codex 需要在全局指令中指向共享记忆索引。': 'Point Codex global instructions at the shared memory index.',
 '例如：project-context.md': 'For example: project-context.md', '配置内容': 'Configuration content',
};
function translateText(text) {
 const trimmed = text.trim();
 if (trimmed.startsWith('安装量 ')) return text.replace(trimmed, 'Installs ' + translateText(trimmed.slice(4)));
 if (trimmed.startsWith('用户评分 ')) return text.replace(trimmed, 'User rating ' + translateText(trimmed.slice(5)));
 if (zhToEn[trimmed]) return text.replace(trimmed, zhToEn[trimmed]);
 if (trimmed.endsWith(' · 从支持的市场安装技能。')) return trimmed.replace(' · 从支持的市场安装技能。', ' · Install skills from supported markets.');
 if (trimmed.endsWith(' · 管理当前应用的本地技能。')) return trimmed.replace(' · 管理当前应用的本地技能。', ' · Manage local skills for the current app.');
 if (trimmed.endsWith(' · 搜索市场技能，安装或移入回收区。')) return trimmed.replace(' · 搜索市场技能，安装或移入回收区。', ' · Search markets, install skills, or move them to trash.');
 if (/^已安装 \(\d+\)$/.test(trimmed)) return trimmed.replace('已安装', 'Installed');
 if (/^回收区 \(\d+\)$/.test(trimmed)) return trimmed.replace('回收区', 'Trash');
 const count = trimmed.match(/^(\d+) 个可用$/);
 if (count) return text.replace(trimmed, count[1] + ' available');
 const found = trimmed.match(/^发现 (\d+) 个模型，已自动填入。$/);
 if (found) return text.replace(trimmed, found[1] + ' models found and filled in.');
 const switched = trimmed.match(/^已切换到「(.+)」，ChatGPT 正在重启。$/);
 if (switched) return text.replace(trimmed, 'Switched to “' + switched[1] + '”; ChatGPT is restarting.');
 const target = trimmed.match(/^切换到 (.+)$/);
 if (target) return text.replace(trimmed, 'Switch to ' + target[1]);
 const modelObject = trimmed.match(/^第 (\d+) 个模型必须是对象$/);
 if (modelObject) return text.replace(trimmed, 'Model ' + modelObject[1] + ' must be an object.');
 const modelID = trimmed.match(/^第 (\d+) 个模型必须包含非空字符串 id$/);
 if (modelID) return text.replace(trimmed, 'Model ' + modelID[1] + ' must include a non-empty string id.');
 const fieldType = trimmed.match(/^(.+) 必须是(字符串|布尔值)$/);
 if (fieldType) return text.replace(trimmed, fieldType[1] + ' must be ' + (fieldType[2] === '字符串' ? 'a string.' : 'a boolean.'));
 const contextSize = trimmed.match(/^(.+) 必须是正整数，未设置时可省略或填写 0$/);
 if (contextSize) return text.replace(trimmed, contextSize[1] + ' must be a positive integer. Omit it or use 0 for the default.');
 return text;
}
function localizeDOM() {
 if (language !== 'en') return;
 const walk = (node) => {
  if (node.nodeType === Node.ELEMENT_NODE && (node.closest('.skill-description, .skill-details dd, .skill-item-title strong, [data-no-translate]') || node.tagName === 'TEXTAREA')) return;
  if (node.nodeType === Node.TEXT_NODE) node.nodeValue = translateText(node.nodeValue);
  else node.childNodes.forEach(walk);
 };
 walk(app);
 app.querySelectorAll('[placeholder],[title],[aria-label]').forEach((el) => {
  ['placeholder', 'title', 'aria-label'].forEach((attr) => { if (el.hasAttribute(attr)) el.setAttribute(attr, translateText(el.getAttribute(attr))); });
 });
}
function applyPreferences() {
 document.documentElement.classList.toggle('theme-dark', theme === 'dark');
 document.body.classList.toggle('theme-dark', theme === 'dark');
}

function profileFor(id) { return state.profiles.find((p) => p.id === id); }
function currentProfile() { return profileFor(draft.id); }
function edit(profile) { draft = profile ? { ...profile, apiKey: profile.apiKey || '', clearApiKey: false, models: normalizeDraftModels(profile.models || []) } : emptyDraft(); keyVisible = false; dirty = false; error = ''; message = ''; modelTestReply = ''; modelTestModel = ''; modelTestProtocol = ''; render(); }
function currentIsLocal() { return state.activeProvider === 'model_switcher_local'; }
function formatBytes(bytes) {
 if (!bytes) return '0 B';
 const units = ['B', 'KB', 'MB', 'GB']; let value = bytes; let index = 0;
 while (value >= 1024 && index < units.length - 1) { value /= 1024; index += 1; }
 return value.toFixed(value >= 10 || index === 0 ? 0 : 1) + ' ' + units[index];
}
function skillMetadataHTML(item, local = false) {
 const values = [['来源', item.repository || item.source || '本地技能'], ['作者', item.author || '未提供'], ['许可', item.license || '未提供'], ['版本', item.version ? item.version.slice(0, 12) : '未提供']];
 if (!local && item.description?.length > 180) values.push(['描述', item.description]);
 if (local) values.push(['目录', item.path], ['修改时间', item.modifiedAt ? new Date(item.modifiedAt).toLocaleString(language === 'en' ? 'en-US' : 'zh-CN') : '未提供']);
 return '<details class="skill-details" id="metadata-' + esc(item.id || item.name) + '"><summary>元数据</summary><dl>' + values.map(([label, value]) => '<dt>' + label + '</dt><dd>' + esc(value) + '</dd>').join('') + '</dl>' + (item.url?.startsWith('https://github.com/') ? '<button class="text-button" data-skill-url="' + esc(item.url) + '">查看来源</button>' : '') + '</details>';
}
function renderSkillRows() {
 if(skillSection === 'market')return renderMarketRows();
 const disabled = busy ? 'disabled' : '';
 const query = skillSection === 'market' ? skillQuery.trim().toLowerCase() : '';
 const matches = (skill) => [skill.name, skill.title, skill.description, skill.repository].some((v) => (v || '').toLowerCase().includes(query));
 const localItems = (showTrash ? skillsState.trash : skillsState.skills).filter(matches);
 const local = localItems.length ? localItems.map((skill) => '<article class="skill-item"><div class="skill-item-main"><div class="skill-item-title">' + (!showTrash && !skill.system ? '<input type="checkbox" aria-label="选择 ' + esc(skill.name) + '" data-select-skill="' + esc(skill.name) + '" ' + (selectedSkills.has(skill.name) ? 'checked' : '') + ' ' + disabled + '>' : '') + '<strong>' + esc(skill.title || skill.name) + '</strong>' + (skill.system ? '<span class="skill-badge protected">受保护</span>' : '') + (skill.hasScripts ? '<span class="skill-badge warning">包含脚本</span>' : '') + '</div><p class="skill-description">' + esc(skill.description || '本地技能') + '</p><code>' + formatBytes(skill.sizeBytes) + ' · ' + skill.fileCount + ' ' + (language === 'en' ? 'files' : '个文件') + '</code>' + skillMetadataHTML(skill, true) + '</div>' + (showTrash ? '<button class="button button-secondary skill-install" data-restore-skill="' + esc(skill.trashId) + '" ' + disabled + '>恢复</button>' : '<button class="icon-button skill-delete" data-delete-skill="' + esc(skill.name) + '" aria-label="移入回收区 ' + esc(skill.name) + '" ' + (skill.system || busy ? 'disabled' : '') + '>' + icon('trash', 15) + '</button>') + '</article>').join('') : '<div class="skill-empty">' + (showTrash ? '回收区为空' : query ? '没有匹配的本地技能，试试其他关键词。' : '暂无技能') + '</div>';
 return '<div class="skill-clean-bar"><span>' + (showTrash ? (language === 'en' ? 'Recoverable skills: ' : '可恢复技能：') + localItems.length : (language === 'en' ? 'Selected: ' : '已选择：') + selectedSkills.size) + '</span>' + (!showTrash ? '<button id="skills-clean" class="button button-secondary delete-button" ' + (busy || !selectedSkills.size ? 'disabled' : '') + '>' + icon('trash', 15) + '清理所选</button>' : '') + '</div><div class="skills-grid local-skills" aria-busy="' + Boolean(busy) + '">' + local + '</div>';
}
const marketCatalogs = new Map();
const marketMetrics = new Map();
let marketGeneration = 0;
let marketProgress = [];
let skillSort = localStorage.getItem('model-switcher-skill-sort') || 'installs';
if(!['installs','stars','name','source'].includes(skillSort))skillSort='installs';
let skillInstallTarget = 'active';
let skillPage = 1;
const skillPageSize = 36;
let cliState = { tools: [], directory: '' };
let cliDirectory = localStorage.getItem('model-switcher-cli-directory') || '';
let selectedCLI = '';
const defaultMarkets = [{id:'openai',name:'OpenAI Skills'}, {id:'anthropic',name:'Anthropic Skills'}, {id:'vercel',name:'Vercel Skills'}, {id:'openai-plugins',name:'OpenAI Plugins'}, {id:'pi-skills',name:'pi-skills'}];
const currentMarkets = () => {
 const markets = skillsState.markets.length ? skillsState.markets : defaultMarkets;
 if (skillMarket !== 'all' && !markets.some((market) => market.id === skillMarket)) skillMarket = markets[0]?.id || 'all';
 return markets;
};
const selectedMarkets = () => currentMarkets().filter((m) => skillMarket === 'all' || m.id === skillMarket);
const marketPending = () => marketProgress.some((m) => m.status === 'loading');
const numberText = (n) => Number(n).toLocaleString(language === 'en' ? 'en-US' : 'zh-CN');
function catalogItems() {
 const items = skillsState.catalog.filter((item) => [item.name,item.directory,item.description,item.author,item.source,item.category,item.repository].some((v) => (v || '').toLowerCase().includes(skillQuery.trim().toLowerCase()))).map((item) => {
  const metric = marketMetrics.get(item.market);
  const count = metric?.skills?.find((s) => s.name === item.name || s.name === item.directory);
  const local = skillsState.skills.find((s) => s.name === item.directory);
  return { ...item, installed: local ? local.sourceId === item.id : false, conflict: local ? local.sourceId !== item.id : false, installs: count?.installs ?? null, statsURL: count?.url || '', stars: metric?.repositoryStars ?? null, metricsPending: metric?.loading };
 });
 const numeric = (a,b,key) => { if(a[key] == null && b[key] == null)return 0; if(a[key] == null)return 1; if(b[key] == null)return -1; return b[key]-a[key]; };
 return items.sort((a,b) => (skillSort === 'installs' ? numeric(a,b,'installs') : skillSort === 'stars' ? numeric(a,b,'stars') : skillSort === 'source' ? a.source.localeCompare(b.source) : 0) || a.name.localeCompare(b.name) || a.id.localeCompare(b.id));
}
function renderMarketRows() {
 const disabled = busy ? 'disabled' : '';
 const items = catalogItems();
 const pages = Math.max(1,Math.ceil(items.length / skillPageSize)); skillPage = Math.min(skillPage,pages);
 const cards = items.slice((skillPage-1)*skillPageSize,skillPage*skillPageSize).map((item) => `<article class="skill-item catalog-item"><div class="skill-item-main"><div class="skill-item-title"><strong>${esc(item.name)}</strong><span class="skill-category">${esc(item.source)}</span>${item.hasScripts ? '<span class="skill-badge warning">包含脚本</span>' : ''}</div><p class="skill-description">${esc(item.description)}</p><div class="skill-stats"><span title="skills.sh 记录的安装次数，非全网下载数">${icon('download',14)}<span class="metric-copy"><span>安装量</span><strong>${item.installs == null ? item.metricsPending ? '获取中…' : '未提供' : numberText(item.installs)}</strong></span></span><span title="整个仓库的 Stars，不是单个技能的评分">${icon('star',14)}<span class="metric-copy"><span>仓库 Stars</span><strong>${item.stars == null ? item.metricsPending ? '获取中…' : '未提供' : numberText(item.stars)}</strong></span></span><span class="metric-copy rating-unavailable"><span>用户评分</span><strong>未提供</strong></span></div><div class="skill-evidence"><button class="text-button" data-skill-url="${esc('https://github.com/'+item.repository+'/issues')}">查看反馈</button>${item.statsURL ? `<button class="text-button" data-skill-url="${esc(item.statsURL)}">统计来源</button>` : ''}</div>${skillMetadataHTML(item)}</div><button class="button button-secondary skill-install" data-install-skill="${esc(item.id)}" data-install-target="${esc(skillInstallTarget)}" ${busy || item.installed || item.conflict ? 'disabled' : ''}>${icon(item.installed ? 'check' : 'download',14)}${item.installed ? '已安装' : item.conflict ? '同名已存在' : '安装'}</button></article>`).join('');
 const progress = marketProgress.length ? `<div class="market-progress" role="status" aria-live="polite"><div class="market-progress-heading"><strong>${marketPending() ? '正在加载市场' : '市场加载完成'}</strong><span>${marketProgress.filter((m)=>m.status!=='loading').length} / ${marketProgress.length}</span></div><div class="market-status-list">${marketProgress.map((m)=>`<span class="market-status ${m.status}">${icon(m.status==='loading'?'refresh':m.status==='error'?'close':'check',12)}${esc(m.name)}${m.status==='ready' ? ' · '+m.count : ''}${m.status==='error' ? ' · '+(language==='en'?'Failed':'失败') : ''}</span>`).join('')}</div></div>` : '';
 return `<form id="skill-search-form" class="skill-search"><label class="field"><span>技能市场</span><select id="skill-market" ${disabled}><option value="all" ${skillMarket==='all'?'selected':''}>全部市场</option>${currentMarkets().map((m)=>`<option value="${esc(m.id)}" ${skillMarket===m.id?'selected':''}>${esc(m.name)}</option>`).join('')}</select></label><label class="field"><span>搜索技能</span><input id="skill-query" type="search" placeholder="名称、描述或作者" value="${esc(skillQuery)}" ${disabled}></label><button class="button button-secondary" type="submit" ${disabled}>${icon('search',15)}搜索</button></form>${progress}<p class="market-data-note">默认加载全部支持的技能市场；共 ${items.length} 个技能，每页 ${skillPageSize} 个。安装量来自 skills.sh；仓库 Stars 仅代表仓库热度。当前市场未提供用户评分。</p>${skillsState.errors.map((err)=>`<div class="skill-market-error" role="alert">${esc(err)}</div>`).join('')}<div class="skills-list-head"><strong>市场技能 <span>${items.length}</span>${pages>1 ? ` <small>第 ${skillPage} / ${pages} 页</small>` : ''}</strong><div class="skill-list-actions"><label class="skill-sort"><span>安装到</span><select id="skill-install-target" ${disabled}><option value="active" ${skillInstallTarget==='active'?'selected':''}>当前应用</option><option value="shared" ${skillInstallTarget==='shared'?'selected':''}>共享技能</option></select></label><label class="skill-sort"><span>排序</span><select id="skill-sort" ${disabled}><option value="installs" ${skillSort==='installs'?'selected':''}>安装量优先</option><option value="stars" ${skillSort==='stars'?'selected':''}>仓库热度优先</option><option value="name" ${skillSort==='name'?'selected':''}>名称 A–Z</option><option value="source" ${skillSort==='source'?'selected':''}>按市场</option><option disabled>用户评分（未提供）</option></select></label></div></div><div class="skills-grid" aria-busy="${marketPending()}">${cards || `<div class="skill-empty">${marketPending() ? '正在读取技能市场…' : skillsState.errors.length ? '市场加载失败，请刷新重试。' : '没有匹配的技能，试试其他关键词或市场。'}</div>`}</div>${pages>1?`<nav class="skill-pagination" aria-label="技能分页"><button id="skill-prev" class="button button-secondary" ${skillPage===1 || busy?'disabled':''}>上一页</button><span>${skillPage} / ${pages}</span><button id="skill-next" class="button button-secondary" ${skillPage===pages || busy?'disabled':''}>下一页</button></nav>`:''}`;
}
function assembleMarketCatalog() {
 skillsState.catalog = selectedMarkets().flatMap((m)=>marketCatalogs.get(m.id)?.catalog || []);
 skillsState.errors = selectedMarkets().flatMap((m)=>marketCatalogs.get(m.id)?.errors || []);
}
async function loadSkillMarkets() {
 const generation = ++marketGeneration, target = state.target;
 const active = () => generation===marketGeneration && target===state.target && page==='skills' && skillSection==='market';
 const local = await LoadLocalSkillsState();
 if(!active())return;
 skillsState=local; const markets=selectedMarkets();
 marketProgress=markets.map((m)=>({...m,status:'loading',count:0})); assembleMarketCatalog(); render();
 for(const market of markets) {
  SearchSkills('',market.id).then((result)=>{
   marketCatalogs.set(market.id,{catalog:result.catalog,errors:result.errors});
   if(!active())return;
   // Keep the local installation snapshot fresh without accepting another target's result.
   if(result.target===target) { skillsState.skills=result.skills; skillsState.trash=result.trash; }
   const progress=marketProgress.find((m)=>m.id===market.id);progress.status=result.errors.length?'error':'ready';progress.count=result.catalog.length;
   assembleMarketCatalog(); render();
  }).catch((e)=>{
   if(!active())return;
   marketProgress.find((m)=>m.id===market.id).status='error';
   marketCatalogs.set(market.id,{catalog:marketCatalogs.get(market.id)?.catalog || [],errors:[market.name+': '+(e?.message||String(e))]});assembleMarketCatalog();render();
  });
  if(!marketMetrics.has(market.id) || marketMetrics.get(market.id).errors?.length) {
   marketMetrics.set(market.id,{loading:true});
   LoadSkillMarketMetrics(market.id).then((metrics)=>{marketMetrics.set(market.id,metrics);if(page==='skills' && skillSection==='market' && selectedMarkets().some((m)=>m.id===market.id))render();}).catch(()=>{marketMetrics.set(market.id,{errors:['unavailable']});if(page==='skills' && skillSection==='market' && selectedMarkets().some((m)=>m.id===market.id))render();});
  }
 }
}
async function refreshSkills() {
 if(skillSection==='market') { await loadSkillMarkets(); return; }
 skillsState=await LoadLocalSkillsState();
 selectedSkills=new Set([...selectedSkills].filter((name)=>skillsState.skills.some((s)=>s.name===name)));
}
async function refreshCLI(detectEnvironment = true) {
 cliState=await LoadCLIState(); if(!cliDirectory)cliDirectory=cliState.directory;
 selectedCLI=isPi()?'pi':'codex';
 if (detectEnvironment) await environment.refresh();
}
async function refreshWorkspace() {
 workspaceState = await LoadWorkspaceState();
 const active = workspaceState.projects.find((p) => p.id === workspaceState.activeId);
 workspaceForm = active ? { id: active.id, name: active.name, path: active.path } : { id: '', name: '', path: '' };
 agentConfig = active ? await LoadAgentConfig() : { workspace: {}, documents: [], codexHome: '', piHome: '', sharedSkillsRoot: '', sharedSkills: [] };
}
async function refreshAdvanced() { advancedState = await withTimeout(LoadAdvancedState(), 8000, '读取高级设置超时，请稍后重试'); }
function renderWorkspacePage(disabled) {
 const active = workspaceState.projects.find((p) => p.id === workspaceState.activeId);
 const projectRows = workspaceState.projects.map((project) => `<li class="workspace-project ${project.id === workspaceState.activeId ? 'selected' : ''}"><button data-workspace-select="${esc(project.id)}" ${disabled}><span class="workspace-project-icon">${icon('briefcase', 17)}</span><span><strong>${esc(project.name)}</strong><small>${esc(project.path)}</small></span></button><button class="icon-button workspace-project-delete" data-workspace-delete="${esc(project.id)}" aria-label="删除工作区 ${esc(project.name)}" ${disabled}>${icon('trash', 14)}</button></li>`).join('');
 const docs = agentConfig.documents || [];
 const grouped = docs.reduce((groups, doc) => { const key = `${doc.target}-${doc.scope}`; (groups[key] ||= []).push(doc); return groups; }, {});
 const docGroups = Object.entries(grouped).map(([key, entries]) => `<section class="agent-doc-group"><div class="panel-heading"><div>${icon(key.startsWith('pi') ? 'spark' : 'server', 17)}<h3>${key.startsWith('pi') ? 'pi agent' : 'Codex'} · ${key.endsWith('global') ? '全局指令' : '项目指令'}</h3></div></div><div class="agent-doc-list">${entries.map((doc) => `<button class="agent-doc-row ${doc.loaded ? 'loaded' : ''}" data-agent-doc="${esc(doc.path)}" ${disabled}><span class="agent-doc-icon">${icon('file', 16)}</span><span><strong>${esc(doc.kind)}</strong><small>${doc.exists ? (doc.loaded ? '已生效' : doc.overridden ? '被覆盖' : '已发现') : '不存在'} · ${esc(doc.path)}</small></span>${doc.loaded ? '<span class="skill-badge">已生效</span>' : ''}</button>`).join('')}</div></section>`).join('');
 const shared = agentConfig.sharedSkills || [];
 return `<section class="workspace-page"><div class="workspace-hero"><div><span class="eyebrow">工作空间</span><h2>${active ? esc(active.name) : '选择项目工作区'}</h2><p>${active ? '统一查看 Codex 与 pi agent 的指令链、共享技能和项目范围。' : '选择一个项目开始管理指令和共享技能。'}</p></div><div class="workspace-hero-actions"><button id="workspace-open" class="button button-secondary" ${disabled || !active ? 'disabled' : ''}>${icon('folder', 15)}打开项目</button><button id="workspace-refresh" class="button button-secondary" ${disabled}>${icon('refresh', 15)}刷新</button></div></div>
 <section class="workspace-picker panel"><div class="panel-heading"><div>${icon('briefcase', 18)}<h3>项目工作区</h3></div><span class="muted-label">${workspaceState.projects.length} 个项目</span></div><div class="workspace-form"><label class="field"><span>工作区名称</span><input id="workspace-name" value="${esc(workspaceForm.name)}" placeholder="例如：Model Switcher" maxlength="80" ${disabled}></label><label class="field workspace-path-field"><span>项目文件夹</span><input id="workspace-path" value="${esc(workspaceForm.path)}" placeholder="选择项目文件夹" ${disabled}></label><button id="workspace-choose" class="button button-secondary" ${disabled}>${icon('folder', 15)}选择文件夹</button><button id="workspace-save" class="button button-primary" ${disabled}>${icon('save', 15)}保存工作区</button></div><ul class="workspace-project-list">${projectRows || '<li class="workspace-empty">没有工作区，请选择一个项目文件夹。</li>'}</ul></section>
 ${active ? `<div class="workspace-columns"><section class="panel"><div class="panel-heading"><div>${icon('file', 18)}<h3>代理指令文件</h3></div><span class="muted-label">${docs.filter((d) => d.exists).length} 个已发现</span></div><p class="workspace-note">Codex 和 pi 会按全局、项目以及子目录的顺序加载指令。这里列出可能影响当前项目的文件，并标记当前实际生效的文件。</p>${docGroups || '<div class="workspace-empty">没有发现指令文件。</div>'}</section><section class="panel"><div class="panel-heading"><div>${icon('users', 18)}<h3>共享技能</h3></div><span class="muted-label">${shared.length} 个技能</span></div><p class="workspace-note">两个代理都能发现 <code>${esc(agentConfig.sharedSkillsRoot)}</code> 下的技能。适合放跨工具复用的 Agent Skills。</p><button id="shared-skills-open" class="button button-secondary" ${disabled}>${icon('folder', 15)}打开共享技能目录</button><div class="shared-skill-list">${shared.map((skill) => `<div class="shared-skill-row"><span>${icon('spark', 15)}</span><span><strong>${esc(skill.title || skill.name)}</strong><small>${esc(skill.description || '本地共享技能')}</small></span>${skill.hasScripts ? '<span class="skill-badge warning">包含脚本</span>' : ''}</div>`).join('') || '<div class="workspace-empty">没有共享技能。</div>'}</div></section></div>` : '<section class="workspace-empty workspace-empty-large">请选择一个工作区开始管理指令和共享技能。</section>'}</section>`;
}
function renderAdvancedPage(disabled) {
 const docs = advancedState.documents || [];
 const memoryRow = (memory) => `<li class="advanced-list-row"><button class="advanced-list-main" data-memory-read="${esc(memory.path)}" ${disabled}><span class="advanced-list-icon">${icon('brain', 16)}</span><span><strong>${esc(memory.title || memory.name)}</strong><small>${esc([memory.scope === 'project' ? (memory.project || '项目记忆') : '全局记忆', memory.name].concat((memory.tags || []).map((tag) => '#' + tag)).join(' · '))} · ${memory.bytes} B</small></span></button><button class="icon-button" data-memory-delete="${esc(memory.path)}" aria-label="删除记忆" ${disabled}>${icon('trash', 14)}</button></li>`;
 const codexRow = (memory) => `<li class="advanced-list-row"><button class="advanced-list-main" data-memory-read="${esc(memory.path)}" ${disabled}><span class="advanced-list-icon">${icon('shield', 16)}</span><span><strong>${esc(memory.title || memory.name)}</strong><small>${esc(memory.name)} · ${memory.bytes} B</small></span></button><button class="icon-button" data-memory-refine="${esc(memory.path)}" aria-label="提炼到共享记忆" ${disabled}>${icon('save', 14)}</button></li>`;
 const vaultMemories = advancedState.memories || [];
 const codexMemories = advancedState.codexMemories || [];
 const activeWorkspace = workspaceState.projects.find((project) => project.id === workspaceState.activeId);
 const vaultGroup = (title, hint, entries) => entries.length ? `<div class="advanced-doc-heading"><strong>${esc(title)}</strong><span class="muted-label">${esc(hint)}</span></div><ul class="advanced-list">${entries.map(memoryRow).join('')}</ul>` : '';
 const otherVaultMemories = vaultMemories.filter((memory) => memory.scope === 'project' && memory.projectId !== advancedState.currentProjectId);
 const otherVaultGroups = Object.entries(otherVaultMemories.reduce((groups, memory) => { const key = memory.project || memory.projectId || '项目记忆'; (groups[key] ||= []).push(memory); return groups; }, {})).map(([label, entries]) => vaultGroup(label, '其他项目记忆', entries)).join('');
 const vaultGroups = vaultGroup('全局记忆', '所有项目共用', vaultMemories.filter((memory) => memory.scope === 'global')) + vaultGroup(activeWorkspace ? activeWorkspace.name : '当前项目记忆', '仅在该项目使用', vaultMemories.filter((memory) => memory.scope === 'project' && memory.projectId === advancedState.currentProjectId)) + otherVaultGroups;
 const serverRows = (advancedState.mcpServers || []).map((server) => `<li class="advanced-list-row"><span class="advanced-list-icon">${icon('plug', 16)}</span><span><strong>${esc(server.name)}</strong><small>${esc(server.target)} · ${esc(server.scope)} · ${esc(server.path)}</small></span><span class="advanced-server-detail">${server.command ? '命令：' + esc(server.command) : server.endpoint ? '端点：' + esc(server.endpoint) : '配置已发现'}${server.enabled === false ? ' · 已禁用' : ''}</span></li>`).join('');
 const docRows = docs.map((doc) => `<li class="advanced-list-row advanced-doc-row"><button class="advanced-list-main" data-advanced-doc="${esc(doc.path)}" ${disabled}><span class="advanced-list-icon">${icon(doc.kind.toLowerCase().includes('mcp') ? 'plug' : 'file', 16)}</span><span><strong>${esc(doc.kind)}</strong><small title="${esc(doc.path)}">${esc(doc.path)}</small></span></button><span class="skill-badge ${doc.exists ? '' : 'muted'}">${doc.exists ? '已创建' : '未创建'}</span></li>`).join('');
 const tab = (id, label, iconName) => `<button class="advanced-tab ${advancedSection === id ? 'active' : ''}" data-advanced-section="${id}" ${disabled}>${icon(iconName, 16)}${label}</button>`;
 let content = '';
 if (advancedSection === 'memory') content = `<div class="advanced-grid"><section class="panel"><div class="panel-heading"><div>${icon('brain', 18)}<h3>共享记忆库</h3></div><span class="muted-label">${vaultMemories.length} 个</span></div><p class="workspace-note">Codex 和 pi agent 共用这一个记忆库，新建和编辑都写在这里，索引会自动更新。</p><div class="advanced-toolbar"><code>${esc(advancedState.memoryVault || '正在读取…')}</code><button id="advanced-memory-open" class="button button-secondary" ${disabled}>${icon('folder', 15)}打开目录</button><button id="advanced-memory-new" class="button button-primary" ${disabled}>${icon('plus', 15)}新建记忆</button></div>${vaultGroups || '<div class="workspace-empty workspace-empty-large">还没有共享记忆，点击「新建记忆」写下第一条。</div>'}</section><section class="panel"><div class="panel-heading"><div>${icon('shield', 18)}<h3>Codex 原生记忆</h3></div><span class="skill-badge">只读</span></div><p class="workspace-note">Codex 自己写入的记忆，本应用只读。打开一条可以复制内容，再存成共享记忆。</p><ul class="advanced-list">${codexMemories.map(codexRow).join('') || '<li class="workspace-empty">Codex 还没有生成记忆。</li>'}</ul><div class="advanced-toolbar"><button id="advanced-codex-memory-open" class="button button-secondary" ${disabled}>${icon('folder', 15)}打开目录</button></div></section></div><section class="panel"><div class="panel-heading"><div>${icon('spark', 18)}<h3>共享记忆怎么用</h3></div></div><div class="advanced-note-list"><p>全局记忆所有项目共用，项目记忆按项目分目录保存，两个代理都会读到同一份内容。</p><p>索引文件由本应用自动生成，请勿手动编辑。<code>INDEX.md</code></p>${isPi() ? '<p>pi CLI 会在每次提问前自动加载全局记忆和当前项目记忆，soul.md 可用于设定名字和身份。</p><p>已打开的 pi 会话请先执行 /reload，之后记忆修改会在下次提问时生效。</p>' : '<p>Codex 需要在全局指令中指向共享记忆索引。<code>~/.codex/AGENTS.md</code><code>~/.agents/memory/INDEX.md</code></p>'}<p>写入和删除前都会备份到应用数据目录。<code>backups/memories</code></p></div></section>`;
 if (advancedSection === 'mcp') content = `<div class="advanced-grid"><section class="panel"><div class="panel-heading"><div>${icon('plug', 18)}<h3>MCP 服务器</h3></div><span class="muted-label">${(advancedState.mcpServers || []).length} 个</span></div><p class="workspace-note">${isPi() ? 'pi agent 从 <code>mcp.json</code> 读取 MCP 配置。' : 'Codex 从 <code>config.toml</code> 和 <code>mcp.json</code> 读取 MCP 配置。'}这里先展示来源和启用状态，完整配置通过编辑器修改。</p><ul class="advanced-list">${serverRows || '<li class="workspace-empty">没有检测到 MCP 服务器。</li>'}</ul></section><section class="panel"><div class="panel-heading"><div>${icon('file', 18)}<h3>配置文件</h3></div></div><ul class="advanced-list">${docRows || '<li class="workspace-empty">没有配置文件。</li>'}</ul><div class="advanced-toolbar">${isPi() ? `<button id="advanced-pi-open" class="button button-secondary" ${disabled}>${icon('folder', 15)}打开 pi 目录</button>` : `<button id="advanced-codex-open" class="button button-secondary" ${disabled}>${icon('folder', 15)}打开 Codex 目录</button>`}</div></section></div>`;
 if (advancedSection === 'settings') content = `<section class="panel"><div class="panel-heading"><div>${icon('sliders', 18)}<h3>运行参数</h3></div><span class="skill-badge">${isPi() ? 'pi agent' : 'ChatGPT / Codex'}</span></div><p class="workspace-note">${isPi() ? 'pi settings 支持启动模型、思考级别、压缩、重试、图像、代理和终端显示设置。' : 'Codex 高级参数保存在 config.toml 中，可直接编辑并自动备份。'}</p><div class="advanced-settings-cards"><div class="advanced-setting-card"><strong>${isPi() ? 'pi settings.json' : 'Codex config.toml'}</strong><small>${esc(isPi() ? advancedState.piHome + '/settings.json' : advancedState.codexHome + '/config.toml')}</small><button class="button button-primary" data-advanced-doc="${esc(isPi() ? advancedState.piHome + '/settings.json' : advancedState.codexHome + '/config.toml')}" ${disabled}>${icon('edit', 15)}编辑运行参数</button></div><div class="advanced-setting-card"><strong>${isPi() ? '可调参数' : '建议参数'}</strong><small>${isPi() ? 'defaultThinkingLevel、thinkingBudgets、compaction、retry、terminal、images、httpProxy' : 'model_reasoning_effort、service_tier、request_max_retries、stream_idle_timeout_ms'}</small></div></div><div class="advanced-doc-heading"><strong>其他配置文件</strong><span class="muted-label">自动备份后写入</span></div><ul class="advanced-list">${docRows || '<li class="workspace-empty">没有配置文件。</li>'}</ul></section>`;
 const diagnostics = (advancedState.diagnostics || []).map((item) => `<li>${esc(item)}</li>`).join('');
 return `<section class="advanced-page"><div class="advanced-hero"><div><span class="eyebrow">代理工作台</span><h2>记忆、MCP 与运行参数</h2><p>把 Codex 和 pi agent 的高级配置放在同一处查看、备份和编辑。</p></div><div class="advanced-hero-actions"><button id="advanced-refresh" class="button button-secondary" ${disabled}>${icon('refresh', 15)}刷新</button></div></div><nav class="advanced-tabs" aria-label="高级设置">${tab('memory', '记忆管理', 'brain')}${tab('mcp', 'MCP 配置', 'plug')}${tab('settings', '运行参数', 'sliders')}</nav>${content}${diagnostics ? `<section class="panel advanced-diagnostics"><div class="panel-heading"><div>${icon('shield', 17)}<h3>诊断</h3></div></div><ul>${diagnostics}</ul></section>` : ''}</section>`;
}
function renderCLIPage(disabled) {
 const currentCLI = isPi() ? 'pi' : 'codex';
 const tools = cliState.tools.filter((tool) => tool.id === currentCLI).map((tool) => { const dependency = environment.getState().tools.find((item) => item.id === tool.id); return { ...tool, ready: dependency ? dependency.ready : tool.installed }; });
 return `<section class="cli-page"><div class="skills-page-heading"><div><h2>启动 CLI</h2><p>在系统终端中打开 ${isPi() ? 'pi agent' : 'Codex'}。</p></div><button id="cli-refresh" class="button button-secondary" ${disabled}>${icon('refresh',16)}重新检测</button></div>${environment.render(disabled)}<section class="panel cli-workspace"><label class="field"><span>工作目录</span><input id="cli-directory" value="${esc(cliDirectory)}" placeholder="选择项目所在文件夹" ${disabled}><small>CLI 会在这个目录中启动。</small></label><button id="cli-choose-directory" class="button button-secondary" ${disabled}>${icon('folder',16)}选择目录</button></section><div class="cli-grid">${tools.map((tool)=>`<article id="cli-card-${tool.id}" class="panel cli-card ${selectedCLI===tool.id?'selected':''}"><div class="cli-card-heading"><span class="cli-icon">${icon('terminal',23)}</span><div><h3>${esc(tool.name)}</h3><span class="cli-detected ${tool.installed?'found':''}">${tool.installed?'已检测到':'未安装或未找到'}</span></div><span class="skill-badge">当前应用</span></div><code>${esc(tool.path || tool.command)}</code>${tool.error?`<p class="cli-error">${esc(tool.error)}</p>`:''}<button class="button button-primary" data-launch-cli="${esc(tool.id)}" ${busy||environment.isInstalling()||!tool.ready?'disabled':''}>${icon('play',15)}${language==='en'?'Launch ':'启动 '}${esc(tool.name)}</button></article>`).join('') || '<div class="skill-empty">正在检测 CLI…</div>'}</div></section>`;
}
async function navigatePage(next, section = skillSection) {
 if (busy) return;
 if (next === 'chat' && !isPi()) return;
 marketGeneration++;
 page = next; skillSection = section; showTrash = section === 'trash'; treeExpanded[next] = true;
 resetPageScroll = true; error = ''; message = ''; render();
 if (page === 'skills') await run('skills', refreshSkills);
 if (page === 'cli') await run('cli-load', refreshCLI);
 if (page === 'workspace') await run('workspace-load', refreshWorkspace);
 if (page === 'advanced') await run('advanced-load', refreshAdvanced);
 if (page === 'chat') await piChat.refresh();
}
function renderUpdateManager() {
 const available = updateState.updateAvailable;
 const status = updateState.status === 'error' ? 'is-error' : available ? 'is-available' : '';
 return '<section class="update-manager"><div class="update-manager-head"><div><strong>更新管理</strong><small>检查新版本并在下载校验后自动重启。</small></div><button id="check-update" class="text-button">' + icon('refresh', 14) + '检查更新</button></div><div class="update-status ' + status + '"><span>当前版本 v' + esc(updateState.currentVersion || '0.2.24') + '</span><span>' + esc(updateState.message || '尚未检查') + '</span></div>' + (available ? '<button id="apply-update" class="button button-primary update-apply">' + icon('download', 15) + '更新到 v' + esc(updateState.latestVersion) + '</button>' : '') + '</section>';
}

function sidebarBounds() {
 const min = window.innerWidth < 740 ? 160 : 220;
 return { min, max: Math.max(min, Math.min(360, window.innerWidth - (window.innerWidth < 740 ? 190 : 580) - 8)) };
}
function applySidebarWidth(width = sidebarWidth, persist = false) {
 const { min, max } = sidebarBounds();
 sidebarWidth = Math.round(Math.max(min, Math.min(max, Number.isFinite(width) ? width : 256)));
 document.querySelector('.shell')?.style.setProperty('--sidebar-width', sidebarWidth + 'px');
 const splitter = document.getElementById('sidebar-resizer');
 if (splitter) { splitter.setAttribute('aria-valuenow', sidebarWidth); splitter.setAttribute('aria-valuemin', min); splitter.setAttribute('aria-valuemax', max); }
 const value = document.getElementById('sidebar-width-value');
 if (value) value.textContent = sidebarWidth + ' px';
 if (persist) localStorage.setItem('model-switcher-sidebar-width', sidebarWidth);
}
function bindSidebarResize() {
 const handle = document.getElementById('sidebar-resizer');
 if (!handle) return;
 let dragging = false;
 handle.addEventListener('pointerdown', (e) => {
  if (e.button !== 0) return;
  e.preventDefault(); dragging = true; handle.setPointerCapture(e.pointerId); handle.focus(); document.body.classList.add('resizing-sidebar');
 });
 handle.addEventListener('pointermove', (e) => { if (dragging) applySidebarWidth(e.clientX - document.querySelector('.shell').getBoundingClientRect().left); });
 const finish = () => { if (!dragging) return; dragging = false; document.body.classList.remove('resizing-sidebar'); applySidebarWidth(sidebarWidth, true); };
 handle.addEventListener('pointerup', finish); handle.addEventListener('pointercancel', finish); handle.addEventListener('lostpointercapture', finish);
 handle.addEventListener('dblclick', () => applySidebarWidth(256, true));
 handle.addEventListener('keydown', (e) => {
  const { min, max } = sidebarBounds();
  const next = { ArrowLeft: sidebarWidth - 16, ArrowRight: sidebarWidth + 16, Home: min, End: max }[e.key];
  if (next === undefined) return; e.preventDefault(); applySidebarWidth(next, true);
 });
}
function renderSidebar(disabled, title) {
 const modelActive = page === 'models', skillActive = page === 'skills', workspaceActive = page === 'workspace', advancedActive = page === 'advanced';
 const profiles = state.profiles.map((p) => `<li><button class="nav-child profile-nav ${modelActive && draft.id === p.id ? 'selected' : ''}" data-profile="${esc(p.id)}" title="${esc(p.name)}" ${modelActive && draft.id === p.id ? 'aria-current="page"' : ''} ${disabled}><span class="profile-dot ${p.id === state.activeProfileId ? 'live' : ''}"></span><span class="profile-copy"><strong>${esc(p.name)}</strong><small>${esc(p.selectedModel || '尚未选择模型')}</small></span>${p.id === state.activeProfileId ? '<span class="active-mini">当前</span>' : ''}</button></li>`).join('');
 const sections = [['installed', 'folder', '已安装'], ['market', 'download', '技能市场'], ['trash', 'trash', '回收区']];
 return `<aside class="sidebar" id="sidebar"><div class="brand"><div class="brand-mark">${icon('route', 30)}</div><div><div class="brand-name">Model Switcher</div><div class="brand-caption">模型与代理工作台</div></div></div>
 <nav class="sidebar-nav" aria-label="主导航"><div class="nav-caption">工作空间</div><ul class="nav-tree"><li><div class="nav-group-heading ${workspaceActive ? 'active' : ''}"><button class="nav-group-link" data-page="workspace" ${disabled}>${icon('briefcase',18)}<span>工作区</span>${workspaceState.projects.length ? `<span class="nav-count">${workspaceState.projects.length}</span>` : ''}</button></div></li>
 <li><div class="nav-group-heading ${modelActive ? 'active' : ''}"><button class="nav-disclosure" id="toggle-models" data-toggle-section="models" aria-label="展开或折叠模型设置" aria-expanded="${treeExpanded.models}" aria-controls="model-nav">${icon(treeExpanded.models ? 'chevronDown' : 'chevronRight', 15)}</button><button class="nav-group-link" data-page="models" ${disabled}>${icon('server', 18)}<span>模型设置</span><span class="nav-count">${state.profiles.length}</span></button></div>
 <ul class="nav-children" id="model-nav" ${treeExpanded.models ? '' : 'hidden'}>${profiles}<li><button class="nav-child nav-add ${modelActive && !draft.id ? 'selected' : ''}" id="new" ${disabled}>${icon('plus', 16)}<span>添加本地配置</span></button></li><li class="nav-paste"><button class="nav-child" id="paste-profile" title="从剪贴板新建配置" ${disabled}>${icon('paste', 16)}<span>${busy === 'profile-paste' ? '正在粘贴…' : '粘贴配置'}</span></button></li></ul></li>
 ${isPi() ? `<li><div class="nav-group-heading ${page === 'chat' ? 'active' : ''}"><button class="nav-group-link chat-nav-link" data-page="chat" ${page === 'chat' ? 'aria-current="page"' : ''} ${disabled}>${icon('chat', 18)}<span>对话</span></button></div></li>` : ''}
 <li><div class="nav-group-heading ${skillActive ? 'active' : ''}"><button class="nav-disclosure" id="toggle-skills" data-toggle-section="skills" aria-label="展开或折叠技能管理" aria-expanded="${treeExpanded.skills}" aria-controls="skill-nav">${icon(treeExpanded.skills ? 'chevronDown' : 'chevronRight', 15)}</button><button class="nav-group-link" data-page="skills" ${disabled}>${icon('spark', 18)}<span>技能管理</span></button></div>
 <ul class="nav-children" id="skill-nav" ${treeExpanded.skills ? '' : 'hidden'}>${sections.map(([id, symbol, label]) => `<li><button class="nav-child ${skillActive && skillSection === id ? 'selected' : ''}" data-skill-section="${id}" ${skillActive && skillSection === id ? 'aria-current="page"' : ''} ${disabled}>${icon(symbol, 16)}<span>${label}</span></button></li>`).join('')}</ul></li><li><div class="nav-group-heading ${page==='cli'?'active':''}"><button class="nav-disclosure" id="toggle-cli" data-toggle-section="cli" aria-label="展开或折叠 CLI 启动器" aria-expanded="${treeExpanded.cli}" aria-controls="cli-nav">${icon(treeExpanded.cli?'chevronDown':'chevronRight',15)}</button><button class="nav-group-link" data-page="cli" ${disabled}>${icon('terminal',18)}<span>CLI 启动器</span></button></div><ul class="nav-children" id="cli-nav" ${treeExpanded.cli?'':'hidden'}>${[[isPi() ? 'pi' : 'codex', isPi() ? 'pi CLI' : 'Codex CLI']].map(([id,name])=>`<li><button id="nav-cli-${id}" class="nav-child ${page==='cli'&&selectedCLI===id?'selected':''}" data-cli-nav="${id}" ${disabled}>${icon('terminal',16)}<span>${name}</span></button></li>`).join('')}</ul></li><li><div class="nav-group-heading ${advancedActive?'active':''}"><button class="nav-disclosure" id="toggle-advanced" data-toggle-section="advanced" aria-label="展开或折叠高级设置" aria-expanded="${treeExpanded.advanced}" aria-controls="advanced-nav">${icon(treeExpanded.advanced?'chevronDown':'chevronRight',15)}</button><button class="nav-group-link" data-page="advanced" ${disabled}>${icon('sliders',18)}<span>高级设置</span></button></div><ul class="nav-children" id="advanced-nav" ${treeExpanded.advanced?'':'hidden'}>${[['memory','brain','记忆管理'],['mcp','plug','MCP 配置'],['settings','sliders','运行参数']].map(([id,symbol,label])=>`<li><button class="nav-child ${advancedActive && advancedSection === id ? 'selected' : ''}" data-advanced-section="${id}" ${disabled}>${icon(symbol,16)}<span>${label}</span></button></li>`).join('')}</ul></li></ul></nav>
 <div class="sidebar-bottom"><button class="connection-card" id="settings" ${disabled} aria-label="打开设置" title="打开设置"><span class="connection-icon">${icon(currentIsLocal() ? 'server' : 'cloud', 19)}</span><span class="connection-copy"><span class="connection-target">${isPi() ? 'pi agent' : 'ChatGPT / Codex'}</span><strong>${esc(title)}</strong><small>${esc(state.activeModel || '使用默认模型')}</small></span>${icon('settings', 17)}</button><div class="sidebar-foot"><span class="status-label"><span class="status-dot"></span>${isPi() ? 'pi agent 配置' : state.chatGptRunning ? 'ChatGPT 运行中' : 'ChatGPT 未运行'}</span><span>v${esc(state.version)}</span></div></div></aside>
 <div id="sidebar-resizer" class="sidebar-resizer" role="separator" aria-orientation="vertical" aria-label="调整菜单栏宽度" aria-controls="sidebar" tabindex="0" title="拖动调整宽度；方向键微调，双击恢复默认"><span></span></div>`;
}
function renderModelPage(disabled, title) {
 const selected = draft.models.find((m) => m.id === draft.selectedModel);
 const activeEndpoint = currentIsLocal() ? profileFor(state.activeProfileId)?.baseUrl : '';
 const rows = draft.models.length ? draft.models.slice(0, 6).map((m) => `<div class="model-row ${m.id === draft.selectedModel ? 'chosen' : ''}">${icon(m.id === draft.selectedModel ? 'check' : 'server', 15)}<span>${esc(m.id)}</span><small class="model-context">${formatContextWindow(m.contextWindow)}</small>${m.supportsImages && isPi() ? '<span class="skill-badge">图像</span>' : ''}</div>`).join('') : `<div class="model-empty">${icon('server', 28)}<strong>先连接你的本地服务</strong><p>填写 API 地址，然后点击「获取模型」。</p></div>`;
 return `<section class="provider-summary" aria-label="模型连接"><span class="summary-icon">${icon('route', 26)}</span><div class="summary-copy"><span>当前使用</span><strong>${esc(title)}</strong>${activeEndpoint ? `<small class="summary-endpoint">${esc(activeEndpoint)}</small>` : ''}</div><span class="route-line" aria-hidden="true"></span><div class="summary-model"><span>${esc(state.activeModel || '使用默认模型')}</span><small>${isPi() ? 'pi agent' : 'ChatGPT / Codex'}</small></div><button id="restore" class="button button-secondary" ${disabled}${state.canRestore ? '' : ' disabled'}>${icon('refresh', 15)}${isPi() ? '恢复原配置' : '切回 OpenAI'}</button></section>
 <div class="editor-heading"><div><h2>${esc(draft.name || '新建配置')}</h2><p>${draft.id ? '编辑连接信息并应用到当前应用。' : '添加一个 OpenAI 兼容的模型服务。'}</p></div><button id="activate-current" class="button button-primary" ${disabled}${draft.selectedModel ? '' : ' disabled'}>${icon('play', 15)}${isPi() ? '应用到 pi agent' : '应用到 ChatGPT'}</button></div>
 <div class="workspace-grid"><section class="panel config-panel"><div class="panel-heading"><div>${icon('server', 18)}<h3>服务连接</h3></div><span class="muted-label">${draft.id ? '编辑配置' : '新建配置'}</span></div><div class="form-grid">
 <label class="field"><span>配置名称</span><input id="name" placeholder="例如：我的 LM Studio" value="${esc(draft.name)}" maxlength="80" ${disabled}></label>
 <label class="field"><span>API 地址</span><input id="url" type="url" placeholder="http://127.0.0.1:1234/v1" value="${esc(draft.baseUrl)}" ${disabled}><small>支持本机、局域网和远程 OpenAI 兼容服务。</small></label>
 <label class="field"><span>API Key <em>可选</em></span><div class="secret-field"><input id="key" type="${keyVisible ? 'text' : 'password'}" autocomplete="off" value="${esc(draft.apiKey)}" placeholder="${draft.hasApiKey && !draft.apiKey ? 'Key 已保存但无法读取，请重新填写' : '服务无需认证时可留空'}" ${disabled}><div class="secret-actions"><button type="button" class="icon-button" id="key-toggle" aria-label="${keyVisible ? '隐藏 API Key' : '显示 API Key'}" title="${keyVisible ? '隐藏 API Key' : '显示 API Key'}" ${disabled}>${icon(keyVisible ? 'eyeOff' : 'eye', 16)}</button><button type="button" class="icon-button" id="key-copy" aria-label="复制 API Key" title="复制 API Key" ${disabled || !draft.apiKey ? 'disabled' : ''}>${icon('copy', 16)}</button></div></div><small>Key 直接保存在 profiles.json，界面默认隐藏；可显示或复制。</small></label>
 ${draft.hasApiKey ? `<label class="clear-key"><input id="clear-key" type="checkbox" ${draft.clearApiKey ? 'checked' : ''} ${disabled}>移除已保存的 Key</label>` : ''}</div><div class="form-actions"><button class="button button-secondary" id="save" ${disabled}>${icon('save', 16)}${busy === 'save' ? '正在保存…' : '保存'}</button><button class="button button-secondary" id="fetch" ${disabled}>${icon('refresh', 16)}${busy === 'fetch' ? '正在获取…' : '获取模型'}</button><button class="button button-secondary" id="copy-profile" title="复制连接信息、API Key 和模型设置" ${disabled}>${icon('copy', 16)}${busy === 'profile-copy' ? '正在复制…' : '复制配置'}</button></div></section>
 <section class="panel model-panel"><div class="panel-heading"><div>${icon('spark', 18)}<h3>选择模型</h3></div><span class="muted-label">${draft.models.length} 个可用</span></div><div class="model-selection-grid"><label class="field"><span>使用的模型</span><select id="model" ${disabled}${draft.models.length ? '' : ' disabled'}><option value="">${draft.models.length ? '选择一个模型' : '等待获取模型列表'}</option>${draft.models.map((m) => `<option value="${esc(m.id)}" ${m.id === draft.selectedModel ? 'selected' : ''}>${esc(m.id)}</option>`).join('')}</select></label><label class="field"><span>上下文大小</span><div class="context-window-input"><input id="model-context-window" type="number" min="1024" step="1024" value="${contextWindowValue(selected?.contextWindow)}" ${disabled}${draft.selectedModel ? '' : ' disabled'}><span>tokens</span></div><small>默认 256K；应用后由当前模型直接使用。</small></label></div>
 ${isPi() ? `<div class="model-capability"><label><input id="model-images" type="checkbox" ${selected?.supportsImages ? 'checked' : ''} ${disabled}${draft.selectedModel ? '' : ' disabled'}>支持图像输入</label><small>仅在模型和 API 支持图像时启用。</small></div>` : ''}<div class="model-list">${rows}</div><p class="model-hint">${isPi() ? 'pi agent 使用 <strong>/v1/chat/completions</strong>' : '接入 Codex 需要兼容 <strong>/v1/responses</strong>'}</p></section></div>
 ${isPi() ? `<section class="connection-details"><strong>配置位置</strong><code>${esc(state.configPath)}</code><small>切换后在 pi 中使用 /model 选择模型；新会话使用已保存的默认模型。技能安装后使用 /reload。</small></section>` : `<section class="connection-details"><div class="path-heading"><label id="chatgpt-path-label">ChatGPT 路径</label><span class="muted-label">${state.chatGptTarget ? '手动选择' : '自动查找'}</span><button class="text-button" id="auto-target" ${disabled}>${icon('search', 15)}自动查找</button></div><button id="chatgpt-target" class="target-picker" aria-labelledby="chatgpt-path-label" ${disabled}>${icon('folder', 17)}<span>${esc(state.chatGptResolvedTarget || state.chatGptTarget || '未找到 ChatGPT，点击选择应用')}</span></button>${state.chatGptTargetError ? `<small class="is-error">${esc(state.chatGptTargetError)}</small>` : ''}</section>`}
 <section class="panel model-test-panel"><div class="panel-heading"><div>${icon('terminal', 18)}<h3>测试对话</h3></div><span class="muted-label">${isPi() ? '/v1/chat/completions' : '/v1/responses'}</span></div><label class="field"><span>发送给当前模型</span><textarea id="model-test-input" class="model-test-input" rows="3" maxlength="4000" placeholder="输入要发送给模型的内容，例如：你好，请介绍一下你自己。" ${disabled}>${esc(modelTestInput)}</textarea></label><div class="model-test-actions"><span class="model-test-hint">当前模型：${esc(draft.selectedModel || '请先选择模型')}</span><button class="button button-primary" id="test-model" ${disabled}${draft.selectedModel ? '' : ' disabled'}>${icon('play', 16)}${busy === 'test-model' ? '正在发送…' : '发送测试'}</button></div>${modelTestReply ? `<div class="model-test-answer" role="status"><div class="model-test-answer-head"><strong>模型回答</strong><span>${esc(modelTestModel || draft.selectedModel)} · ${esc(modelTestProtocol || (isPi() ? 'chat.completions' : 'responses'))}</span></div><div class="model-test-markdown">${markdownHTML(modelTestReply)}</div></div>` : `<div class="model-test-empty">输入一段话并点击“发送测试”，模型的完整回答会显示在这里。</div>`}</section>
 <footer class="config-links"><button id="codex-dir" class="text-button" ${disabled}>${icon('folder', 16)}${isPi() ? '打开 pi 配置目录' : '打开 .codex 目录'}</button><button id="config-view" class="text-button" ${disabled}>${icon('edit', 16)}查看 ${configName()}</button>${draft.id ? `<button id="delete" class="text-button delete-button" ${disabled}>${icon('trash', 16)}删除配置</button>` : ''}</footer>`;
}
function renderSettings() {
 return `<div class="settings-modal" role="dialog" aria-modal="true" aria-labelledby="settings-title"><div class="settings-card"><div class="settings-head"><div><h3 id="settings-title">设置</h3><small>Model Switcher · v${esc(state.version)}</small></div><button class="icon-button" id="settings-close" aria-label="关闭设置">${icon('close', 18)}</button></div>
 <div class="settings-feedback ${error ? 'is-error' : ''}" role="status">${esc(error || (busy ? '正在处理…' : ''))}</div>
 <div class="settings-row"><div><strong>语言</strong><small>界面语言和外观会保存在本机。</small></div><div class="settings-segment"><button id="lang-zh" class="${language === 'zh' ? 'active' : ''}">中文</button><button id="lang-en" class="${language === 'en' ? 'active' : ''}">English</button></div></div>
 <div class="settings-row"><div><strong>主题</strong><small>选择适合当前环境的显示模式。</small></div><div class="settings-segment"><button id="theme-light" class="${theme === 'light' ? 'active' : ''}">浅色</button><button id="theme-dark" class="${theme === 'dark' ? 'active' : ''}">深色</button></div></div>
 <div class="settings-row"><div><strong>菜单栏宽度</strong><small>拖动分隔条或用方向键调整。</small></div><div class="sidebar-size-controls"><button class="icon-button" id="sidebar-shrink" aria-label="缩窄菜单栏">${icon('minus', 16)}</button><output id="sidebar-width-value">${sidebarWidth} px</output><button class="icon-button" id="sidebar-expand" aria-label="加宽菜单栏">${icon('plus', 16)}</button></div></div>
 <div class="settings-row settings-path-row"><div><strong>配置文件位置</strong><small>启动时实际读取的 profiles.json；复制的 App 可能被 macOS 临时转移。</small></div><code title="${esc(state.profilePath || '')}">${esc(state.profilePath || '未找到，将在保存时创建')}</code></div>${renderUpdateManager()}</div></div>`;
}
function render() {
 piChat.syncState();
 if (page === 'chat' && !isPi()) page = 'models';
 const openDetails = [...app.querySelectorAll('details[open][id]')].map((el)=>el.id);
 const focusID = (!busy && restoreFocusID) || document.activeElement?.id;
 if (!busy) restoreFocusID = '';
 const selection = document.activeElement?.selectionStart;
 const scroll = document.querySelector('.page-body')?.scrollTop || 0;
 const navScroll = document.querySelector('.sidebar-nav')?.scrollTop || 0;
 const title = currentIsLocal() ? (profileFor(state.activeProfileId)?.name || '本地模型') : isPi() ? (state.activeProvider === 'automatic' ? 'pi 默认模型' : state.activeProvider) : 'OpenAI 云端';
 const disabled = busy || !loaded ? 'disabled' : '';
 const sectionTitle = { installed: '已安装', market: '技能市场', trash: '回收区' }[skillSection];
 const pageTitle = page === 'chat' ? '对话' : page === 'models' ? '模型设置' : page === 'cli' ? 'CLI 启动器' : page === 'workspace' ? '工作区' : page === 'advanced' ? '高级设置' : '技能管理';
 const pageSubtitle = page === 'chat' ? '与当前 pi 模型进行多轮对话，实时查看回复。' : page === 'models' ? '连接模型服务，选择模型并应用配置。' : page === 'cli' ? '选择工作目录，启动对应的命令行代理。' : page === 'workspace' ? '管理项目指令链和两个代理都能发现的共享技能。' : page === 'advanced' ? '管理记忆、MCP 服务器和高级运行参数。' : '搜索、安装和整理当前应用的技能。';
 app.innerHTML = `<div class="shell">${renderSidebar(disabled, title)}<main class="content ${page === 'chat' ? 'conversation-content' : ''}" id="main-content"><header class="page-header"><div class="page-title"><h1>${pageTitle}</h1><p>${pageSubtitle}</p></div><div class="page-actions"><label class="target-select"><span>当前应用</span><select id="app-target" ${disabled}><option value="chatgpt" ${isPi() ? '' : 'selected'}>ChatGPT / Codex</option><option value="pi" ${isPi() ? 'selected' : ''}>pi agent</option></select></label><button class="icon-button" id="reload" title="刷新当前状态" aria-label="刷新当前状态" ${disabled}>${icon('refresh', 18)}</button></div></header>
 <div class="page-body ${page === 'chat' ? 'chat-page-body' : ''}"><div class="feedback ${error ? 'is-error' : ''}" role="status">${esc(error || (busy ? '正在处理…' : message || (dirty ? '有未保存的修改。' : '')))}</div>${page !== 'cli' ? environment.notice(state.target) : ''}${page === 'chat' ? piChat.render(disabled) : page === 'models' ? renderModelPage(disabled, title) : page === 'cli' ? renderCLIPage(disabled) : page === 'workspace' ? renderWorkspacePage(disabled) : page === 'advanced' ? renderAdvancedPage(disabled) : `<section class="skills-page"><div class="skills-page-heading"><div><h2>${sectionTitle}</h2><p>${isPi() ? 'pi agent' : 'ChatGPT / Codex'} · ${sectionTitle === '技能市场' ? '从支持的市场安装技能。' : '管理当前应用的本地技能。'}</p></div><button id="skills-refresh" class="button button-secondary" ${disabled}>${icon('refresh', 16)}刷新技能</button></div><div class="skills-root"><span>目录</span><code>${esc(skillsState.root || '正在读取…')}</code></div>${renderSkillRows()}</section>`}</div></main></div>`;
 if (configPreview !== null) app.insertAdjacentHTML('beforeend', `<div class="config-modal" role="dialog" aria-modal="true" aria-labelledby="config-title"><div class="config-modal-card"><div class="config-modal-head"><div><h3 id="config-title">编辑 ${configName()}</h3></div><button class="icon-button" id="config-close" aria-label="关闭配置编辑">${icon('close', 18)}</button></div><textarea id="config-editor" class="config-preview" aria-label="${configName()}" spellcheck="false">${esc(configPreview)}</textarea><div class="config-modal-actions"><span>${isPi() ? '保存前会校验 JSON/JSONC，并自动备份原文件。' : '保存前会校验 TOML，并自动备份原文件。'}</span><button class="button button-secondary" id="config-save">${icon('save', 16)}保存 ${configName()}</button></div></div></div>`);
 if (agentPreview !== null) app.insertAdjacentHTML('beforeend', `<div class="config-modal" role="dialog" aria-modal="true" aria-labelledby="agent-title"><div class="config-modal-card"><div class="config-modal-head"><div><h3 id="agent-title">编辑代理指令</h3><small>${esc(agentPreview.path)}</small></div><button class="icon-button" id="agent-close" aria-label="关闭指令编辑">${icon('close', 18)}</button></div><textarea id="agent-editor" class="config-preview" aria-label="文件内容" spellcheck="false">${esc(agentPreview.content)}</textarea><div class="config-modal-actions"><span>保存前会自动备份原文件；保存后重新加载作用链。</span><button class="button button-secondary" id="agent-save">${icon('save', 16)}保存指令</button></div></div></div>`);
 if (advancedPreview !== null) app.insertAdjacentHTML('beforeend', `<div class="config-modal" role="dialog" aria-modal="true" aria-labelledby="advanced-title"><div class="config-modal-card"><div class="config-modal-head"><div><h3 id="advanced-title">${esc(advancedPreview.title || '编辑高级配置')}</h3><small>${esc(advancedPreview.path || advancedState.memoryVault || '')}</small></div><button class="icon-button" id="advanced-close" aria-label="关闭高级配置编辑">${icon('close', 18)}</button></div>${advancedPreview.isNew ? `<div class="advanced-memory-fields"><label class="field advanced-memory-name"><span>记忆名称</span><input id="advanced-memory-name" value="${esc(advancedPreview.name || '')}" placeholder="例如：project-context.md" maxlength="100"></label><label class="field advanced-memory-scope"><span>保存位置</span><select id="advanced-memory-scope"><option value="global"${advancedPreview.scope === 'project' ? '' : ' selected'}>全局记忆（所有项目共用）</option><option value="project"${advancedPreview.scope === 'project' ? ' selected' : ''}>当前项目记忆</option></select></label></div>` : ''}<textarea id="advanced-editor" class="config-preview" aria-label="${advancedPreview.isNew ? '记忆内容' : '配置内容'}" spellcheck="false"${advancedPreview.readOnly ? ' readonly' : ''}>${esc(advancedPreview.content || '')}</textarea><div class="config-modal-actions"><span>${advancedPreview.readOnly ? '这是代理自己生成的记忆，本应用只读；可复制内容后新建共享记忆。' : advancedPreview.kind === 'memory' ? '保存前会自动备份原记忆；删除会移入应用备份目录。' : '保存前会校验 JSON/JSONC 或 TOML，并自动备份原文件。'}</span>${advancedPreview.readOnly ? '' : `<button class="button button-secondary" id="advanced-save">${icon('save', 16)}${advancedPreview.kind === 'memory' ? '保存记忆' : '保存配置'}</button>`}</div></div></div>`);
 if (settingsOpen) app.insertAdjacentHTML('beforeend', renderSettings());
 openDetails.forEach((id)=>{ const detail=document.getElementById(id); if(detail)detail.open=true; });
 applyPreferences(); localizeDOM(); applySidebarWidth();
 document.querySelector('.shell').inert = settingsOpen || configPreview !== null || agentPreview !== null || advancedPreview !== null;
 document.querySelector('.sidebar-nav').scrollTop = navScroll;
 document.querySelector('.page-body').scrollTop = resetPageScroll ? 0 : scroll; resetPageScroll = false;
 if (busy) app.querySelectorAll('.settings-card button:not(#settings-close), .config-modal-card button:not(#config-close)').forEach((b) => { b.disabled = true; });
 bind(); bindSidebarResize();
 const focus = document.getElementById(focusID);
 if (focus && !focus.disabled && !focus.closest('[inert]')) { focus.focus({preventScroll:true}); if (typeof selection === 'number' && focus.setSelectionRange && ['search', 'text', 'password'].includes(focus.type)) focus.setSelectionRange(selection, selection); }
 else if (settingsOpen) document.getElementById('settings-close')?.focus({preventScroll:true});
 else if (configPreview !== null) document.getElementById('config-close')?.focus({preventScroll:true});
 else if (agentPreview !== null) document.getElementById('agent-close')?.focus({preventScroll:true});
 else if (advancedPreview !== null) document.getElementById('advanced-close')?.focus({preventScroll:true});
}

function bind() {
 environment.bind(() => navigatePage('cli'));
 piChat.bind({ openModels: () => navigatePage('models'), switchProfile: (id) => run('chat-profile', async () => { state = await ActivateProfile(id); piChat.syncState(); await piChat.refresh(); }) });
 const on = (id, event, fn) => document.getElementById(id)?.addEventListener(event, fn);
 on('name', 'input', (e) => { draft.name = e.target.value; dirty = true; });
 on('url', 'input', (e) => { draft.baseUrl = e.target.value; dirty = true; });
 on('key', 'input', (e) => { draft.apiKey = e.target.value; dirty = true; });
 on('key-toggle', 'click', () => { keyVisible = !keyVisible; render(); });
 on('key-copy', 'click', async () => {
  if (!draft.apiKey) return;
  try {
   await writeClipboardText(draft.apiKey);
   message = 'API Key 已复制。'; error = ''; render();
  } catch { error = '无法复制 API Key，请手动选择后复制。'; render(); }
 });
 on('model-test-input', 'input', (e) => { modelTestInput = e.target.value; });
 on('model-test-input', 'keydown', (e) => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); document.getElementById('test-model')?.click(); } });
 on('clear-key', 'change', (e) => { draft.clearApiKey = e.target.checked; dirty = true; });
 on('model', 'change', (e) => { draft.selectedModel = e.target.value; dirty = true; render(); });
 on('model-context-window', 'change', (e) => { const value = contextWindowValue(e.target.value); draft.models = draft.models.map((m) => m.id === draft.selectedModel ? { ...m, contextWindow: value } : m); dirty = true; render(); });
 document.querySelectorAll('[data-profile]').forEach((b) => b.addEventListener('click', () => { page = 'models'; treeExpanded.models = true; resetPageScroll = true; edit(profileFor(b.dataset.profile)); }));
 document.querySelectorAll('[data-page]').forEach((b) => b.addEventListener('click', () => navigatePage(b.dataset.page)));
 document.querySelectorAll('[data-skill-section]').forEach((b) => b.addEventListener('click', () => navigatePage('skills', b.dataset.skillSection)));
 document.querySelectorAll('[data-toggle-section]').forEach((b) => b.addEventListener('click', () => { treeExpanded[b.dataset.toggleSection] = !treeExpanded[b.dataset.toggleSection]; localStorage.setItem('model-switcher-navigation', JSON.stringify(treeExpanded)); render(); }));
 on('sidebar-shrink', 'click', () => applySidebarWidth(sidebarWidth - 16, true));
 on('sidebar-expand', 'click', () => applySidebarWidth(sidebarWidth + 16, true));
 on('activate-current', 'click', () => run('activate', async () => { if (dirty || !draft.id) await save(); state = await ActivateProfile(draft.id); message = isPi() ? 'pi 默认模型已保存；在 pi 中使用 /model 选择模型或开启新会话。' : '已切换到「' + draft.name + '」，ChatGPT 正在重启。'; }));
 on('model-images', 'change', (e) => { draft.models = draft.models.map((m) => m.id === draft.selectedModel ? { ...m, supportsImages: e.target.checked } : m); dirty = true; render(); });
 on('new', 'click', () => { page = 'models'; treeExpanded.models = true; resetPageScroll = true; edit(); });
 on('copy-profile', 'click', () => run('profile-copy', async () => {
  await writeClipboardText(serializeProfile(draft));
  message = '配置已复制到剪贴板。';
 }));
 on('paste-profile', 'click', () => run('profile-paste', async () => {
  const profile = parseProfile(await readClipboardText());
  page = 'models'; treeExpanded.models = true; resetPageScroll = true;
  draft = profile; dirty = true; keyVisible = false;
  modelTestReply = ''; modelTestModel = ''; modelTestProtocol = '';
  message = '已粘贴为新配置，保存后即可使用。';
  restoreFocusID = 'name';
 }));
 on('settings', 'click', () => { settingsOpen = true; render(); run('settings-load', async () => { updateState = await CheckForUpdate(); }); });
 on('settings-close', 'click', () => { settingsOpen = false; render(); document.getElementById('settings')?.focus(); });
 on('skills-refresh', 'click', () => run('skills', refreshSkills));
 on('skill-sort', 'change', (e)=>{skillSort=e.target.value;localStorage.setItem('model-switcher-skill-sort',skillSort);skillPage=1;render();});
 on('skill-install-target', 'change', (e)=>{skillInstallTarget=e.target.value;render();});
 on('skill-prev', 'click', ()=>{skillPage--;resetPageScroll=true;render();});
 on('skill-next', 'click', ()=>{skillPage++;resetPageScroll=true;render();});
 document.querySelectorAll('[data-cli-nav]').forEach((b)=>b.addEventListener('click',()=>{selectedCLI=b.dataset.cliNav;navigatePage('cli');}));
 on('cli-refresh','click',()=>run('cli-load',refreshCLI));
 on('cli-directory','input',(e)=>{cliDirectory=e.target.value;});
 on('cli-choose-directory','click',()=>run('cli-directory',async()=>{const selected=await ChooseCLIDirectory(cliDirectory);if(selected){cliDirectory=selected;localStorage.setItem('model-switcher-cli-directory',cliDirectory);}}));
 document.querySelectorAll('[data-launch-cli]').forEach((b)=>b.addEventListener('click',()=>run('cli-launch',async()=>{await LaunchCLI(b.dataset.launchCli,cliDirectory);localStorage.setItem('model-switcher-cli-directory',cliDirectory);message=language==='en'?'CLI opened in your system terminal.':'CLI 已在系统终端中打开。';})));
 on('skill-query', 'input', (e) => { skillQuery = e.target.value; });
 on('skill-market', 'change', (e) => { skillMarket = e.target.value; skillPage=1; run('skill-search', refreshSkills); });
 on('skill-search-form', 'submit', (e) => { e.preventDefault(); skillPage=1; resetPageScroll=true; render(); });
 document.querySelectorAll('[data-select-skill]').forEach((b) => b.addEventListener('change', () => { if (b.checked) selectedSkills.add(b.dataset.selectSkill); else selectedSkills.delete(b.dataset.selectSkill); const clean = document.getElementById('skills-clean'); clean.disabled = !selectedSkills.size; clean.previousElementSibling.textContent = (language === 'en' ? 'Selected: ' : '已选择：') + selectedSkills.size; }));
 on('skills-clean', 'click', () => run('skill-clean', async () => { await CleanSkills([...selectedSkills]); selectedSkills.clear(); await refreshSkills(); message = '所选技能已移入回收区，可恢复。'; }));
 document.querySelectorAll('[data-restore-skill]').forEach((b) => b.addEventListener('click', () => run('skill-restore', async () => { await RestoreSkill(b.dataset.restoreSkill); await refreshSkills(); message = '技能已恢复。'; })));
 document.querySelectorAll('[data-skill-url]').forEach((b) => b.addEventListener('click', () => { if (/^https:\/\/(github\.com|skills\.sh)\//.test(b.dataset.skillUrl)) BrowserOpenURL(b.dataset.skillUrl); }));
 on('workspace-name', 'input', (e) => { workspaceForm.name = e.target.value; });
 on('workspace-path', 'input', (e) => { workspaceForm.path = e.target.value; });
 on('workspace-choose', 'click', () => run('workspace-choose', async () => { const selected = await ChooseWorkspaceDirectory(workspaceForm.path); if (selected) { workspaceForm.path = selected; if (!workspaceForm.name) workspaceForm.name = selected.split(/[\\/]/).filter(Boolean).pop() || ''; render(); } }));
 on('workspace-save', 'click', () => run('workspace-save', async () => { workspaceState = await SaveWorkspace(workspaceForm.id, workspaceForm.name, workspaceForm.path); await refreshWorkspace(); message = '工作区已保存。'; }));
 on('workspace-refresh', 'click', () => run('workspace-refresh', refreshWorkspace));
 on('workspace-open', 'click', () => run('workspace-open', OpenWorkspaceDirectory));
 on('shared-skills-open', 'click', () => run('shared-skills-open', OpenSharedSkillsDirectory));
 document.querySelectorAll('[data-workspace-select]').forEach((b) => b.addEventListener('click', () => run('workspace-select', async () => { workspaceState = await SelectWorkspace(b.dataset.workspaceSelect); await refreshWorkspace(); message = '已切换工作区。'; })));
 document.querySelectorAll('[data-workspace-delete]').forEach((b) => b.addEventListener('click', () => { if (window.confirm('删除这个工作区记录？不会删除项目文件。')) run('workspace-delete', async () => { workspaceState = await DeleteWorkspace(b.dataset.workspaceDelete); await refreshWorkspace(); message = '工作区已删除。'; }); }));
 document.querySelectorAll('[data-agent-doc]').forEach((b) => b.addEventListener('click', () => run('agent-read', async () => { agentPreview = { path: b.dataset.agentDoc, content: await ReadAgentDocument(b.dataset.agentDoc) }; agentDirty = false; })));
 on('agent-editor', 'input', (e) => { if (agentPreview) agentPreview.content = e.target.value; agentDirty = true; });
 on('agent-save', 'click', () => run('agent-save', async () => { await WriteAgentDocument(agentPreview.path, agentPreview.content); agentDirty = false; agentConfig = await LoadAgentConfig(); message = '指令文件已保存。'; }));
 on('agent-close', 'click', () => { if (agentDirty && !window.confirm('指令文件有未保存修改，确定关闭吗？')) return; agentPreview = null; agentDirty = false; render(); });
 on('advanced-refresh', 'click', () => run('advanced-refresh', refreshAdvanced));
 document.querySelectorAll('[data-advanced-section]').forEach((b) => b.addEventListener('click', () => { advancedSection = b.dataset.advancedSection; treeExpanded.advanced = true; navigatePage('advanced'); }));
 on('advanced-memory-open', 'click', () => run('advanced-memory-open', () => OpenAdvancedDirectory('memory')));
 on('advanced-codex-memory-open', 'click', () => run('advanced-codex-memory-open', () => OpenAdvancedDirectory('codex-memory')));
 on('advanced-codex-open', 'click', () => run('advanced-codex-open', () => OpenAdvancedDirectory('codex')));
 on('advanced-pi-open', 'click', () => run('advanced-pi-open', () => OpenAdvancedDirectory('pi')));
 on('advanced-memory-new', 'click', () => { advancedPreview = { kind: 'memory', title: '新建记忆', path: '', name: '', scope: 'global', content: '', isNew: true }; advancedDirty = false; render(); });
 document.querySelectorAll('[data-memory-read]').forEach((b) => b.addEventListener('click', () => run('memory-read', async () => { advancedPreview = { kind: 'memory', title: '编辑记忆', path: b.dataset.memoryRead, content: await ReadMemory(b.dataset.memoryRead), isNew: false }; advancedDirty = false; })));
 document.querySelectorAll('[data-memory-read]').forEach((b) => b.addEventListener('click', () => run('memory-read', async () => { const path = b.dataset.memoryRead; const fromCodex = (advancedState.codexMemories || []).some((memory) => memory.path === path); advancedPreview = { kind: 'memory', title: fromCodex ? 'Codex 记忆（只读）' : '编辑记忆', path, content: await ReadMemory(path), isNew: false, readOnly: fromCodex }; advancedDirty = false; })));
 document.querySelectorAll('[data-memory-refine]').forEach((b) => b.addEventListener('click', () => run('memory-refine', async () => { const path = b.dataset.memoryRefine; const source = (advancedState.codexMemories || []).find((memory) => memory.path === path); const content = await ReadMemory(path); const base = ((source ? source.name : 'codex-memory').split('/').pop() || 'codex-memory').replace(/\.md$/i, ''); advancedPreview = { kind: 'memory', title: '提炼到共享记忆', path: '', name: base, scope: 'project', isNew: true, content: `> 提炼自 Codex 记忆：${source ? source.title || base : base}\n\n${content}` }; advancedDirty = false; })));
 document.querySelectorAll('[data-advanced-doc]').forEach((b) => b.addEventListener('click', () => run('advanced-read', async () => { const path = b.dataset.advancedDoc; advancedPreview = { kind: 'document', title: path.endsWith('mcp.json') ? '编辑 MCP 配置' : path.endsWith('settings.json') ? '编辑 pi settings' : path.endsWith('models.json') ? '编辑 pi models' : '编辑 Codex 配置', path, content: await ReadRuntimeDocument(path), isNew: false }; advancedDirty = false; })));
 on('advanced-memory-name', 'input', (e) => { if (advancedPreview) advancedPreview.name = e.target.value; advancedDirty = true; });
 on('advanced-memory-scope', 'change', (e) => { if (advancedPreview) advancedPreview.scope = e.target.value; advancedDirty = true; });
 on('advanced-editor', 'input', (e) => { if (advancedPreview) advancedPreview.content = e.target.value; advancedDirty = true; });
 on('advanced-save', 'click', () => run('advanced-save', async () => { if (advancedPreview.kind === 'memory') { if (advancedPreview.isNew) await CreateMemory(advancedPreview.name || '', advancedPreview.content || '', advancedPreview.scope || 'global'); else await WriteMemory(advancedPreview.path, advancedPreview.content || ''); } else await WriteRuntimeDocument(advancedPreview.path, advancedPreview.content || ''); advancedDirty = false; await refreshAdvanced(); message = advancedPreview.kind === 'memory' ? '记忆已保存。' : '高级配置已保存，原文件已备份。'; }));
 on('advanced-close', 'click', () => { if (advancedDirty && !window.confirm('高级配置有未保存修改，确定关闭吗？')) return; advancedPreview = null; advancedDirty = false; render(); });
 on('app-target', 'change', (e) => run('target', async () => { marketGeneration++; state = await SetTarget(e.target.value); selectedCLI=isPi()?'pi':'codex'; skillMarket='all'; skillsState = { root: '', target: state.target, skills: [], trash: [], catalog: [], markets: skillsState.markets, errors: [] }; selectedSkills.clear(); if (page === 'skills') await refreshSkills(); if (page === 'cli') await refreshCLI(); if (page === 'advanced') await refreshAdvanced(); message = isPi() ? '已选择 pi agent。' : '已选择 ChatGPT / Codex。'; }));
 document.querySelectorAll('[data-install-skill]').forEach((b) => b.addEventListener('click', () => run('skill-install', async () => { if (b.dataset.installTarget === 'shared') { await InstallSkillTo(b.dataset.installSkill, 'shared'); message = '技能已安装到共享目录。'; } else { skillsState.skills = await InstallSkillTo(b.dataset.installSkill, 'active'); await refreshSkills(); message = '技能已安装。'; } })));
 document.querySelectorAll('[data-delete-skill]').forEach((b) => b.addEventListener('click', () => run('skill-delete', async () => { await DeleteSkill(b.dataset.deleteSkill); selectedSkills.delete(b.dataset.deleteSkill); await refreshSkills(); message = '技能已移入回收区，可恢复。'; })));
 on('check-update', 'click', () => run('update-check', async () => { updateState = await CheckForUpdate(); }));
 on('apply-update', 'click', () => { if (!window.confirm('确认下载并安装新版本？程序会自动关闭并重启。')) return; run('update-apply', async () => { updateState = await StartUpdate(); }); });
 on('lang-zh', 'click', () => { language = 'zh'; localStorage.setItem('model-switcher-language', language); render(); });
 on('lang-en', 'click', () => { language = 'en'; localStorage.setItem('model-switcher-language', language); render(); });
 on('theme-light', 'click', () => { theme = 'light'; localStorage.setItem('model-switcher-theme', theme); applyPreferences(); render(); });
 on('theme-dark', 'click', () => { theme = 'dark'; localStorage.setItem('model-switcher-theme', theme); applyPreferences(); render(); });
 on('reload', 'click', () => run('reload', async () => { state = await LoadState(); loaded = true; if (page === 'skills') await refreshSkills(); if(page==='cli')await refreshCLI(); if(page==='advanced')await refreshAdvanced(); if(page==='chat'){piChat.syncState();await piChat.refresh();} }));
 on('chatgpt-target', 'click', () => run('target', async () => { state = await ChooseChatGPTPath(); }));
 on('auto-target', 'click', () => run('target', async () => { state = await SetChatGPTPath(''); message = state.chatGptResolvedTarget ? '已自动找到 ChatGPT，当前路径已更新。' : ''; }));
 on('codex-dir', 'click', () => run('codex-dir', OpenCodexDirectory));
 on('config-view', 'click', () => run('config-view', async () => { configPreview = await ReadConfigText(); configDirty = false; }));
 on('config-editor', 'input', (e) => { configPreview = e.target.value; configDirty = true; });
 on('config-save', 'click', () => run('config-save', async () => { await WriteConfigText(configPreview || ''); state = await LoadState(); loaded = true; configDirty = false; message = configName() + ' 已保存，原文件已备份。'; }));
 on('config-close', 'click', () => { if (configDirty && !window.confirm(configName() + ' 有未保存的修改，确定关闭吗？')) return; configPreview = null; configDirty = false; render(); document.getElementById('config-view')?.focus(); });
 on('fetch', 'click', () => run('fetch', async () => {
  const previous = new Map(draft.models.map((m) => [m.id, { supportsImages: m.supportsImages, contextWindow: m.contextWindow }]));
  draft.models = normalizeDraftModels((await FetchModels(draft)).map((m) => { const old = previous.get(m.id); return { ...m, supportsImages: old ? Boolean(old.supportsImages) : Boolean(m.supportsImages), contextWindow: old?.contextWindow || m.contextWindow }; }));
  if (!draft.models.some((m) => m.id === draft.selectedModel)) draft.selectedModel = draft.models[0]?.id || '';
  dirty = true; message = '发现 ' + draft.models.length + ' 个模型，已自动填入。';
 }));
 on('test-model', 'click', () => run('test-model', async () => {
  const result = await TestModel(draft, state.target, modelTestInput);
  modelTestReply = result.reply || '';
  modelTestModel = result.model || draft.selectedModel;
  modelTestProtocol = result.protocol || '';
  message = language === 'en' ? 'Model replied.' : '模型已回复。';
 }));
 on('save', 'click', () => run('save', async () => { await save(); message = '配置已保存。'; }));
 on('restore', 'click', () => run('restore', async () => { state = await ActivateOpenAI(); message = isPi() ? '已恢复 pi 切换前的配置。' : '已恢复切换前的 OpenAI 设置。'; }));
 on('delete', 'click', () => { if (window.confirm('删除配置「' + draft.name + '」？其保存的 Key 也会移除。')) run('delete', async () => { state = await DeleteProfile(draft.id); edit(); message = '配置已删除。'; }); });
}
async function save() {
 if (!draft.name.trim()) throw new Error('请输入配置名称。');
 if (!draft.baseUrl.trim()) throw new Error('请输入 API 地址。');
 const id = draft.id;
 state = await SaveProfile(draft);
 const profile = state.profiles.find((p) => p.id === id) || state.profiles[state.profiles.length - 1];
 draft = { ...profile, apiKey: profile?.apiKey || '', clearApiKey: false, models: profile.models || [] }; keyVisible = false; dirty = false;
}
function withTimeout(promise, milliseconds, message) {
 let timer;
 return Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(message)), milliseconds); })]).finally(() => clearTimeout(timer));
}
async function run(action, fn) {
 if (busy) return;
 restoreFocusID = document.activeElement?.id || '';
 busy = action; error = ''; message = ''; render();
 try { await fn(); } catch (e) { error = e?.message || String(e); if (!['load', 'profile-copy', 'profile-paste'].includes(action)) { try { state = await withTimeout(LoadState(), 8000, '刷新当前状态超时'); loaded = true; } catch {} } }
 finally { busy = ''; render(); }
}
document.addEventListener('keydown', (e) => {
const modal = document.querySelector('.settings-modal, .config-modal');
 if (!modal) return;
 if (e.key === 'Escape') { e.preventDefault(); document.getElementById(settingsOpen ? 'settings-close' : agentPreview !== null ? 'agent-close' : advancedPreview !== null ? 'advanced-close' : 'config-close')?.click(); return; }
 if (e.key !== 'Tab') return;
 const controls = [...modal.querySelectorAll('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea, summary')];
 if (!controls.length) return;
 const first = controls[0], last = controls[controls.length - 1];
 if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
 else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
});
window.addEventListener('resize', () => applySidebarWidth());
async function initialLoad() {
 render();
 void environment.refresh();
 try {
  state = await withTimeout(LoadState(), 8000, '读取当前配置超时，请点击刷新重试');
  loaded = true;
  error = state.loadError || '';
  if (state.profiles.length) edit(state.profiles[0]); else render();
  try {
   workspaceState = await withTimeout(LoadWorkspaceState(), 8000, '读取工作区超时');
   workspaceState.projects ||= [];
   render();
  } catch {}
  try {
   advancedState = await withTimeout(LoadAdvancedState(), 8000, '读取高级设置超时');
   if (page === 'advanced') render();
  } catch {}
 } catch (e) {
  error = e?.message || String(e);
  render();
 }
}
render();
initialLoad();
