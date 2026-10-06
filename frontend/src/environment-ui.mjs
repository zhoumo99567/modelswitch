export function createEnvironmentManager({ detect, startInstall, getInstallState, redraw, getLanguage, icon, esc, onInstalled }) {
 let state = { tools: [], platform: '', architecture: '', installDirectory: '' };
 let detecting = false;
 let detectionError = '';
 let job = { status: '', message: '', progress: 0, error: '' };
 let polling = false;
 const en = () => getLanguage() === 'en';
 const text = (zh, english) => en() ? english : zh;
 const installing = () => job.status === 'running';

 async function refresh() {
  if (detecting || installing()) return;
  detecting = true; detectionError = ''; redraw();
  try { state = await detect(); }
  catch (error) { detectionError = error?.message || String(error); }
  finally { detecting = false; redraw(); }
 }

 async function poll() {
  if (polling) return;
  polling = true;
  try {
   while (installing()) {
    await new Promise((resolve) => setTimeout(resolve, 800));
    try { job = await getInstallState(); detectionError = ''; }
    catch (error) {
     // A transient transport failure does not imply the background install stopped.
     detectionError = text('暂时无法读取安装进度，正在重连…', 'Reconnecting to installation progress…');
     redraw(); continue;
    }
    redraw();
   }
   await refresh();
   await onInstalled();
  } finally { polling = false; }
 }

 async function install(id) {
  if (installing() || detecting) return;
  job = { id, status: 'running', message: text('正在准备安装…', 'Preparing installation…'), progress: 0, error: '' };
  redraw();
  try { job = await startInstall(id); }
  catch (error) { job = { id, status: 'error', message: text('无法开始安装', 'Unable to start installation'), progress: 0, error: error?.message || String(error) }; redraw(); return; }
  try { await poll(); }
  catch (error) { detectionError = text('安装结束，但刷新状态失败，请重新检测。', 'Installation ended, but refresh failed. Detect again.'); redraw(); }
 }

 function render(disabled = '') {
  const blocked = disabled || detecting || installing();
  const cards = state.tools.map((tool) => {
   const label = tool.ready ? text('可用', 'Ready') : tool.installed ? text('需要修复', 'Needs repair') : text('未安装', 'Missing');
   const action = tool.installed ? text('修复 / 安装', 'Repair / install') : text('安装', 'Install');
   return `<article class="dependency-card" id="dependency-${esc(tool.id)}"><div class="dependency-card-heading"><strong>${esc(tool.name)}</strong><span class="cli-detected ${tool.ready ? 'found' : ''}">${label}</span></div><span class="dependency-version">${esc(tool.version || text('未检测到版本', 'Version unavailable'))}${tool.managed ? ` · ${text('由本软件安装', 'Installed by this app')}` : ''}</span>${tool.path ? `<code title="${esc(tool.path)}">${esc(tool.path)}</code>` : ''}${tool.error ? `<p class="cli-error">${esc(tool.error)}</p>` : ''}${tool.ready ? '' : `<button class="button button-secondary" data-install-dependency="${esc(tool.id)}" ${blocked ? 'disabled' : ''}>${icon('download', 15)}${job.id === tool.id && installing() ? text('正在安装…', 'Installing…') : action}</button>`}</article>`;
  }).join('');
  const progress = job.status ? `<div class="dependency-progress ${job.status === 'error' ? 'is-error' : ''}" role="status" aria-live="polite"><div><strong>${text('安装状态', 'Installation status')}</strong><span>${esc(job.message)}</span></div>${installing() ? `<progress max="100" value="${Number(job.progress) || 0}" aria-label="${text('安装进度', 'Installation progress')}"></progress>` : ''}${job.error ? `<p>${esc(job.error)}</p><button class="button button-secondary" data-install-dependency="${esc(job.id)}" ${blocked ? 'disabled' : ''}>${text('重试', 'Retry')}</button>` : ''}</div>` : '';
  return `<section class="panel environment-panel" aria-label="${text('环境检测与安装', 'Environment detection and installation')}"><div class="environment-heading"><div><h3>${text('环境检测与安装', 'Environment setup')}</h3><p>${text('启动时自动检测；点击安装后按当前系统补齐依赖。', 'Checked at startup. Click Install to set up dependencies for this system.')}</p></div><button class="button button-secondary" id="environment-detect" ${blocked ? 'disabled' : ''}>${icon('refresh', 15)}${detecting ? text('正在检测…', 'Detecting…') : text('检测环境', 'Detect environment')}</button></div><p class="environment-note">${text('Windows 优先 winget，macOS 优先 Homebrew；不可用时使用官方用户目录安装包。安装 pi / Codex 会先补齐 Node.js 和 npm。', 'Windows uses winget and macOS uses Homebrew when available, with an official per-user download as fallback. Node.js and npm are set up before pi / Codex.')}</p><div class="dependency-grid" aria-busy="${detecting}">${cards || `<p>${text('正在检测当前环境…', 'Detecting your environment…')}</p>`}</div>${state.installDirectory ? `<p class="environment-location">${esc(state.platform)} / ${esc(state.architecture)}${state.nodeInstallMethod ? ` · Node.js: ${esc(state.nodeInstallMethod)}` : ''} · ${text('安装目录', 'Install folder')}<code>${esc(state.installDirectory)}</code></p>` : ''}${detectionError ? `<p class="cli-error" role="alert">${esc(detectionError)}</p>` : ''}${progress}</section>`;
 }

 function notice(target) {
  if (!state.tools.length) return '';
  const missing = state.tools.filter((tool) => ['node', 'npm', target === 'pi' ? 'pi' : 'codex'].includes(tool.id) && !tool.ready);
  if (!missing.length) return '';
  return `<div class="environment-notice"><span>${text('环境尚未就绪', 'Environment needs setup')}：${missing.map((tool) => esc(tool.name)).join('、')}</span><button class="text-button" id="open-environment">${text('检测与安装', 'Detect and install')}${icon('chevronRight', 14)}</button></div>`;
 }

 function bind(openEnvironment) {
  document.getElementById('environment-detect')?.addEventListener('click', refresh);
  document.getElementById('open-environment')?.addEventListener('click', openEnvironment);
  document.querySelectorAll('[data-install-dependency]').forEach((button) => button.addEventListener('click', () => install(button.dataset.installDependency)));
 }

 return { refresh, render, notice, bind, install, getState: () => state, isInstalling: installing };
}
