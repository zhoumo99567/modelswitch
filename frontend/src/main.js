import './style.css';
import './app.css';
import { ActivateOpenAI, ActivateProfile, ChooseChatGPTPath, DeleteProfile, FetchModels, LoadState, LoadSkillsState, InstallSkill, DeleteSkill, CheckForUpdate, StartUpdate, OpenCodexDirectory, ReadConfigText, SaveProfile, SetChatGPTPath, WriteConfigText } from '../wailsjs/go/main/App';

const paths = {
 spark: '<path d="m12 3-1.3 5.7L5 10l5.7 1.3L12 17l1.3-5.7L19 10l-5.7-1.3L12 3Z"/><path d="m5 16-.5 2.5L2 19l2.5.5L5 22l.5-2.5L8 19l-2.5-.5L5 16Z"/>',
 server: '<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01M11 7h8M11 17h8"/>',
 refresh: '<path d="M20 11A8 8 0 0 0 6 6L3 9M3 3v6h6M4 13a8 8 0 0 0 14 5l3-3M21 21v-6h-6"/>',
 check: '<path d="m5 12 4 4L19 6"/>',
 cloud: '<path d="M17.5 19H9a6 6 0 1 1 5.7-8H16a4 4 0 1 1 1.5 8Z"/>',
 play: '<path d="m8 5 11 7-11 7V5Z"/>',
 folder: '<path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v9H3V7Z"/>',
 plus: '<path d="M12 5v14M5 12h14"/>',
 trash: '<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>',
 settings: '<path d="M12 15.2a3.2 3.2 0 1 0 0-6.4 3.2 3.2 0 0 0 0 6.4Z"/><path d="m19.4 15 .1.1a1.8 1.8 0 1 1-2.5 2.5l-.1-.1a1.8 1.8 0 0 0-3.1 1.3v.2a1.8 1.8 0 1 1-3.6 0v-.2a1.8 1.8 0 0 0-3.1-1.3l-.1.1a1.8 1.8 0 1 1-2.5-2.5l.1-.1a1.8 1.8 0 0 0-1.3-3.1h-.2a1.8 1.8 0 1 1 0-3.6h.2a1.8 1.8 0 0 0 1.3-3.1l-.1-.1a1.8 1.8 0 1 1 2.5-2.5l.1.1a1.8 1.8 0 0 0 3.1-1.3v-.2a1.8 1.8 0 1 1 3.6 0v.2a1.8 1.8 0 0 0 3.1 1.3l.1-.1a1.8 1.8 0 1 1 2.5 2.5l-.1.1a1.8 1.8 0 0 0 1.3 3.1h.2a1.8 1.8 0 1 1 0 3.6h-.2a1.8 1.8 0 0 0-1.3 3.1Z"/>',
 close: '<path d="m6 6 12 12M18 6 6 18"/>',
 save: '<path d="M5 4h11l3 3v13H5V4Z"/><path d="M8 4v6h8V4M8 20v-6h8v6"/>',
 search: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/>',
 edit: '<path d="m4 16-.8 4.8L8 20l10.8-10.8a2.8 2.8 0 0 0-4-4L4 16Z"/><path d="m13.5 6.5 4 4"/>',
 download: '<path d="M12 3v12M7 10l5 5 5-5"/><path d="M5 21h14"/>',
 shield: '<path d="M12 3 20 6v5c0 5-3.4 8.5-8 10-4.6-1.5-8-5-8-10V6l8-3Z"/><path d="m9 12 2 2 4-4"/>',
};
const icon = (name, size = 18) => '<svg width="' + size + '" height="' + size + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + (paths[name] || '') + '</svg>';
const esc = (value = '') => String(value).replace(/[&<>'"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#039;', '"': '&quot;' }[c]));
const emptyDraft = () => ({ id: '', name: '', baseUrl: '', apiKey: '', selectedModel: '', models: [], clearApiKey: false, hasApiKey: false });
let state = { version: '0.1.0', profiles: [], activeProvider: 'openai', activeModel: '', activeProfileId: '', configPath: '', canRestore: false, chatGptRunning: false, chatGptTarget: '' };
let draft = emptyDraft();
let busy = '';
let error = '';
let message = '';
let dirty = false;
let loaded = false;
let configPreview = null;
let configDirty = false;
let settingsOpen = false;
let skillsState = { root: '', skills: [], catalog: [] };
let updateState = { status: 'unconfigured', message: '更新源未配置，已跳过检查。', currentVersion: '0.1.0', updateAvailable: false };
let language = localStorage.getItem('model-switcher-language') || 'zh';
let theme = localStorage.getItem('model-switcher-theme') || 'light';
const app = document.querySelector('#app');

