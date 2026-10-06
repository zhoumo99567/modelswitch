// Agent-compatible facade for the Node Pi AgentSession. The UI owns editable
// pending input; each prompt runs through Pi's skills and extension lifecycle.
export function requirePiRuntimeBindings(app, message = 'The Pi conversation backend is out of date. Restart the app; in development, restart wails dev.') {
 if (['OpenPiChatRuntime', 'PiChatRuntimeCommand', 'ClosePiChatRuntime'].some((name) => typeof app?.[name] !== 'function')) throw new Error(message);
}

export class PiRuntimeAgent {
 constructor({ id, configID, open, command, close, eventsOn, onRuntimeEvent = () => {} }) {
  Object.assign(this, { id, configID, open, command, close, onRuntimeEvent });
  this.state = { messages: [], isStreaming: false, errorMessage: '', closed: false };
  this.listeners = new Set(); this.queue = []; this.pending = null; this.handled = false;
  this.unsubscribe = eventsOn('pi-chat-runtime', (packet) => {
   if (packet.sessionID === this.id) this.receive(packet.event);
  });
 }
 async initialize() { this.info = await this.open(this.id, this.configID); return this; }
 subscribe(listener) { this.listeners.add(listener); return () => this.listeners.delete(listener); }
 emit(event) { for (const listener of this.listeners) listener(event); }
 async send(input) { return this.command(this.id, JSON.stringify(input)); }
 clearAllQueues() { this.queue = []; }
 steer(message) { this.queue.push({ message, mode: 'steering' }); }
 followUp(message) { this.queue.push({ message, mode: 'followUp' }); }
 continue() {
  const item = this.queue.find((item) => item.mode === 'steering') || this.queue[0];
  if (!item) return Promise.resolve();
  this.queue = this.queue.filter((other) => other !== item);
  return this.prompt(item.message);
 }
 prompt(message) {
  if (this.pending) return Promise.reject(new Error('Pi is already running'));
  if (this.state.closed) return Promise.reject(new Error('Pi runtime has closed'));
  const id = crypto.randomUUID();
  this.state.isStreaming = true; this.state.errorMessage = ''; this.handled = false;
  // Retain the original entry's identity even when an extension rewrites input
  // or consumes a slash command before Pi emits a user message.
  this.emit({ type: 'message_start', message });
  return new Promise((resolve, reject) => {
   this.pending = { id, resolve, reject };
   void this.send({ type: 'run', id, message, history: this.state.messages }).catch((error) => this.finish(error));
  });
 }
 abort() { if (this.pending) void this.send({ type: 'abort' }).catch((error) => this.finish(error)); }
 dispose() {
  this.unsubscribe?.(); this.unsubscribe = null; this.state.closed = true;
  this.finish(new Error('Pi runtime disposed'));
  // Generated Wails wrappers can throw before returning a Promise when a dev
  // process still has the old Go bindings. Cleanup must retain the first error.
  void Promise.resolve().then(() => this.close(this.id)).catch(() => {});
 }
 finish(error) {
  const pending = this.pending; this.pending = null; this.state.isStreaming = false;
  if (error) this.state.errorMessage = error.message || String(error);
  if (pending) { if (error) pending.reject(error); else pending.resolve(); }
 }
 receive(record) {
  if (record.type === 'closed') {
   this.state.closed = true; this.finish(new Error(record.message)); this.onRuntimeEvent(record, this); return;
  }
  if (record.type === 'event') {
   if (!this.pending || record.runID !== this.pending.id) return;
   const event = record.event;
   if (event.type === 'message_end') this.state.messages.push(event.message);
   if (event.message?.role === 'user') return;
   this.emit(event); return;
  }
  if (record.type === 'done' || record.type === 'failed') {
   if (!this.pending || record.runID !== this.pending.id) return;
   if (record.messages) this.state.messages = record.messages;
   this.handled = record.disposition === 'handled';
   if (this.handled) this.emit({ type: 'input_handled' });
   this.finish(record.type === 'failed' ? new Error(record.message) : undefined); return;
  }
  this.onRuntimeEvent(record, this);
 }
}
