// Runs with the user's installed Pi SDK, outside the WebView. Stdout is JSONL
// only; extension logs go to stderr so they cannot corrupt the IPC protocol.
import { pathToFileURL } from 'node:url';
import { dirname, join } from 'node:path';
import { createHash, randomUUID } from 'node:crypto';

const output = process.stdout.write.bind(process.stdout);
const emit = (event) => output(JSON.stringify(event) + '\n');
console.log = console.info = console.debug = (...args) => console.error(...args);
let session, loader, sdk, extensionTheme, cwd, agentDir, busy = false, shuttingDown = false, runID = '', editorText = '';
const dialogs = new Map();
const historyLeaves = new Map();
const historyKey = (history) => createHash('sha256').update(JSON.stringify(history)).digest('hex');
function rememberHistory(history = session.messages) {
 historyLeaves.set(historyKey(history), session.sessionManager.getLeafId());
 if (historyLeaves.size > 64) historyLeaves.delete(historyLeaves.keys().next().value);
}

function cancelDialogs() {
 for (const finish of [...dialogs.values()]) finish(undefined);
}
function dialog(method, data, opts = {}) {
 const id = randomUUID();
 return new Promise((resolve) => {
  let timer;
  const finish = (value) => {
   clearTimeout(timer); opts.signal?.removeEventListener('abort', abort);
   dialogs.delete(id); emit({ type: 'ui_close', id }); resolve(value);
  };
  const abort = () => finish(undefined);
  dialogs.set(id, finish);
  opts.signal?.addEventListener('abort', abort, { once: true });
  if (opts.signal?.aborted) { finish(undefined); return; }
  if (opts.timeout) timer = setTimeout(abort, opts.timeout);
  emit({ type: 'ui', id, method, ...data });
 });
}
function createUI() {
 const ui = {
  select: (title, options, opts) => dialog('select', { title, options }, opts),
  confirm: async (title, message, opts) => (await dialog('confirm', { title, message }, opts)) === true,
  input: (title, placeholder, opts) => dialog('input', { title, placeholder }, opts),
  editor: (title, prefill) => dialog('editor', { title, prefill }),
  notify: (message, level) => emit({ type: 'notice', message, level }),
  setStatus: (key, text) => emit({ type: 'status', key, text }),
  setWidget: (key, content) => { if (!content || Array.isArray(content)) emit({ type: 'widget', key, lines: content }); },
  setEditorText: (text) => { editorText = text; emit({ type: 'editor_text', text }); },
  getEditorText: () => editorText,
  pasteToEditor: (text) => ui.setEditorText(text),
  onTerminalInput: () => () => {}, custom: async () => undefined,
  getAllThemes: () => [], getTheme: () => undefined,
  setTheme: () => ({ success: false, error: 'Terminal themes are unavailable in the conversation UI.' }),
  getToolsExpanded: () => false, getEditorComponent: () => undefined,
  theme: extensionTheme,
 };
 for (const name of ['setWorkingMessage', 'setWorkingVisible', 'setWorkingIndicator', 'setHiddenThinkingLabel', 'setFooter', 'setHeader', 'setTitle', 'addAutocompleteProvider', 'setEditorComponent', 'setToolsExpanded']) ui[name] = () => {};
 // Match Pi's RPC UI contract: terminal component factories are unavailable.
 return ui;
}

async function init(input) {
 sdk = await import(pathToFileURL(input.sdkPath).href);
 if (!sdk.ModelRuntime) throw new Error('当前 Pi SDK 版本过旧，请更新 pi CLI 后重试。');
 const settingsManager = sdk.SettingsManager.create(input.cwd, input.agentDir);
 cwd = input.cwd; agentDir = input.agentDir;
 sdk.initTheme(settingsManager.getTheme(), false);
 ({ theme: extensionTheme } = await import(pathToFileURL(join(dirname(input.sdkPath), 'modes', 'interactive', 'theme', 'theme.js')).href));
 const factories = ['createCodemodeExtension', 'createToolSearchExtension', 'createMcpExtension']
  .filter((name) => typeof sdk[name] === 'function').map((name) => sdk[name]());
 loader = new sdk.DefaultResourceLoader({ cwd: input.cwd, agentDir: input.agentDir, settingsManager, extensionFactories: factories });
 await loader.reload();
 const modelRuntime = await sdk.ModelRuntime.create({
  authPath: join(input.agentDir, 'auth.json'), modelsPath: join(input.agentDir, 'models.json'), refreshOnCreate: false,
 });
 // Credentials arrive over the private stdin pipe, never through frontend state,
 // command-line arguments, or temporary files.
 modelRuntime.registerProvider(input.model.provider, {
  api: input.model.api, baseUrl: input.model.baseUrl, apiKey: 'model-switcher-runtime',
  headers: input.headers, models: [input.model],
 });
 await modelRuntime.setRuntimeApiKey(input.model.provider, input.apiKey || 'model-switcher-local');
 ({ session } = await sdk.createAgentSession({
  cwd: input.cwd, agentDir: input.agentDir, settingsManager, modelRuntime,
  model: modelRuntime.getModel(input.model.provider, input.model.id),
  resourceLoader: loader, sessionManager: sdk.SessionManager.inMemory(input.cwd),
 }));
 session.subscribe((event) => emit({ type: 'event', runID, event }));
 await session.bindExtensions({
  mode: 'rpc', uiContext: createUI(),
  onError: (error) => emit({ type: 'notice', level: 'error', message: error.message }),
  commandContextActions: {
   waitForIdle: () => session.waitForIdle(), reload: async () => { await session.reload(); emit({ type: 'resources', info: resourceInfo() }); },
   ...Object.fromEntries(['newSession', 'fork', 'navigateTree', 'switchSession'].map((name) => [name, async () => { throw new Error('内置对话不支持 Pi CLI 历史会话操作，请使用新对话按钮或 pi CLI。'); }])),
  },
  shutdownHandler: () => { cancelDialogs(); void session.abort(); emit({ type: 'notice', message: 'Pi extension requested shutdown.' }); },
 });
 rememberHistory([]); rememberHistory();
 emit({ type: 'ready', info: resourceInfo() });
}
function resourceInfo() {
 const extensions = loader.getExtensions();
 return {
  cwd, agentDir,
  skills: loader.getSkills().skills.map((skill) => ({ name: skill.name, path: skill.filePath })),
  extensions: extensions.extensions.map((extension) => extension.path),
  tools: session.getActiveToolNames(),
  diagnostics: [...extensions.errors.map((error) => `${error.path}: ${error.error}`),
   ...loader.getSkills().diagnostics.map((item) => item.message)],
 };
}