const zhToEn = {
 '让模型切换更简单': 'Switch models simply', '工作空间': 'Workspace', '添加本地配置': 'Add local profile',
 '只属于你的配置': 'Your profiles', 'API Key 使用系统安全存储。': 'API keys use secure system storage.',
 'Codex 配置': 'Codex config', 'OpenAI 云端': 'OpenAI Cloud', '模型切换': 'Model Switcher', '模型配置': 'Model Profiles',
 '一个地方，管理你的本地模型连接。': 'Manage local model connections in one place.', 'ChatGPT 运行中': 'ChatGPT running',
 'ChatGPT 未运行': 'ChatGPT not running', '服务连接': 'Service connection', '编辑配置': 'Edit profile', '新建配置': 'New profile',
 '配置名称': 'Profile name', 'API 地址': 'API address', '支持本机、局域网和远程 OpenAI 兼容服务。': 'Local, LAN, and remote OpenAI-compatible services are supported.',
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
 '更新包已校验，程序即将重启。': 'Update verified; the app will restart.', '当前版本': 'Current version', '发现新版本': 'New version available', '更新': 'Update',
};
function translateText(text) {
 const trimmed = text.trim();
 if (zhToEn[trimmed]) return text.replace(trimmed, zhToEn[trimmed]);
 const count = trimmed.match(/^(\d+) 个可用$/);
 if (count) return text.replace(trimmed, count[1] + ' available');
 const found = trimmed.match(/^发现 (\d+) 个模型，已自动填入。$/);
 if (found) return text.replace(trimmed, found[1] + ' models found and filled in.');
 const switched = trimmed.match(/^已切换到「(.+)」，ChatGPT 正在重启。$/);
 if (switched) return text.replace(trimmed, 'Switched to “' + switched[1] + '”; ChatGPT is restarting.');
 const target = trimmed.match(/^切换到 (.+)$/);
 if (target) return text.replace(trimmed, 'Switch to ' + target[1]);
 return text;
}
function localizeDOM() {
 if (language !== 'en') return;
 const walk = (node) => {
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
function edit(profile) { draft = profile ? { ...profile, apiKey: '', clearApiKey: false, models: profile.models || [] } : emptyDraft(); dirty = false; error = ''; message = ''; render(); }
function currentIsLocal() { return state.activeProvider === 'model_switcher_local'; }
function formatBytes(bytes) {
 if (!bytes) return '0 B';
 const units = ['B', 'KB', 'MB', 'GB']; let value = bytes; let index = 0;
 while (value >= 1024 && index < units.length - 1) { value /= 1024; index += 1; }
 return value.toFixed(value >= 10 || index === 0 ? 0 : 1) + ' ' + units[index];
}
function renderSkillRows() {
 const local = skillsState.skills.length ? skillsState.skills.map((skill) => '<div class="skill-item"><div class="skill-item-main"><div class="skill-item-title"><strong>' + esc(skill.name) + '</strong>' + (skill.system ? '<span class="skill-badge protected">受保护</span>' : '') + (skill.hasScripts ? '<span class="skill-badge warning">包含脚本，请先审阅</span>' : '') + '</div><small>' + esc(skill.description) + '</small><code>' + formatBytes(skill.sizeBytes) + ' · ' + skill.fileCount + ' 个文件</code></div><button class="icon-button skill-delete" data-delete-skill="' + esc(skill.name) + '" title="删除技能" ' + (skill.system ? 'disabled' : '') + '>' + icon('trash', 15) + '</button></div>').join('') : '<div class="skill-empty">暂无技能</div>';
 const catalog = skillsState.catalog.length ? skillsState.catalog.map((item) => '<div class="skill-item catalog-item"><div class="skill-item-main"><div class="skill-item-title"><strong>' + esc(item.name) + '</strong><span class="skill-category">' + esc(item.category || 'Codex') + '</span></div><small>' + esc(item.source) + '</small></div><button class="button button-secondary skill-install" data-install-skill="' + esc(item.id) + '">' + icon(item.installed ? 'check' : 'download', 14) + (item.installed ? '已安装' : '安装') + '</button></div>').join('') : '<div class="skill-empty">无法加载官方目录</div>';
 return '<div class="skills-columns"><section class="skills-list"><div class="skills-list-head"><strong>已安装</strong><span>' + skillsState.skills.length + '</span></div><div class="skills-scroll">' + local + '</div></section><section class="skills-list"><div class="skills-list-head"><strong>官方目录</strong><span>' + skillsState.catalog.length + '</span></div><div class="skills-scroll">' + catalog + '</div></section></div>';
}
function renderUpdateManager() {
 const available = updateState.updateAvailable;
 const status = updateState.status === 'error' ? 'is-error' : available ? 'is-available' : '';
 return '<section class="update-manager"><div class="update-manager-head"><div><strong>更新管理</strong><small>检查新版本并在下载校验后自动重启。</small></div><button id="check-update" class="text-button">' + icon('refresh', 14) + '检查更新</button></div><div class="update-status ' + status + '"><span>当前版本 v' + esc(updateState.currentVersion || '0.1.0') + '</span><span>' + esc(updateState.message || '尚未检查') + '</span></div>' + (available ? '<button id="apply-update" class="button button-primary update-apply">' + icon('download', 15) + '更新到 v' + esc(updateState.latestVersion) + '</button>' : '') + '</section>';
}

function render() {
 const active = currentProfile();
 const local = currentIsLocal();
 const title = local ? (active?.name || '本地模型') : 'OpenAI 云端';
 const disabled = busy || !loaded ? 'disabled' : '';
 const modelRows = draft.models.length ? draft.models.slice(0, 6).map((m, i) => '<div class="model-row ' + (m.id === draft.selectedModel ? 'chosen' : '') + '"><span class="model-number">0' + (i + 1) + '</span><span>' + esc(m.id) + '</span>' + (m.id === draft.selectedModel ? icon('check', 15) : '') + '</div>').join('') : '<div class="model-empty">' + icon('server', 34) + '<strong>先连接你的本地服务</strong><p>填写 API 地址，然后点击「获取模型」。</p></div>';
 const profileRows = state.profiles.length ? state.profiles.map((p) => '<div class="profile-item-row"><button class="profile-item ' + (p.id === draft.id ? 'selected' : '') + '" data-profile="' + esc(p.id) + '" ' + disabled + '><span class="profile-dot ' + (p.id === state.activeProfileId ? 'live' : '') + '"></span><span class="profile-copy"><strong>' + esc(p.name) + '</strong><small>' + esc(p.selectedModel || '尚未选择模型') + '</small></span>' + (p.id === state.activeProfileId ? '<span class="active-mini">已应用</span>' : '') + '</button><button class="profile-switch" data-activate="' + esc(p.id) + '" aria-label="切换到 ' + esc(p.name) + '" ' + disabled + (p.id === state.activeProfileId ? ' disabled' : '') + '>' + (p.id === state.activeProfileId ? icon('check', 13) + '当前' : icon('play', 13) + '切换') + '</button></div>').join('') : '<div class="empty-sidebar">你的模型，从这里开始。<br><span>添加第一个本地服务配置。</span></div>';
 app.innerHTML = `
 <div class="shell">
  <aside class="sidebar">
   <div class="brand"><div class="brand-mark">${icon('spark', 22)}</div><div><div class="brand-name">Model Switcher</div><div class="brand-caption">让模型切换更简单</div></div></div>
   <div class="sidebar-label">工作空间 <span class="sidebar-count">${state.profiles.length}</span></div>
   <div class="profile-list">${profileRows}</div>
   <button class="new-profile" id="new" ${disabled}>${icon('plus', 16)} 添加本地配置</button>
   <div class="sidebar-bottom">
    <button class="connection-card ${local ? 'local' : ''}" id="settings" ${disabled} title="打开设置"><div class="connection-icon">${icon(local ? 'server' : 'cloud', 17)}</div><div><span>Codex 配置</span><strong>${esc(title)}</strong></div></button>
    <div class="sidebar-foot">DESKTOP · PORTABLE EDITION <span>v${esc(state.version || '0.1.0')}</span></div>
   </div>
  </aside>
  <main class="content">
   <header class="topbar"><div><div class="eyebrow">YOUR MODELS. YOUR CHOICE.</div><h1>模型配置<span class="title-dot">.</span></h1><p>一个地方，管理你的本地模型连接。</p></div><div class="top-actions"><span class="app-status"><span class="status-dot"></span>${state.chatGptRunning ? 'ChatGPT 运行中' : 'ChatGPT 未运行'}</span><button class="icon-button" id="reload" title="刷新当前状态" ${disabled}>${icon('refresh', 18)}</button></div></header>
   <section class="hero-card ${local ? 'hero-local' : ''}"><div class="hero-orb"></div><div class="hero-content"><div class="hero-kicker">${icon(local ? 'server' : 'cloud', 15)} CURRENT PROVIDER</div><h2>${esc(title)}</h2><p>${esc(state.activeModel || '使用官方默认模型设置')}</p></div><div class="hero-chip"><span class="pulse"></span>配置状态 · ${local ? '本地' : 'OpenAI'}</div></section>
   <div class="feedback feedback-top ${error ? 'is-error' : ''}" role="status">${esc(error || message || (dirty ? '有未保存的修改。' : '配置保存在此电脑，切换前自动备份 Codex 配置。'))}</div>
   <div class="workspace-grid">
    <section class="panel config-panel"><div class="panel-heading"><div><span class="panel-index">01</span><h3>服务连接</h3></div><span class="muted-label">${draft.id ? '编辑配置' : '新建配置'}</span></div><div class="form-grid">
     <label class="field field-wide"><span>配置名称</span><input id="name" placeholder="例如：我的 LM Studio" value="${esc(draft.name)}" maxlength="80" ${disabled}></label>
     <label class="field field-wide"><span>API 地址</span><input id="url" type="url" placeholder="http://127.0.0.1:1234/v1" value="${esc(draft.baseUrl)}" ${disabled}><small>支持本机、局域网和远程 OpenAI 兼容服务。</small></label>
     <label class="field field-wide"><span>API Key <em>可选</em></span><input id="key" type="password" autocomplete="off" placeholder="${draft.hasApiKey ? 'Key 已加密保存；留空保持原值' : '服务无需认证时可留空'}" ${disabled}></label>
     ${draft.hasApiKey ? `<label class="clear-key"><input id="clear-key" type="checkbox" ${draft.clearApiKey ? 'checked' : ''} ${disabled}> 移除已保存的 Key</label>` : ''}
    </div><div class="form-actions"><button class="button button-secondary" id="save" ${disabled}>${icon('save', 16)}${busy === 'save' ? '正在保存…' : '保存'}</button><button class="button button-secondary" id="fetch" ${disabled}>${icon('refresh', 17)}${busy === 'fetch' ? '正在获取…' : '获取模型'}</button><span class="endpoint">GET /models</span></div></section>
    <section class="panel model-panel"><div class="panel-heading"><div><span class="panel-index">02</span><h3>选择模型</h3></div><span class="model-count">${draft.models.length} 个可用</span></div><label class="field"><span>使用的模型</span><select id="model" ${disabled}${draft.models.length ? '' : ' disabled'}><option value="">${draft.models.length ? '选择一个模型' : '等待获取模型列表'}</option>${draft.models.map((m) => `<option value="${esc(m.id)}" ${m.id === draft.selectedModel ? 'selected' : ''}>${esc(m.id)}</option>`).join('')}</select></label><div class="model-list">${modelRows}</div><div class="model-hint">接入 Codex 需要兼容 <strong>/v1/responses</strong></div></section>
   </div>
   <section class="chatgpt-path-panel"><div class="chatgpt-path-inline"><label id="chatgpt-path-label">ChatGPT 路径 <span class="target-mode">${state.chatGptTarget ? '手动选择' : '自动查找'}</span></label><button type="button" id="chatgpt-target" class="target-picker" aria-labelledby="chatgpt-path-label" title="${esc(state.chatGptResolvedTarget || state.chatGptTarget || '点击选择 ChatGPT 应用')}" ${disabled}>${icon('folder', 16)}<span>${esc(state.chatGptResolvedTarget || state.chatGptTarget || (loaded ? '未找到 ChatGPT，点击选择应用' : '正在查找 ChatGPT…'))}</span></button><button class="text-button" id="auto-target" ${disabled}>${icon('search', 15)}自动查找</button></div>${state.chatGptTargetError ? `<small class="chatgpt-path-error">${esc(state.chatGptTargetError)}</small>` : ''}</section>
   <footer class="footnote"><button id="folder" class="text-button" ${disabled}>${icon('folder', 15)} 打开 Codex 配置</button><div class="footer-right">${draft.id ? `<button class="text-button delete-button" id="delete" ${disabled}>${icon('trash', 15)}删除配置</button>` : ''}<button id="restore" class="text-button restore-button" ${disabled}${state.canRestore ? '' : ' disabled'}>${icon('cloud', 16)}${busy === 'restore' ? '正在恢复…' : '切回 OpenAI'}</button></div></footer>
  </main>
 </div>`;
 const folder = document.getElementById('folder');
 if (folder) {
  folder.outerHTML = '<button id="codex-dir" class="text-button" ' + disabled + '>' + icon('folder', 15) + ' 打开 .codex 目录</button><button id="config-view" class="text-button" ' + disabled + '>' + icon('edit', 15) + ' 查看 config.toml</button>';
 }
 const restore = document.getElementById('restore');
 const sidebar = document.querySelector('.sidebar');
 const sidebarLabel = document.querySelector('.sidebar-label');
 if (restore && sidebar) {
  restore.classList.add('sidebar-restore');
  restore.innerHTML = icon('cloud', 15) + ' 切回 OpenAI';
  sidebar.insertBefore(restore, sidebarLabel || sidebar.firstChild);
 }
 const footerRight = document.querySelector('.footer-right');
 if (footerRight && footerRight.children.length === 0) footerRight.remove();
 if (configPreview !== null) {
  app.insertAdjacentHTML('beforeend', '<div class="config-modal" role="dialog" aria-modal="true"><div class="config-modal-card"><div class="config-modal-head"><div><span class="panel-index">CODEX CONFIG</span><h3>编辑 config.toml</h3></div><button class="icon-button" id="config-close" aria-label="关闭配置编辑">' + icon('close', 18) + '</button></div><textarea id="config-editor" class="config-preview" spellcheck="false">' + esc(configPreview) + '</textarea><div class="config-modal-actions"><span>保存前会校验 TOML，并自动备份原文件。</span><button class="button button-secondary" id="config-save">' + icon('save', 16) + '保存 config.toml</button></div></div></div>');
 }
 if (settingsOpen) {
   app.insertAdjacentHTML('beforeend', '<div class="settings-modal" role="dialog" aria-modal="true"><div class="settings-card"><div class="settings-head"><div><h3>设置</h3></div><button class="icon-button" id="settings-close" aria-label="关闭设置">' + icon('close', 18) + '</button></div><div class="settings-row"><div><strong>语言</strong><small>界面语言和外观会保存在本机。</small></div><div class="settings-segment"><button id="lang-zh" class="' + (language === 'zh' ? 'active' : '') + '">中文</button><button id="lang-en" class="' + (language === 'en' ? 'active' : '') + '">English</button></div></div><div class="settings-row"><div><strong>主题</strong><small>选择适合当前环境的显示模式。</small></div><div class="settings-segment"><button id="theme-light" class="' + (theme === 'light' ? 'active' : '') + '">浅色</button><button id="theme-dark" class="' + (theme === 'dark' ? 'active' : '') + '">深色</button></div></div><section class="skills-manager"><div class="skills-manager-head"><div><strong>技能管理</strong><small>管理本机 .codex/skills 中的技能，也可从官方目录安装。</small></div><button id="skills-refresh" class="text-button">' + icon('refresh', 14) + '刷新技能</button></div><div class="skills-root"><span>目录</span><code>' + esc(skillsState.root || '正在读取…') + '</code></div>' + renderSkillRows() + '</section>' + renderUpdateManager() + '</div></div>');
 }
 applyPreferences();
 localizeDOM();
 bind();
}

function bind() {
 const on = (id, event, fn) => document.getElementById(id)?.addEventListener(event, fn);
 on('name', 'input', (e) => { draft.name = e.target.value; dirty = true; });
 on('url', 'input', (e) => { draft.baseUrl = e.target.value; dirty = true; });
 on('key', 'input', (e) => { draft.apiKey = e.target.value; dirty = true; });
 on('clear-key', 'change', (e) => { draft.clearApiKey = e.target.checked; dirty = true; });
 on('model', 'change', (e) => { draft.selectedModel = e.target.value; dirty = true; render(); });
 document.querySelectorAll('[data-profile]').forEach((b) => b.addEventListener('click', () => { edit(profileFor(b.dataset.profile)); }));
 document.querySelectorAll('[data-activate]').forEach((b) => b.addEventListener('click', () => run('activate', async () => { const id = b.dataset.activate; state = await ActivateProfile(id); const profile = state.profiles.find((p) => p.id === id); if (profile) draft = { ...profile, apiKey: '', clearApiKey: false, models: profile.models || [] }; dirty = false; message = '已切换到「' + (profile?.name || '本地配置') + '」，ChatGPT 正在重启。'; })));
 on('new', 'click', () => edit());
 on('settings', 'click', () => { settingsOpen = true; render(); run('settings-load', async () => { const result = await Promise.all([LoadSkillsState(), CheckForUpdate()]); skillsState = result[0]; updateState = result[1]; }); });
 on('settings-close', 'click', () => { settingsOpen = false; render(); });
 on('skills-refresh', 'click', () => run('skills', async () => { skillsState = await LoadSkillsState(); }));
 document.querySelectorAll('[data-install-skill]').forEach((b) => b.addEventListener('click', () => run('skill-install', async () => { skillsState.skills = await InstallSkill(b.dataset.installSkill); skillsState = await LoadSkillsState(); message = '技能已安装。'; })));
 document.querySelectorAll('[data-delete-skill]').forEach((b) => b.addEventListener('click', () => { if (b.disabled || !window.confirm('删除技能「' + b.dataset.deleteSkill + '」？技能会先移入 .trash 回收区。')) return; run('skill-delete', async () => { skillsState.skills = await DeleteSkill(b.dataset.deleteSkill); message = '技能已移入回收区。'; }); }));
 on('check-update', 'click', () => run('update-check', async () => { updateState = await CheckForUpdate(); }));
 on('apply-update', 'click', () => { if (!window.confirm('确认下载并安装新版本？程序会自动关闭并重启。')) return; run('update-apply', async () => { updateState = await StartUpdate(); }); });
 on('lang-zh', 'click', () => { language = 'zh'; localStorage.setItem('model-switcher-language', language); render(); });
 on('lang-en', 'click', () => { language = 'en'; localStorage.setItem('model-switcher-language', language); render(); });
 on('theme-light', 'click', () => { theme = 'light'; localStorage.setItem('model-switcher-theme', theme); applyPreferences(); render(); });
 on('theme-dark', 'click', () => { theme = 'dark'; localStorage.setItem('model-switcher-theme', theme); applyPreferences(); render(); });
 on('reload', 'click', () => run('reload', async () => { state = await LoadState(); loaded = true; }));
 on('chatgpt-target', 'click', () => run('target', async () => { state = await ChooseChatGPTPath(); }));
 on('auto-target', 'click', () => run('target', async () => { state = await SetChatGPTPath(''); message = state.chatGptResolvedTarget ? '已自动找到 ChatGPT，当前路径已更新。' : ''; }));
 on('codex-dir', 'click', () => run('codex-dir', OpenCodexDirectory));
 on('config-view', 'click', () => run('config-view', async () => { configPreview = await ReadConfigText(); configDirty = false; }));
 on('config-editor', 'input', (e) => { configPreview = e.target.value; configDirty = true; });
 on('config-save', 'click', () => run('config-save', async () => { await WriteConfigText(configPreview || ''); state = await LoadState(); loaded = true; configDirty = false; message = 'config.toml 已保存，原文件已备份。'; }));
 on('config-close', 'click', () => { if (configDirty && !window.confirm('config.toml 有未保存的修改，确定关闭吗？')) return; configPreview = null; configDirty = false; render(); });
 on('fetch', 'click', () => run('fetch', async () => {
  draft.models = await FetchModels(draft);
  if (!draft.models.some((m) => m.id === draft.selectedModel)) draft.selectedModel = draft.models[0]?.id || '';
  dirty = true; message = '发现 ' + draft.models.length + ' 个模型，已自动填入。';
 }));
 on('save', 'click', () => run('save', async () => { await save(); message = '配置已保存。'; }));
 on('restore', 'click', () => run('restore', async () => { state = await ActivateOpenAI(); message = '已恢复切换前的 OpenAI 设置。'; }));
 on('delete', 'click', () => { if (window.confirm('删除配置「' + draft.name + '」？其保存的 Key 也会移除。')) run('delete', async () => { state = await DeleteProfile(draft.id); edit(); message = '配置已删除。'; }); });
}
async function save() {
 if (!draft.name.trim()) throw new Error('请输入配置名称。');
 if (!draft.baseUrl.trim()) throw new Error('请输入 API 地址。');
 const id = draft.id;
 state = await SaveProfile(draft);
 const profile = state.profiles.find((p) => p.id === id) || state.profiles[state.profiles.length - 1];
 draft = { ...profile, apiKey: '', clearApiKey: false, models: profile.models || [] }; dirty = false;
}
async function run(action, fn) {
 if (busy) return;
 busy = action; error = ''; message = ''; render();
 try { await fn(); } catch (e) { error = e?.message || String(e); try { state = await LoadState(); loaded = true; } catch {} }
 finally { busy = ''; render(); }
}
render();
run('load', async () => { state = await LoadState(); loaded = true; if (state.profiles.length) edit(state.profiles[0]); else render(); });

