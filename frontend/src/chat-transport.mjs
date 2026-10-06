// Give pi-ai a fetch-compatible stream over Wails IPC. This preserves its
// provider parsing and abort behavior without exposing keys or requiring CORS.
export function wailsChatFetch(configId, start, cancel, subscribe) {
 let lastReleased = Promise.resolve();
 const fetch = async (_url, options) => new Promise((resolve, reject) => {
  const id = crypto.randomUUID();
  const signal = options.signal;
  let controller, settled = false, finished = false, bodyCancelled = false;
  const abortError = () => new DOMException('Generation stopped', 'AbortError');
  const abort = () => { void cancel(id).catch(() => {}); };
  let release;
  const released = new Promise((resolveRelease) => { release = resolveRelease; });
  lastReleased = released;
  const cleanup = () => { finished = true; unsubscribe(); signal?.removeEventListener('abort', abort); release(); };
  const fail = (message) => {
   if (finished) return;
   cleanup();
   const err = signal?.aborted ? abortError() : new Error(message);
   if (settled) { if (!bodyCancelled) controller.error(err); }
   else if (signal?.aborted) reject(err);
   else resolve(new Response(JSON.stringify({ error: { message } }), { status: 400, headers: { 'Content-Type': 'application/json' } }));
  };
  const body = new ReadableStream({
   start(streamController) { controller = streamController; },
   async cancel() {
    if (finished) return;
    bodyCancelled = true;
    // The SDK closes its reader at [DONE] or response.completed. Wait for Go's
    // terminal event, emitted after it releases the request slot, before it
    // starts a queued turn. Cancellation alone only initiates that release.
    try { await cancel(id); } catch (err) { fail(err?.message || String(err)); }
    await released;
   },
  });
  const unsubscribe = subscribe('pi-chat-stream', (event) => {
   if (event.id !== id || finished) return;
   if (event.type === 'headers') {
    settled = true;
    resolve(new Response(body, { status: event.status, headers: { 'Content-Type': event.contentType || 'text/event-stream' } }));
   } else if (event.type === 'chunk') {
    if (!bodyCancelled) controller.enqueue(Uint8Array.from(atob(event.data), (char) => char.charCodeAt(0)));
   } else if (event.type === 'error') fail(event.message);
   else if (event.type === 'end') {
    if (!settled) { fail('模型服务没有返回响应'); return; }
    cleanup(); if (!bodyCancelled) controller.close();
   }
  });
  signal?.addEventListener('abort', abort, { once: true });
  if (signal?.aborted) { cleanup(); reject(abortError()); return; }
  start({ id, configId, body: options.body }).then(() => {
   // Abort can arrive while the IPC invocation is being registered in Go.
   if (signal?.aborted) abort();
  }).catch((err) => fail(err?.message || String(err)));
 });
 fetch.waitForIdle = () => lastReleased;
 return fetch;
}

// An SDK abort can finish before fetch receives response headers. Gate its
// terminal event as well as reader cancellation on the backend acknowledgement.
export function waitForChatTransport(stream, fetch) {
 return {
  async *[Symbol.asyncIterator]() {
   try {
    for await (const event of stream) {
     if (event.type === 'done' || event.type === 'error') await fetch.waitForIdle();
     yield event;
    }
   } finally { await fetch.waitForIdle(); }
  },
  async result() { const result = await stream.result(); await fetch.waitForIdle(); return result; },
 };
}
