import assert from 'node:assert/strict';
import test from 'node:test';
import { waitForChatTransport, wailsChatFetch } from './chat-transport.mjs';

test('reader cancellation waits for the backend to release its request slot', async () => {
 let listener, request, acknowledge, unsubscribed = false;
 const fetch = wailsChatFetch('current', async (input) => {
  request = input;
  listener({ id: request.id, type: 'headers', status: 200 });
 }, async (id) => {
  assert.equal(id, request.id);
  acknowledge = () => listener({ id, type: 'error', message: 'context canceled' });
 }, (_name, callback) => { listener = callback; return () => { unsubscribed = true; }; });
 const response = await fetch('ignored', { body: '{}', signal: new AbortController().signal });
 const reader = response.body.getReader();
 let cancelled = false;
 const pending = reader.cancel().then(() => { cancelled = true; });
 await new Promise((resolve) => setImmediate(resolve));
 assert.equal(cancelled, false); assert.equal(unsubscribed, false);
 acknowledge(); await pending;
 assert.equal(cancelled, true); assert.equal(unsubscribed, true);
});

test('an SDK abort before response headers waits for the backend acknowledgement', async () => {
 let listener, request, acknowledge;
 const fetch = wailsChatFetch('current', async (input) => { request = input; }, async (id) => {
  acknowledge = () => listener({ id, type: 'error', message: 'context canceled' });
 }, (_name, callback) => { listener = callback; return () => {}; });
 const controller = new AbortController();
 const response = fetch('ignored', { body: '{}', signal: controller.signal });
 controller.abort(); assert.ok(request); assert.ok(acknowledge);
 const result = { stopReason: 'aborted' };
 const stream = waitForChatTransport({
  async *[Symbol.asyncIterator]() { yield { type: 'error', reason: 'aborted', error: result }; },
  async result() { return result; },
 }, fetch);
 let completed = false;
 const next = stream[Symbol.asyncIterator]().next().then((event) => { completed = true; return event; });
 await new Promise((resolve) => setImmediate(resolve));
 assert.equal(completed, false);
 acknowledge(); await response.catch(() => {});
 assert.equal((await next).value.reason, 'aborted');
 assert.equal(await stream.result(), result);
});

test('normal transport completion releases the reader without another cancel call', async () => {
 let listener, cancelCount = 0;
 const fetch = wailsChatFetch('current', async ({ id }) => {
  listener({ id, type: 'headers', status: 200 });
  listener({ id, type: 'chunk', data: btoa('data: [DONE]\n\n') });
  listener({ id, type: 'end' });
 }, async () => { cancelCount++; }, (_name, callback) => { listener = callback; return () => {}; });
 const response = await fetch('ignored', { body: '{}', signal: new AbortController().signal });
 const reader = response.body.getReader();
 assert.match(new TextDecoder().decode((await reader.read()).value), /\[DONE\]/);
 await reader.cancel(); assert.equal(cancelCount, 0);
});

test('late chunks and a normal terminal event after reader cancellation are ignored safely', async () => {
 let listener, request;
 const fetch = wailsChatFetch('current', async (input) => {
  request = input; listener({ id: request.id, type: 'headers', status: 200 });
 }, async () => {}, (_name, callback) => { listener = callback; return () => {}; });
 const response = await fetch('ignored', { body: '{}', signal: new AbortController().signal });
 const pending = response.body.getReader().cancel();
 await new Promise((resolve) => setImmediate(resolve));
 assert.doesNotThrow(() => listener({ id: request.id, type: 'chunk', data: btoa('late bytes') }));
 assert.doesNotThrow(() => listener({ id: request.id, type: 'end' }));
 await pending;
});
