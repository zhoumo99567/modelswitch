import assert from 'node:assert/strict';
import test from 'node:test';
import { PiRuntimeAgent, requirePiRuntimeBindings } from './chat-runtime.mjs';
import { ChatSession, createChatEntry } from './chat-session.mjs';
import { OpenPiChatRuntime, ClosePiChatRuntime, PiChatRuntimeCommand } from '../wailsjs/go/main/App.js';

const tick = () => new Promise((resolve) => setImmediate(resolve));
function fixture() {
 let receive;
 const calls = [], restored = [], runtimeEvents = [];
 const agent = new PiRuntimeAgent({ id: 'session', configID: 'config',
  open: async () => ({ skills: [{ name: 'global' }], extensions: ['global.js'] }),
  command: async (_, json) => { calls.push(JSON.parse(json)); }, close: async () => {},
  eventsOn: (_, callback) => { receive = callback; return () => {}; },
  onRuntimeEvent: (event) => runtimeEvents.push(event),
 });
 const emit = (event) => receive({ sessionID: 'session', event });
 const session = new ChatSession({ getAgent: async () => agent, onRestore: (entry) => { restored.push(entry); return true; } });
 const requests = () => calls.filter((call) => call.type === 'run');
 const wait = async (count) => { for (let i = 0; i < 100 && requests().length < count; i++) await tick(); assert.equal(requests().length, count); };
 const finish = (text = 'reply', stopReason = 'stop') => {
  const request = requests().at(-1);
  const message = { role: 'assistant', content: [{ type: 'text', text }], stopReason };
  emit({ type: 'event', runID: request.id, event: { type: 'message_end', message } });
  emit({ type: 'done', runID: request.id, messages: [...request.history, request.message, message] });
 };
 return { agent, session, emit, calls, requests, wait, finish, restored, runtimeEvents };
}

test('runtime loads metadata and retains FIFO with one backend run at a time', async () => {
 const f = fixture(); await f.agent.initialize();
 assert.equal(f.agent.info.skills[0].name, 'global');
 f.session.enqueue(createChatEntry('first')); await f.wait(1);
 f.session.enqueue(createChatEntry('second')); f.session.enqueue(createChatEntry('third'));
 assert.equal(f.requests().length, 1);
 f.finish(); await f.wait(2); assert.equal(f.requests()[1].message.content[0].text, 'second');
 assert.equal(f.requests()[1].history.length, 2);
 f.finish(); await f.wait(3); f.finish(); await f.session.waitForIdle();
 assert.equal(f.session.messages.length, 6); assert.equal(f.session.queue.length, 0);
});

test('steering waits for cancellation and preserves partial text in SDK history', async () => {
 const f = fixture(); f.session.enqueue(createChatEntry('original')); await f.wait(1);
 const guidance = createChatEntry('guidance'); f.session.enqueue(guidance);
 f.session.steer(guidance.id);
 assert.equal(f.calls.at(-1).type, 'abort'); assert.equal(f.requests().length, 1);
 f.finish('partial answer', 'aborted'); await f.wait(2);
 assert.equal(f.requests()[1].history[1].stopReason, 'stop');
 assert.equal(f.requests()[1].history[1].content[0].text, 'partial answer');
 assert.equal(f.requests()[1].message.content[0].text, 'guidance');
 f.finish(); await f.session.waitForIdle();
});

test('failed runtime turn restores the draft and rolls back only that turn', async () => {
 const f = fixture(); f.session.enqueue(createChatEntry('success')); await f.wait(1); f.finish(); await f.session.waitForIdle();
 f.session.enqueue(createChatEntry('retry')); await f.wait(2);
 const request = f.requests()[1];
 f.emit({ type: 'failed', runID: request.id, messages: [...request.history, request.message], message: 'provider failed' });
 await f.session.waitForIdle();
 assert.equal(f.restored[0].prompt, 'retry'); assert.equal(f.agent.state.messages.length, 2); assert.equal(f.session.paused, true);
});

test('extension commands consumed before user events finish without stalling the queue', async () => {
 const f = fixture(); f.session.enqueue(createChatEntry('/extension')); await f.wait(1);
 f.session.enqueue(createChatEntry('next'));
 f.emit({ type: 'done', runID: f.requests()[0].id, disposition: 'handled', messages: [] });
 await f.wait(2); f.finish(); await f.session.waitForIdle();
 assert.equal(f.session.messages[1].status, 'Handled by extension');
 assert.equal(f.session.queue.length, 0);
});

test('runtime forwards extension UI and tool events and ignores another session', async () => {
 const f = fixture(); f.session.enqueue(createChatEntry('tools')); await f.wait(1);
 f.emit({ type: 'ui', id: 'dialog', method: 'confirm', title: 'Extension' });
 assert.equal(f.runtimeEvents[0].id, 'dialog');
 f.emit({ type: 'event', runID: f.requests()[0].id, event: { type: 'tool_execution_start', toolCallId: 'tool', toolName: 'global_tool' } });
 assert.equal(f.session.active.reply.tools[0].running, true);
 f.emit({ type: 'event', runID: f.requests()[0].id, event: { type: 'tool_execution_end', toolCallId: 'tool', toolName: 'global_tool' } });
 assert.equal(f.session.active.reply.tools[0].running, false);
 f.finish(); await f.session.waitForIdle();
});

test('closing a runtime settles pending work and removes its event listener', async () => {
 const f = fixture(); const pending = f.agent.prompt(createChatEntry('old').message);
 f.agent.dispose(); await assert.rejects(pending, /disposed/); assert.equal(f.agent.state.closed, true);
});

test('missing Go close binding cannot throw during failed initialization cleanup', async (t) => {
 const previousWindow = globalThis.window;
 globalThis.window = { go: { main: { App: {} } } };
 t.after(() => { if (previousWindow === undefined) delete globalThis.window; else globalThis.window = previousWindow; });
 const f = fixture(); f.agent.open = OpenPiChatRuntime; f.agent.close = ClosePiChatRuntime;
 await assert.rejects(async () => {
  try { await f.agent.initialize(); } catch (error) { f.agent.dispose(); throw error; }
 }, /OpenPiChatRuntime/);
 await tick();
});

test('synchronous Go command failures settle the pending prompt and allow retry', async (t) => {
 const previousWindow = globalThis.window;
 globalThis.window = { go: { main: { App: {} } } };
 t.after(() => { if (previousWindow === undefined) delete globalThis.window; else globalThis.window = previousWindow; });
 const f = fixture(); f.agent.command = PiChatRuntimeCommand;
 await assert.rejects(f.agent.prompt(createChatEntry('missing command').message), /PiChatRuntimeCommand/);
 assert.equal(f.agent.pending, null); assert.equal(f.agent.state.isStreaming, false);
});

test('runtime checks the actual Go bindings before invoking generated wrappers', () => {
 assert.throws(() => requirePiRuntimeBindings({}), /restart wails dev/i);
 assert.throws(() => requirePiRuntimeBindings({ OpenPiChatRuntime() {}, PiChatRuntimeCommand() {} }), /restart wails dev/i);
 assert.doesNotThrow(() => requirePiRuntimeBindings({ OpenPiChatRuntime() {}, PiChatRuntimeCommand() {}, ClosePiChatRuntime() {} }));
});
