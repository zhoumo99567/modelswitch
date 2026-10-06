import './pi-chat.css';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { LoadPiChatConfig, OpenPiChatRuntime, PiChatRuntimeCommand, ClosePiChatRuntime, ReadPiChatClipboardFiles } from '../wailsjs/go/main/App';
import { attachmentError, attachmentSize, readChatAttachment, validateAttachmentBatch } from './chat-attachments.mjs';
import { ChatSession, createChatEntry, MAX_QUEUED_MESSAGES } from './chat-session.mjs';
import { PiRuntimeAgent, requirePiRuntimeBindings } from './chat-runtime.mjs';

export function createPiChat({ redraw, getState, getLanguage, icon, esc, markdown }) {
 let config = null, messages = [], input = '', loading = false, sending = false;
 let issue = '', note = '', signature = '', generation = 0, frame = 0, followBottom = true, scrollTop = 0;
 let attachments = [], reading = 0, attachmentGeneration = 0, attachmentQueue = Promise.resolve(), filePasteVersion = 0, nativePasteTimer;
 let queueMenuId = '', queueEditId = '', queueEditText = '', resumeAfterEdit = false;
 let runtimeInfo = null, runtimeDialogs = [], showResources = false;
 const runtimeStatuses = new Map(), runtimeWidgets = new Map();
 const text = (zh, en) => getLanguage() === 'en' ? en : zh;
 const canSend = () => config && !loading && !reading && (input.trim() || attachments.length);
 const session = new ChatSession({
  text,
  getAgent: async (existing) => {
   const current = generation;
   const next = await LoadPiChatConfig();
   if (current !== generation) return null;
   if (config && next.id !== config.id) {
    reset(text('配置已变化，待发送消息已暂停，请确认后继续。', 'Configuration changed. Pending messages are paused. Review the model before continuing.'));
    config = next; redraw(); return null;
   }
   config = next;
   if (session.queue.some((entry) => entry.attachments.some((file) => file.kind === 'image')) && !config.model.input?.includes('image')) throw new Error(imageInputIssue());
   return existing && !existing.state.closed ? existing : await createAgent();
  },
  onRestore: (entry) => {
   if (input.trim() || attachments.length || reading) return false;
   input = entry.prompt; attachments = entry.attachments.slice(); return true;
  },
  onChange: (kind) => {
   messages = session.messages; sending = session.running;
   if (session.error) resumeAfterEdit = false;
   if (kind === 'stream') schedulePaint();
   else {
    if (kind === 'turn') followBottom = true;
    if (!session.queue.some((entry) => entry.id === queueMenuId)) queueMenuId = '';
    redraw(); paint();
   }
  },
 });
 const imageInputIssue = () => text('当前模型未开启图像输入，请在模型设置中开启并应用，或切换到支持图像的配置。', 'Image input is disabled for this model. Enable and apply it in model settings, or select a profile that supports images.');
 const attachmentHTML = (files, draft = false) => files.length ? `<div class="chat-attachments ${draft ? 'is-draft' : ''}">${files.map((file) => `<div class="chat-attachment" data-attachment-id="${esc(file.id)}">${file.kind === 'image' ? `<img src="${esc(file.preview)}" alt="${esc(file.name)}">` : `<span class="chat-file-icon">${icon('file', 20)}</span>`}<div class="chat-attachment-info"><strong title="${esc(file.name)}">${esc(file.name)}</strong><small>${attachmentSize(file.size)}${file.truncated ? ` · ${text('已截取正文', 'Excerpt attached')}` : ''}</small></div>${draft ? `<button type="button" class="chat-attachment-remove" data-remove-attachment="${esc(file.id)}" aria-label="${esc(text('移除附件：', 'Remove attachment: ') + file.name)}" title="${text('移除附件', 'Remove attachment')}" >${icon('close', 14)}</button>` : ''}</div>`).join('')}</div>` : '';
 const addFiles = (files, initialErrors = []) => {
  if (!files.length || loading || !config) return;
  const current = attachmentGeneration;
  reading++; issue = ''; redraw();
  attachmentQueue = attachmentQueue.then(async () => {
   if (current !== attachmentGeneration) return;
   try {
    validateAttachmentBatch(files, attachments);
    const errors = initialErrors.map((error) => attachmentError(error, getLanguage()));
    for (const file of files) {
     try {
      const attachment = await readChatAttachment(file);
      if (current !== attachmentGeneration) return;
      attachments.push(attachment);
     } catch (err) { errors.push(`${file.name}: ${attachmentError(err, getLanguage())}`); }
    }
    if (current === attachmentGeneration) {
     issue = errors.join(' · ');
     note = text('附件已就绪，可添加说明后发送。', 'Attachments ready. Add a message or send them now.');
    }
   } catch (err) { if (current === attachmentGeneration) issue = attachmentError(err, getLanguage()); }
  }).finally(() => {
   if (current === attachmentGeneration) { reading--; redraw(); document.getElementById('chat-input')?.focus({ preventScroll: true }); }
  });
 };
 const pasteNativeFiles = async (version) => {
  if (loading || !config || version !== filePasteVersion) return;
  const current = attachmentGeneration;
  try {
   const clipboard = await ReadPiChatClipboardFiles();
   // Prefer browser clipboard files when both interfaces expose the same paste.
   if (version !== filePasteVersion || current !== attachmentGeneration) return;
   const files = (clipboard.files || []).map((file) => new File([Uint8Array.from(atob(file.data), (char) => char.charCodeAt(0))], file.name, { type: file.type }));
   addFiles(files, clipboard.errors || []);
   if (!files.length && clipboard.errors?.length) { issue = clipboard.errors.map((error) => attachmentError(error, getLanguage())).join(' · '); redraw(); }
  } catch (err) { if (current === attachmentGeneration) { issue = attachmentError(err, getLanguage()); redraw(); } }
 };
 const scheduleNativePaste = () => {
  clearTimeout(nativePasteTimer);
  const version = filePasteVersion;
  nativePasteTimer = setTimeout(() => { void pasteNativeFiles(version); }, 0);
 };
 const reset = (notice = '', preserveQueue = true) => {
  generation++; queueMenuId = ''; queueEditId = ''; resumeAfterEdit = false;
  runtimeInfo = null; runtimeDialogs = []; showResources = false; runtimeStatuses.clear(); runtimeWidgets.clear();
  issue = ''; note = notice; followBottom = true; scrollTop = 0;
  session.reset(preserveQueue);
 };
 const syncState = () => {
  const state = getState();
  const active = state.profiles.find((p) => p.id === state.activeProfileId);
  const next = JSON.stringify([state.target, state.activeProfileId, state.activeProvider, state.activeModel, active?.baseUrl, active?.apiKey]);
  if (next === signature) return;
  const hadChat = messages.length > 0;
  signature = next; reset(hadChat ? text('配置已切换，已开启新对话。', 'Configuration changed. A new conversation is ready.') : '');
  config = null; loading = false;
 };
 const refresh = async () => {
  if (loading || getState().target !== 'pi') return;
  const current = generation;
  loading = true; issue = ''; redraw();
  try {
   const next = await LoadPiChatConfig();
   if (current !== generation) return;
   if (config && next.id !== config.id) reset(text('配置已切换，已开启新对话。', 'Configuration changed. A new conversation is ready.'));
   config = next;
   if (!session.agent || session.agent.state.closed) {
    const agent = await createAgent();
    if (current !== generation) { agent?.dispose(); return; }
    session.agent = agent;
   }
  } catch (err) { if (current === generation) { config = null; issue = err?.message || String(err); } }
  finally { if (current === generation || config) { loading = false; redraw(); } }
 };
 const messageHTML = () => messages.map((m, index) => `<article class="chat-message ${m.role}" data-no-translate><div class="chat-message-label">${m.role === 'user' ? text('你', 'You') : 'pi agent'}</div>${attachmentHTML(m.attachments || [])}${m.content || m.role === 'assistant' ? `<div class="chat-message-content ${m.role === 'assistant' ? 'model-test-markdown' : ''}">${m.role === 'assistant' ? (m.content ? markdown(m.content) : m.status ? '' : `<span class="chat-waiting">${text('正在思考…', 'Thinking…')}</span>`) : esc(m.content)}</div>` : ''}${m.thinking ? `<details class="chat-thinking" id="chat-thinking-${index}"><summary>${text('思考过程', 'Reasoning')}</summary><div>${esc(m.thinking)}</div></details>` : ''}${m.status ? `<small class="chat-message-status">${esc(m.status)}</small>` : ''}</article>`).join('');
 const paint = () => {
  const list = document.getElementById('chat-messages');
  if (!list) return;
  const opened = [...list.querySelectorAll('details[open]')].map((el) => el.id);
  list.innerHTML = messageHTML();
  messages.forEach((message, index) => {
   if (!message.tools?.length) return;
   const article = list.querySelectorAll('.chat-message')[index];
   article?.insertAdjacentHTML('beforeend', `<details class="chat-thinking chat-tool-list"><summary>${text('工具调用', 'Tool calls')} (${message.tools.length})</summary><div>${message.tools.map((tool) => `${esc(tool.name)} · ${tool.running ? text('执行中', 'Running') : tool.error ? text('失败', 'Failed') : text('完成', 'Done')}`).join('<br>')}</div></details>`);
  });
  opened.forEach((id) => { const el = document.getElementById(id); if (el) el.open = true; });
  if (followBottom) list.scrollTop = list.scrollHeight;
 };
 const schedulePaint = () => { if (!frame) frame = requestAnimationFrame(() => { frame = 0; paint(); }); };
 const createAgent = async () => {
  requirePiRuntimeBindings(window.go?.main?.App, text('当前后端尚未加载对话运行时，请重启应用；开发模式请重新启动 wails dev。', 'The Pi conversation backend is out of date. Restart the app; in development, restart wails dev.'));
  const current = generation;
  const agent = new PiRuntimeAgent({
   id: crypto.randomUUID(), configID: config.id, open: OpenPiChatRuntime,
   command: PiChatRuntimeCommand, close: ClosePiChatRuntime, eventsOn: EventsOn,
   onRuntimeEvent: (event, source) => {
    if (current !== generation) return;
    if (event.type === 'ui') runtimeDialogs.push({ ...event, source });
    if (event.type === 'ui_close') runtimeDialogs = runtimeDialogs.filter((dialog) => dialog.id !== event.id);
    if (event.type === 'notice') note = event.message || '';
    if (event.type === 'status') { if (event.text) runtimeStatuses.set(event.key, event.text); else runtimeStatuses.delete(event.key); }
    if (event.type === 'widget') { if (event.lines) runtimeWidgets.set(event.key, event.lines.join('\n')); else runtimeWidgets.delete(event.key); }
    if (event.type === 'editor_text') input = event.text || '';
    if (event.type === 'resources') runtimeInfo = event.info;
    if (event.type === 'closed') { runtimeDialogs = []; issue = event.message; }
    redraw();
   },
  });
  try {
   await agent.initialize();
   if (current !== generation) { agent.dispose(); return null; }
   runtimeInfo = agent.info; redraw();
   return agent;
  } catch (error) { agent.dispose(); throw error; }
 };

 const runtimeModalHTML = () => {
  const dialog = runtimeDialogs[0];
  if (!dialog && !showResources) return '';
  const title = dialog?.title || text('全局资源', 'Global resources');
  const content = dialog ? `${dialog.message ? `<p>${esc(dialog.message)}</p>` : ''}${dialog.method === 'select' ? `<select id="chat-extension-value">${dialog.options.map((option) => `<option value="${esc(option)}" ${dialog.value === option ? 'selected' : ''}>${esc(option)}</option>`).join('')}</select>` : ['input', 'editor'].includes(dialog.method) ? `<textarea id="chat-extension-value" rows="${dialog.method === 'editor' ? 5 : 2}" placeholder="${esc(dialog.placeholder || '')}">${esc(dialog.value ?? dialog.prefill ?? '')}</textarea>` : ''}` : `<p class="chat-resource-path">${esc(runtimeInfo?.agentDir || '')}</p><h3>Skills (${runtimeInfo?.skills.length || 0})</h3><ul>${(runtimeInfo?.skills || []).map((skill) => `<li><strong>${esc(skill.name)}</strong><small>${esc(skill.path)}</small></li>`).join('')}</ul><h3>Extensions (${runtimeInfo?.extensions.length || 0})</h3><ul>${(runtimeInfo?.extensions || []).map((path) => `<li>${esc(path)}</li>`).join('')}</ul><h3>${text('工具', 'Tools')}</h3><p>${esc((runtimeInfo?.tools || []).join(', '))}</p>${runtimeInfo?.diagnostics.length ? `<h3>${text('加载提示', 'Loading diagnostics')}</h3><ul class="is-error">${runtimeInfo.diagnostics.map((message) => `<li>${esc(message)}</li>`).join('')}</ul>` : ''}`;
  return `<div class="chat-runtime-overlay"><section class="chat-runtime-modal" role="dialog" aria-modal="true" aria-labelledby="chat-runtime-title"><header><h2 id="chat-runtime-title">${esc(title)}</h2><button type="button" id="chat-extension-close" aria-label="${text('关闭', 'Close')}">${icon('close', 18)}</button></header><div class="chat-runtime-body">${content}</div>${dialog ? `<footer><button type="button" class="button button-secondary" id="chat-extension-cancel">${text('取消', 'Cancel')}</button><button type="button" class="button button-primary" id="chat-extension-submit">${text('确定', 'Confirm')}</button></footer>` : ''}</section></div>`;
 };
 const send = () => {
  if (!canSend()) return;
  if (attachments.some((file) => file.kind === 'image') && !config.model.input?.includes('image')) { issue = imageInputIssue(); redraw(); return; }
  const entry = createChatEntry(input, attachments, text('请分析附件中的内容。', 'Please review the attached files and images.'));
  if (session.queue.length >= MAX_QUEUED_MESSAGES) { issue = text('队列最多保留 20 条消息，请先发送或删除部分消息。', 'The queue holds up to 20 messages. Send or remove some first.'); redraw(); return; }
  input = ''; attachments = []; issue = ''; note = '';
  const wasRunning = sending;
  session.enqueue(entry);
  if (!wasRunning && !queueEditId) session.resume();
  document.getElementById('chat-input')?.focus({ preventScroll: true });
 };
 const beginQueueEdit = (id) => {
  const entry = session.queue.find((entry) => entry.id === id);
  if (!entry) return;
  queueMenuId = ''; queueEditId = id; queueEditText = entry.prompt;
  resumeAfterEdit = !session.paused; session.pause();
  document.getElementById('chat-queue-edit')?.focus();
 };
 const finishQueueEdit = (save) => {
  if (save && !session.update(queueEditId, queueEditText)) return;
  const resume = resumeAfterEdit;
  queueEditId = ''; queueEditText = ''; resumeAfterEdit = false;
  if (resume) session.resume(); else redraw();
  document.getElementById('chat-input')?.focus({ preventScroll: true });
 };
 const queueHTML = () => {
  if (!session.queue.length) return '';
  const menuEntry = session.queue.find((entry) => entry.id === queueMenuId);
  return `<section class="chat-queue-panel ${queueEditId ? 'is-editing' : ''}" aria-label="${text('待发送消息', 'Pending messages')}">
   ${session.paused && !queueEditId ? `<div class="chat-queue-paused"><span>${text('队列已暂停', 'Queue paused')}</span><button id="chat-queue-resume" type="button">${icon('play', 14)}${text('继续发送', 'Resume queue')}</button></div>` : ''}
   <div class="chat-queue-list" role="list">${session.queue.map((entry, index) => `<div class="chat-queue-row ${entry.mode === 'steering' ? 'is-steering' : ''}" role="listitem" data-queue-id="${esc(entry.id)}">
    ${queueEditId === entry.id ? `<div class="chat-queue-edit"><label class="sr-only" for="chat-queue-edit">${text('编辑待发送消息', 'Edit pending message')}</label><textarea id="chat-queue-edit" rows="2" maxlength="16000">${esc(queueEditText)}</textarea>${entry.attachments.length ? `<small>${entry.attachments.length} ${text('个附件将保留', 'attachments will be kept')}</small>` : ''}<div><button type="button" id="chat-queue-edit-cancel">${text('取消', 'Cancel')}</button><button type="button" id="chat-queue-edit-save" ${!queueEditText.trim() && !entry.attachments.length ? 'disabled' : ''}>${text('保存', 'Save')}</button></div></div>` : `<span class="chat-queue-icon">${icon(entry.attachments.length ? 'paperclip' : 'file', 16)}</span><div class="chat-queue-preview"><span title="${esc(entry.prompt || entry.attachments.map((file) => file.name).join(', '))}" data-no-translate>${esc(entry.prompt || entry.attachments.map((file) => file.name).join(', '))}</span>${entry.attachments.length && entry.prompt ? `<small>${entry.attachments.length} ${text('个附件', 'attachments')}</small>` : ''}</div><div class="chat-queue-actions"><button type="button" class="chat-queue-steer" data-queue-steer="${esc(entry.id)}" title="${text('中断当前回复，优先处理这条消息', 'Interrupt the current reply and handle this message first')}" ${queueEditId ? 'disabled' : ''}>${icon('steer', 16)}<span>${text('引导', 'Steer')}</span></button><button type="button" data-queue-delete="${esc(entry.id)}" aria-label="${text('删除待发送消息', 'Delete pending message')}" title="${text('删除', 'Delete')}" ${queueEditId ? 'disabled' : ''}>${icon('trash', 16)}</button><button type="button" id="chat-queue-more-${esc(entry.id)}" data-queue-more="${esc(entry.id)}" aria-label="${text('待发送消息的更多操作', 'More actions for pending message')}" aria-expanded="${queueMenuId === entry.id}" aria-controls="chat-queue-popover" ${queueEditId ? 'disabled' : ''}>${icon('more', 16)}</button></div>`}
   </div>`).join('')}</div>
   ${menuEntry ? `<div class="chat-queue-popover" id="chat-queue-popover"><button type="button" data-queue-edit="${esc(menuEntry.id)}">${icon('edit', 15)}${text('编辑消息', 'Edit message')}</button>${session.queue.indexOf(menuEntry) > 0 ? `<button type="button" data-queue-first="${esc(menuEntry.id)}">${icon('arrowUp', 15)}${text('移到队首', 'Move to front')}</button>` : ''}</div>` : ''}
  </section>`;
 };
 const render = (disabled = '') => {
  const state = getState();
  const draftDisabled = disabled || (!config || loading ? ' disabled' : '');
  const noticeIssue = issue || session.error;
  return `<section class="chat-page" aria-label="${text('对话', 'Conversation')}">
   <div class="chat-toolbar">
    <div class="chat-connection"><span class="chat-connection-icon">${icon('spark', 20)}</span><div><strong data-no-translate>${esc(config?.model.id || state.activeModel || text('尚未选择模型', 'No model selected'))}</strong><small data-no-translate>${esc(config?.profileName || text('使用当前 pi agent 配置', 'Use the current pi agent configuration'))}</small></div></div>
    <div class="chat-toolbar-actions"><label class="chat-profile-field"><select id="chat-profile" aria-label="${text('对话配置', 'Conversation profile')}" ${disabled}${sending || loading ? ' disabled' : ''}><option value="">${text('选择已保存的配置', 'Choose a saved profile')}</option>${state.profiles.map((p) => `<option value="${esc(p.id)}" ${state.activeProfileId === p.id ? 'selected' : ''}>${esc(p.name)}</option>`).join('')}</select></label><button id="chat-new" class="button button-secondary" ${disabled}${sending || loading ? ' disabled' : ''}>${icon('plus', 16)}${text('新对话', 'New chat')}</button></div>
   </div>
   <div class="chat-transcript" id="chat-messages" role="log" aria-label="${text('聊天记录', 'Conversation')}" aria-live="off" aria-busy="${sending}">${messages.length ? messageHTML() : `<div class="chat-empty"><span class="chat-empty-icon">${icon('spark', 32)}</span><h2>${text('今天想聊什么？', 'What would you like to talk about?')}</h2><p>${text('可粘贴图片、文件，或直接输入消息。', 'Paste images or files, or type a message.')}</p><div class="chat-suggestions">${[text('你好，请介绍一下你自己。', 'Hello, please introduce yourself.'), text('用简单的例子解释什么是 Agent。', 'Explain what an agent is with a simple example.')].map((p) => `<button class="chat-suggestion" data-chat-prompt="${esc(p)}" ${disabled}${!config || loading ? ' disabled' : ''}>${esc(p)}${icon('chevronRight', 16)}</button>`).join('')}</div></div>`}</div>
   <div class="chat-composer-area">
    <div class="chat-notice ${noticeIssue ? 'is-error' : ''}" role="status" aria-live="polite">${esc(noticeIssue || (loading ? text('正在读取当前配置…', 'Reading current configuration…') : reading ? text('正在读取附件…', 'Reading attachments…') : note || session.notice))}${!config && !loading ? `<button class="text-button" id="chat-model-settings">${text('前往模型设置', 'Go to model settings')}</button>` : ''}</div>
    ${queueHTML()}
    <form class="chat-composer" id="chat-form">
     ${attachmentHTML(attachments, true)}
     <label class="sr-only" for="chat-input">${text('发送消息', 'Message')}</label><textarea id="chat-input" rows="2" maxlength="16000" placeholder="${text('输入消息，或粘贴图片和文件…', 'Message, or paste images and files…')}" ${disabled}${!config || loading ? ' disabled' : ''}>${esc(input)}</textarea>
     <div class="chat-composer-footer"><div class="chat-composer-tools"><button class="chat-attach" type="button" id="chat-attach" aria-label="${text('添加图片或文件', 'Attach images or files')}" title="${text('添加图片或文件（最多 8 个，单个 8 MB）', 'Attach images or files (8 files, 8 MB each)')}" ${draftDisabled}>${icon('paperclip', 20)}</button><input id="chat-files" type="file" multiple hidden ${draftDisabled}><span>${text('Enter 发送 · Shift + Enter 换行', 'Enter to send · Shift + Enter for a new line')}</span></div><div class="chat-composer-actions">${sending ? `<button class="chat-send chat-stop" type="button" id="chat-stop" aria-label="${text('停止生成', 'Stop generating')}" title="${text('停止生成', 'Stop generating')}">${icon('stop', 18)}</button>` : ''}<button class="chat-send" type="submit" id="chat-send" aria-label="${sending ? text('加入队列', 'Queue message') : text('发送消息', 'Send message')}" title="${sending ? text('加入队列', 'Queue message') : text('发送消息', 'Send message')}" ${sending && !input.trim() && !attachments.length ? 'hidden' : ''} ${disabled}${!canSend() ? ' disabled' : ''}>${icon('arrowUp', 20)}</button></div></div>
    </form>
    ${(runtimeStatuses.size || runtimeWidgets.size) ? `<div class="chat-extension-status" role="status">${esc([...runtimeStatuses.values(), ...runtimeWidgets.values()].join('\n'))}</div>` : ''}
    <p class="chat-footnote">${runtimeInfo ? `<button class="text-button" type="button" id="chat-resources">${runtimeInfo.skills.length} skills · ${runtimeInfo.extensions.length} extensions${runtimeInfo.diagnostics.length ? ` · ${text('查看加载提示', 'View diagnostics')}` : ''}</button> · ` : ''}${text('对话仅保留在本次应用会话中', 'Conversations stay in this app session')}</p>
   </div>
   ${runtimeModalHTML()}
  </section>`;
 };
 const bind = ({ switchProfile, openModels }) => {
  const dialog = runtimeDialogs[0];
  const replyToExtension = (value) => {
   if (dialog) { runtimeDialogs = runtimeDialogs.filter((item) => item !== dialog); void dialog.source.send({ type: 'ui_response', id: dialog.id, value }).catch((error) => { issue = error.message; redraw(); }); }
   showResources = false; redraw(); if (!runtimeDialogs.length) document.getElementById('chat-input')?.focus();
  };
  document.getElementById('chat-resources')?.addEventListener('click', () => { showResources = true; redraw(); document.getElementById('chat-extension-close')?.focus(); });
  document.getElementById('chat-extension-close')?.addEventListener('click', () => replyToExtension(null));
  document.getElementById('chat-extension-cancel')?.addEventListener('click', () => replyToExtension(null));
  document.getElementById('chat-extension-submit')?.addEventListener('click', () => replyToExtension(dialog?.method === 'confirm' ? true : document.getElementById('chat-extension-value')?.value));
  const extensionField = document.getElementById('chat-extension-value');
  extensionField?.addEventListener('input', () => { dialog.value = extensionField.value; dialog.selection = [extensionField.selectionStart, extensionField.selectionEnd]; });
  extensionField?.addEventListener('change', () => { dialog.value = extensionField.value; });
  const modal = document.querySelector('.chat-runtime-modal');
  modal?.addEventListener('keydown', (event) => {
   if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); replyToExtension(null); }
   if (event.key === 'Tab') {
    const controls = [...modal.querySelectorAll('button, select, textarea')];
    const first = controls[0], last = controls.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
   }
  });
  if (modal && !modal.contains(document.activeElement)) (document.getElementById('chat-extension-value') || document.getElementById('chat-extension-cancel') || document.getElementById('chat-extension-close'))?.focus();
  if (extensionField?.tagName === 'TEXTAREA' && dialog.selection) extensionField.setSelectionRange(...dialog.selection);
  const field = document.getElementById('chat-input');
  field?.addEventListener('input', (event) => { input = event.target.value; const button = document.getElementById('chat-send'); if (button) { button.disabled = !canSend(); button.hidden = sending && !input.trim() && !attachments.length; } field.style.height = 'auto'; field.style.height = Math.min(field.scrollHeight, 160) + 'px'; });
  field?.addEventListener('keydown', (event) => {
   if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && event.keyCode !== 229) { event.preventDefault(); void send(); }
   // Explorer's copied files may expose no web clipboard formats or paste event.
   if (event.key.toLowerCase() === 'v' && (event.ctrlKey || event.metaKey) && !event.shiftKey && !event.repeat) scheduleNativePaste();
  });
  field?.addEventListener('paste', (event) => {
   const files = [...(event.clipboardData?.files || [])];
   if (files.length) { event.preventDefault(); clearTimeout(nativePasteTimer); filePasteVersion++; addFiles(files); }
   else scheduleNativePaste();
  });
  field?.addEventListener('focus', () => {
   if (queueMenuId) {
    queueMenuId = ''; document.getElementById('chat-queue-popover')?.remove();
    document.querySelector('[data-queue-more][aria-expanded="true"]')?.setAttribute('aria-expanded', 'false');
   }
  });
  document.getElementById('chat-queue-resume')?.addEventListener('click', () => { issue = ''; note = ''; session.resume(); });
  document.querySelectorAll('[data-queue-steer]').forEach((button) => button.addEventListener('click', () => { queueMenuId = ''; issue = ''; note = ''; session.steer(button.dataset.queueSteer); }));
  document.querySelectorAll('[data-queue-delete]').forEach((button) => button.addEventListener('click', () => session.remove(button.dataset.queueDelete)));
  document.querySelectorAll('[data-queue-more]').forEach((button) => button.addEventListener('click', () => {
   queueMenuId = queueMenuId === button.dataset.queueMore ? '' : button.dataset.queueMore;
   redraw(); document.querySelector('[data-queue-edit]')?.focus();
  }));
  document.querySelectorAll('[data-queue-edit]').forEach((button) => button.addEventListener('click', () => beginQueueEdit(button.dataset.queueEdit)));
  document.querySelectorAll('[data-queue-first]').forEach((button) => button.addEventListener('click', () => { queueMenuId = ''; session.moveFirst(button.dataset.queueFirst); }));
  const queueEditor = document.getElementById('chat-queue-edit');
  queueEditor?.addEventListener('input', (event) => {
   queueEditText = event.target.value;
   const entry = session.queue.find((entry) => entry.id === queueEditId);
   document.getElementById('chat-queue-edit-save').disabled = !queueEditText.trim() && !entry?.attachments.length;
  });
  queueEditor?.addEventListener('keydown', (event) => {
   if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); finishQueueEdit(false); }
   if (event.key === 'Enter' && (event.ctrlKey || event.metaKey) && !event.isComposing) { event.preventDefault(); finishQueueEdit(true); }
  });
  document.getElementById('chat-queue-edit-save')?.addEventListener('click', () => finishQueueEdit(true));
  document.getElementById('chat-queue-edit-cancel')?.addEventListener('click', () => finishQueueEdit(false));
  document.querySelector('.chat-queue-panel')?.addEventListener('keydown', (event) => {
   if (event.key === 'Escape' && queueMenuId) {
    const id = queueMenuId; queueMenuId = ''; event.preventDefault(); redraw();
    document.getElementById(`chat-queue-more-${id}`)?.focus();
   }
  });
  const form = document.getElementById('chat-form');
  form?.addEventListener('submit', (event) => { event.preventDefault(); void send(); });
  form?.addEventListener('dragover', (event) => { if (event.dataTransfer?.types.includes('Files')) { event.preventDefault(); if (config) form.classList.add('is-dragging'); } });
  form?.addEventListener('dragleave', (event) => { if (!form.contains(event.relatedTarget)) form.classList.remove('is-dragging'); });
  form?.addEventListener('drop', (event) => { event.preventDefault(); form.classList.remove('is-dragging'); addFiles([...(event.dataTransfer?.files || [])]); });
  document.getElementById('chat-attach')?.addEventListener('click', () => document.getElementById('chat-files')?.click());
  document.getElementById('chat-files')?.addEventListener('change', (event) => { addFiles([...event.target.files]); event.target.value = ''; });
  document.querySelectorAll('[data-remove-attachment]').forEach((button) => button.addEventListener('click', () => { attachments = attachments.filter((file) => file.id !== button.dataset.removeAttachment); issue = ''; note = ''; redraw(); document.getElementById('chat-input')?.focus(); }));
  document.getElementById('chat-stop')?.addEventListener('click', () => { resumeAfterEdit = false; session.stop(); });
  document.getElementById('chat-new')?.addEventListener('click', () => { reset('', false); clearTimeout(nativePasteTimer); input = ''; attachments = []; attachmentGeneration++; reading = 0; redraw(); void refresh(); document.getElementById('chat-input')?.focus(); });
  document.getElementById('chat-model-settings')?.addEventListener('click', openModels);
  document.getElementById('chat-profile')?.addEventListener('change', (event) => { if (event.target.value) switchProfile(event.target.value); });
  document.querySelectorAll('[data-chat-prompt]').forEach((button) => button.addEventListener('click', () => { input = button.dataset.chatPrompt; redraw(); document.getElementById('chat-input')?.focus(); }));
  const list = document.getElementById('chat-messages');
  list?.addEventListener('scroll', () => { scrollTop = list.scrollTop; followBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 60; });
  if (list) list.scrollTop = followBottom && messages.length ? list.scrollHeight : scrollTop;
 };
 return { syncState, refresh, render, bind };
}
