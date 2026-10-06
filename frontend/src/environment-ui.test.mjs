import assert from 'node:assert/strict';
import test from 'node:test';
import { createEnvironmentManager } from './environment-ui.mjs';

const esc = (value = '') => String(value).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }[c]));
const missingEnvironment = () => ({ platform: 'windows', architecture: 'amd64', installDirectory: 'C:\\Users\\New User\\ModelSwitcher\\tools', tools: ['node', 'npm', 'pi', 'codex'].map((id) => ({ id, name: id, ready: false, installed: false, error: '未安装' })) });
const manager = (options = {}) => createEnvironmentManager({ detect: async () => missingEnvironment(), startInstall: async (id) => ({ id, status: 'running', progress: 10 }), getInstallState: async () => ({ id: 'pi', status: 'success', progress: 100, message: '已完成' }), redraw() {}, getLanguage: () => 'zh', icon: () => '', esc, onInstalled: async () => {}, ...options });

test('a clean environment offers all four installations and a target-specific notice', async () => {
 const app = manager(); await app.refresh();
 const html = app.render();
 for (const id of ['node', 'npm', 'pi', 'codex']) assert.match(html, new RegExp(`data-install-dependency="${id}"`));
 assert.match(app.notice('pi'), /node、npm、pi/);
 assert.doesNotMatch(app.notice('pi'), /codex/);
});

test('installed and compatible tools hide installation controls; old Node offers repair', async () => {
 const state = missingEnvironment(); state.tools[0] = { id: 'node', name: 'Node.js', ready: false, installed: true, version: 'v20.0.0', error: '需要升级' };
 state.tools[1].ready = true;
 const app = manager({ detect: async () => state }); await app.refresh();
 assert.match(app.render(), /修复 \/ 安装/);
 assert.doesNotMatch(app.render(), /data-install-dependency="npm"/);
});

test('installation prevents duplicate requests, survives transient polling failure and refreshes readiness', async () => {
 let installs = 0, polls = 0, detections = 0, completions = 0;
 const app = manager({
  detect: async () => { detections++; const state = missingEnvironment(); state.tools.forEach((tool) => { tool.ready = installs > 0; }); return state; },
  startInstall: async (id) => { installs++; return { id, status: 'running', progress: 30 }; },
  getInstallState: async () => { if (++polls === 1) throw new Error('temporary disconnect'); return { id: 'pi', status: 'success', progress: 100, message: '已完成' }; },
  onInstalled: async () => { completions++; },
 });
 await app.refresh();
 const pending = app.install('pi');
 await app.install('codex');
 assert.equal(installs, 1); assert.equal(app.isInstalling(), true);
 assert.match(app.render(), /<progress/);
 assert.match(app.render(), /data-install-dependency="codex" disabled/);
 await pending;
 assert.equal(app.isInstalling(), false); assert.equal(detections, 2); assert.equal(completions, 1);
 assert.equal(app.notice('pi'), '');
});

test('failed installs expose errors and a usable retry without HTML injection', async () => {
 const app = manager({ getInstallState: async () => ({ id: 'pi', status: 'error', message: '安装失败', error: '<script>unsafe</script>' }) });
 await app.refresh(); await app.install('pi');
 assert.match(app.render(), /重试/);
 assert.match(app.render(), /&lt;script&gt;/);
 assert.doesNotMatch(app.render(), /<script>/);
 assert.equal(app.isInstalling(), false);
});

test('detection failure keeps a manual retry available', async () => {
 const app = manager({ detect: async () => { throw new Error('读取失败'); } });
 await app.refresh(); assert.match(app.render(), /读取失败/);
 assert.match(app.render(), /id="environment-detect" >/);
});
