import assert from 'node:assert/strict';
import test from 'node:test';
import { Agent } from '@earendil-works/pi-agent-core';
import { AssistantMessageEventStream } from '@earendil-works/pi-ai';
import { ChatSession, createChatEntry } from './chat-session.mjs';

const model = { id: 'test', name: 'test', api: 'openai-completions', provider: 'test', input: ['text', 'image'], contextWindow: 32768, maxTokens: 1000, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 } };
const usage = { input: 0, output: 0, totalTokens: 0, cacheRead: 0, cacheWrite: 0, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } };
const tick = () => new Promise((resolve) => setImmediate(resolve));
const userText = (request) => request.messages.filter((message) => message.role === 'user').map((message) => message.content.map((part) => part.text || '').join(''));

function fixture() {
 const requests = [], restored = [];
 const streamFn = (_model, context, { signal }) => {
  const stream = new AssistantMessageEventStream();
  const message = { role: 'assistant', content: [{ type: 'text', text: 'partial answer' }], api: model.api, provider: model.provider, model: model.id, usage, timestamp: Date.now(), stopReason: 'stop' };
  let ended = false;
  const finish = (reason = 'stop') => {
   if (ended) return;
   ended = true; signal.removeEventListener('abort', abort);
   message.stopReason = reason;
   if (reason === 'stop') stream.push({ type: 'done', reason, message });
   else { message.errorMessage = reason; stream.push({ type: 'error', reason, error: message }); }
   stream.end(message);
  };
  const abort = () => finish('aborted');
  signal.addEventListener('abort', abort, { once: true });
  stream.push({ type: 'start', partial: message });
  stream.push({ type: 'text_delta', contentIndex: 0, delta: 'partial answer', partial: message });
  requests.push({ messages: structuredClone(context.messages), finish });
  if (signal.aborted) abort();
  return stream;
 };
 const agent = new Agent({ initialState: { model, systemPrompt: 'helpful', tools: [] }, streamFn });
 const session = new ChatSession({ getAgent: async (existing) => existing || agent, onRestore: (entry) => { restored.push(entry); return true; } });
 const wait = async (count) => {
  for (let index = 0; index < 100 && requests.length < count; index++) await tick();
  assert.equal(requests.length, count);
 };
 return { session, agent, requests, restored, wait };
}

test('follow-up messages wait for the current reply, preserve FIFO, and can be deleted', async () => {
 const { session, requests, wait } = fixture();
 session.enqueue(createChatEntry('first')); await wait(1);
 const removed = createChatEntry('remove me'); session.enqueue(removed);
 session.enqueue(createChatEntry('second')); session.enqueue(createChatEntry('third'));
 session.remove(removed.id);
 assert.equal(requests.length, 1);
 requests[0].finish(); await wait(2);
 assert.deepEqual(userText(requests[1]), ['first', 'second']);
 requests[1].finish(); await wait(3);
 assert.deepEqual(userText(requests[2]), ['first', 'second', 'third']);
 requests[2].finish(); await session.waitForIdle();
 assert.equal(session.queue.length, 0); assert.equal(session.messages.length, 6);
});

test('steering interrupts now, retains partial context, and takes priority over follow-ups', async () => {
 const { session, requests, restored, wait } = fixture();
 session.enqueue(createChatEntry('first')); await wait(1);
 session.enqueue(createChatEntry('ordinary follow-up'));
 const steering = createChatEntry('new direction'); session.enqueue(steering); session.steer(steering.id);
 await wait(2);
 assert.deepEqual(userText(requests[1]), ['first', 'new direction']);
 assert.equal(requests[1].messages.find((message) => message.role === 'assistant').content[0].text, 'partial answer');
 assert.equal(requests[1].messages.find((message) => message.role === 'assistant').stopReason, 'stop');
 assert.equal(restored.length, 0);
 requests[1].finish(); await wait(3);
 assert.deepEqual(userText(requests[2]), ['first', 'new direction', 'ordinary follow-up']);
 requests[2].finish(); await session.waitForIdle();
 assert.match(session.messages[1].status, /guidance/);
});

test('stopping preserves and pauses queued messages until explicit resume', async () => {
 const { session, requests, restored, wait } = fixture();
 session.enqueue(createChatEntry('first')); await wait(1);
 session.enqueue(createChatEntry('later')); session.stop(); await session.waitForIdle();
 assert.equal(requests.length, 1); assert.equal(session.paused, true);
 assert.equal(session.queue[0].prompt, 'later'); assert.equal(restored[0].prompt, 'first');
 session.resume(); await wait(2);
 assert.deepEqual(userText(requests[1]), ['later']);
 requests[1].finish(); await session.waitForIdle();
});

test('editing pauses native queues, updates text without losing attachments, and resumes', async () => {
 const { session, requests, wait } = fixture();
 session.enqueue(createChatEntry('first')); await wait(1);
 const attachment = { kind: 'file', name: 'notes.txt', text: 'document text', size: 10 };
 const entry = createChatEntry('before edit', [attachment]); session.enqueue(entry);
 session.pause(); requests[0].finish(); await session.waitForIdle();
 assert.equal(requests.length, 1);
 session.update(entry.id, 'after edit'); session.resume(); await wait(2);
 assert.match(userText(requests[1]).at(-1), /after edit.*document text/s);
 assert.equal(session.messages[2].attachments[0], attachment);
 requests[1].finish(); await session.waitForIdle();
});

test('a later failed turn rolls back only that turn and holds remaining queue entries', async () => {
 const { session, requests, restored, wait } = fixture();
 session.enqueue(createChatEntry('successful')); await wait(1);
 session.enqueue(createChatEntry('failed')); session.enqueue(createChatEntry('pending'));
 requests[0].finish(); await wait(2); requests[1].finish('error'); await session.waitForIdle();
 assert.equal(session.paused, true); assert.equal(restored[0].prompt, 'failed');
 assert.deepEqual(session.agent.state.messages.filter((m) => m.role === 'user').map((m) => m.content[0].text), ['successful']);
 session.resume(); await wait(3);
 assert.deepEqual(userText(requests[2]), ['successful', 'pending']);
 requests[2].finish(); await session.waitForIdle();
});

test('reset retains pending work as paused and rejects stale callbacks; new chat clears it', async () => {
 const { session, requests, wait } = fixture();
 session.enqueue(createChatEntry('active')); await wait(1);
 session.enqueue(createChatEntry('queued')); const oldRun = session.waitForIdle();
 session.reset(); await oldRun;
 assert.deepEqual(session.queue.map((entry) => entry.prompt), ['active', 'queued']);
 assert.equal(session.paused, true); assert.equal(session.running, false); assert.equal(session.messages.length, 0);
 session.reset(false); assert.equal(session.queue.length, 0); assert.equal(requests.length, 1);
});