function syncHistory(history) {
 if (JSON.stringify(history) === JSON.stringify(session.messages)) return;
 const manager = session.sessionManager;
 const key = historyKey(history);
 if (historyLeaves.has(key)) {
  const leaf = historyLeaves.get(key);
  if (leaf) manager.branch(leaf); else manager.resetLeaf();
  session.agent.state.messages = history;
  return;
 }
 // Steering keeps a normalized partial answer. Branch immediately before its
 // original entry so compaction and extension metadata remain authoritative.
 const current = session.messages;
 if (history.length === current.length && history.at(-1)?.role === 'assistant' && historyKey(history.slice(0, -1)) === historyKey(current.slice(0, -1))) {
  const original = [...manager.getBranch()].reverse().find((entry) => entry.type === 'message' && entry.message.role === 'assistant');
  if (original) {
   if (original.parentId) manager.branch(original.parentId); else manager.resetLeaf();
   manager.appendMessage(history.at(-1)); session.agent.state.messages = history; rememberHistory(); return;
  }
 }
 const branch = manager.getBranch();
 const entries = branch.filter((entry) => entry.type === 'message');
 let common = 0;
 while (common < history.length && common < entries.length && JSON.stringify(history[common]) === JSON.stringify(entries[common].message)) common++;
 // Keep extension metadata and earlier messages on the existing branch. Only
 // a stopped/failed turn or a normalized partial answer creates a new branch.
 if (common) manager.branch(entries[common - 1].id);
 else {
  const firstMessage = branch.findIndex((entry) => entry.type === 'message');
  if (firstMessage > 0) manager.branch(branch[firstMessage - 1].id);
  else manager.resetLeaf();
 }
 for (const message of history.slice(common)) manager.appendMessage(message);
 session.agent.state.messages = history;
}

async function run(input) {
 if (busy) throw new Error('上一条回复尚未结束');
 busy = true; runID = input.id;
 try {
  syncHistory(input.history);
  editorText = input.editorText || '';
  const message = input.message;
  const text = message.content.filter((part) => part.type === 'text').map((part) => part.text).join('\n');
  let disposition;
  if (text.trim() === '/reload') {
   await session.reload(); disposition = 'handled';
   emit({ type: 'resources', info: resourceInfo() });
   emit({ type: 'notice', message: '已重新加载 Pi 全局 skills 和 extensions。' });
  } else await session.prompt(text, { images: message.content.filter((part) => part.type === 'image'), source: 'interactive', preflightResult: (value) => { disposition = value; } });
  rememberHistory();
  emit({ type: 'done', runID, disposition, messages: session.messages });
 } catch (error) {
  rememberHistory();
  emit({ type: 'failed', runID, message: error.message || String(error), messages: session.messages });
 } finally { busy = false; runID = ''; }
}

let initialized = false;
async function command(input) {
 if (input.type === 'init') { void init(input).then(() => { initialized = true; }).catch((error) => emit({ type: 'fatal', message: error.message || String(error) })); return; }
 if (input.type === 'ui_response') { dialogs.get(input.id)?.(input.value); return; }
 if (!initialized) throw new Error('Pi runtime is not ready');
 if (input.type === 'run') { void run(input).catch((error) => emit({ type: 'failed', runID: input.id, message: error.message })); return; }
 if (input.type === 'abort') { cancelDialogs(); await session.abort(); return; }
 if (input.type === 'editor_state') { editorText = input.text || ''; return; }
 if (input.type === 'dispose') { await shutdown(); return; }
 throw new Error('Unsupported Pi runtime command');
}
async function shutdown() {
 if (shuttingDown) return;
 shuttingDown = true;
 cancelDialogs();
 if (session) {
  await session.abort();
  // Extensions such as MCP own subprocesses and release them on this event.
  await session.extensionRunner?.emit({ type: 'session_shutdown' });
  session.dispose();
 }
 process.exit(0);
}
// Split only on LF. U+2028/U+2029 are valid in JSON strings.
let buffer = '', commandChain = Promise.resolve();
process.stdin.setEncoding('utf8');
process.stdin.on('data', (chunk) => {
 buffer += chunk;
 let end;
 while ((end = buffer.indexOf('\n')) >= 0) {
  const line = buffer.slice(0, end); buffer = buffer.slice(end + 1);
  commandChain = commandChain.then(() => command(JSON.parse(line))).catch((error) => emit({ type: initialized ? 'notice' : 'fatal', level: 'error', message: error.message || String(error) }));
 }
});
process.stdin.on('end', () => void shutdown());
