import { attachmentImages, attachmentPrompt } from './chat-attachments.mjs';
export const MAX_QUEUED_MESSAGES = 20;

function toolResultText(result) {
 const text = Array.isArray(result?.content) ? result.content.filter((part) => part?.type === 'text').map((part) => part.text || '').join('\n').trim() : '';
 return text.length > 600 ? `${text.slice(0, 600)}…` : text;
}

function toolInputValue(event) {
 const call = event.toolCall || event;
 return call.args ?? call.input ?? call.arguments;
}

export function createChatEntry(prompt, attachments = [], defaultPrompt = 'Please review the attached files and images.') {
 const entry = { id: crypto.randomUUID(), prompt: prompt.trim(), attachments: attachments.slice(), mode: 'followUp', defaultPrompt };
 updateEntryMessage(entry);
 return entry;
}

function updateEntryMessage(entry) {
 entry.message = {
  role: 'user', timestamp: Date.now(),
  content: [{ type: 'text', text: attachmentPrompt(entry.prompt || entry.defaultPrompt, entry.attachments) }, ...attachmentImages(entry.attachments)],
 };
}

// Mirror the SDK queues so users can edit/delete pending input. The SDK owns
// turn scheduling; entries leave the UI queue only when its user event starts.
export class ChatSession {
 constructor({ getAgent, onChange = () => {}, onRestore = () => false, text = (zh, en) => en }) {
  Object.assign(this, { getAgent, onChange, onRestore, text });
  this.messages = []; this.queue = []; this.running = false; this.paused = false;
  this.error = ''; this.notice = ''; this.agent = null; this.active = null;
  this.epoch = 0; this.entries = new WeakMap(); this.runPromise = Promise.resolve();
  this.stopRequested = false; this.interruptRequested = false;
 }

 enqueue(entry) {
  if (this.queue.length >= MAX_QUEUED_MESSAGES) throw new Error(this.text('队列最多保留 20 条消息，请先发送或删除部分消息。', 'The queue holds up to 20 messages. Send or remove some first.'));
  this.entries.set(entry.message, entry);
  this.queue.push(entry); this.error = '';
  this.notice = this.paused ? this.text('已加入队列，队列当前已暂停。', 'Queued. The queue is currently paused.') : this.text('已加入队列，当前回复结束后依次发送。', 'Queued. Messages will be sent in order after the current reply.');
  this.syncQueues(); this.onChange(); this.start();
 }

 syncQueues() {
  this.agent?.clearAllQueues();
  if (!this.agent || this.paused) return;
  for (const entry of this.queue) {
   if (entry.mode === 'steering') this.agent.steer(entry.message);
   else this.agent.followUp(entry.message);
  }
 }

 remove(id) {
  this.queue = this.queue.filter((entry) => entry.id !== id);
  this.syncQueues(); this.onChange();
 }

 update(id, prompt) {
  const entry = this.queue.find((entry) => entry.id === id);
  if (!entry || (!prompt.trim() && !entry.attachments.length)) return false;
  entry.prompt = prompt.trim(); updateEntryMessage(entry); this.entries.set(entry.message, entry);
  this.syncQueues(); this.onChange(); return true;
 }

 moveFirst(id) {
  const entry = this.queue.find((entry) => entry.id === id);
  if (!entry) return;
  this.queue = [entry, ...this.queue.filter((other) => other !== entry)];
  this.syncQueues(); this.onChange();
 }

 steer(id) {
  const entry = this.queue.find((entry) => entry.id === id);
  if (!entry) return;
  this.queue.forEach((other) => { other.mode = 'followUp'; }); entry.mode = 'steering';
  this.queue = [entry, ...this.queue.filter((other) => other !== entry)]; this.paused = false;
  this.syncQueues();
  if (this.agent?.state.isStreaming) {
   this.interruptRequested = true; this.stopRequested = false; this.agent.abort();
  }
  this.notice = this.text('正在引导，优先处理这条消息。', 'Steering. This message will be handled first.');
  this.onChange(); this.start();
 }

 pause() { this.paused = true; this.syncQueues(); this.onChange(); }
 resume() { this.paused = false; this.error = ''; this.syncQueues(); this.onChange(); this.start(); }
 stop() {
  this.stopRequested = true; this.interruptRequested = false;
  this.pause(); this.agent?.abort();
 }

 reset(preserveQueue = true) {
  this.epoch++; this.agent?.clearAllQueues(); this.agent?.abort(); this.agent?.dispose?.();
  if (preserveQueue && this.active && !this.active.completed && !this.queue.some((entry) => entry.id === this.active.entry.id)) {
   this.queue.unshift(this.active.entry);
  }
  if (!preserveQueue) this.queue = [];
  this.agent = null; this.active = null; this.messages = []; this.running = false;
  this.queue.forEach((entry) => { entry.mode = 'followUp'; });
  this.paused = this.queue.length > 0; this.error = ''; this.notice = '';
  this.onChange();
 }

 waitForIdle() { return this.runPromise; }

 start() {
  if (this.running || this.paused || !this.queue.length) return;
  const epoch = this.epoch;
  this.running = true; this.stopRequested = false; this.interruptRequested = false;
  this.error = ''; this.onChange();
  this.runPromise = this.run(epoch, this.runPromise);
 }

 handleEvent(event) {
  if (event.type === 'run_outcome' && this.active) {
   this.active.reply.outcome = event.outcome;
   if (event.outcome.reason === 'recovery_exhausted' && event.outcome.stopReason === 'length') this.active.reply.status = this.text('上下文接近模型上限，已尝试 compact 并续跑 2 次，模型仍未给出最终回复', 'The context limit was reached; after attempting compaction and 2 recovery attempts the model still did not finish');
   else if (event.outcome.reason === 'recovery_exhausted') this.active.reply.status = this.text('工具失败后已自动续跑 2 次，模型仍未给出最终回复', 'The model did not finish after 2 recovery attempts following a tool failure');
   else if (event.outcome.reason === 'tool_terminated') this.active.reply.status = this.text('工具明确结束了本轮执行', 'The tool explicitly ended this run');
   else if (event.outcome.reason === 'output_limit') this.active.reply.status = this.text('上下文接近模型上限，输出被截断；请继续对话或开启新会话', 'The context limit was reached and the output was truncated; continue or start a new chat');
   this.onChange('stream');
  }
  if (event.type === 'input_handled' && this.active) {
   this.active.completed = true;
   this.active.reply.status = this.text('已由扩展处理', 'Handled by extension');
   this.onChange('stream');
  }
  if (event.type.startsWith('tool_execution_') && this.active) {
   const tools = this.active.reply.tools ||= [];
   const now = Date.now();
   if (event.type === 'tool_execution_start' && !this.active.reply.toolsStartedAt) this.active.reply.toolsStartedAt = now;
   let tool = tools.find((tool) => tool.id === event.toolCallId);
   if (!tool) { tool = { id: event.toolCallId, name: event.toolName }; tools.push(tool); }
   const input = toolInputValue(event);
   if (input !== undefined) tool.input = input;
   if (event.toolName && !tool.name) tool.name = event.toolName;
   tool.running = event.type !== 'tool_execution_end'; tool.error = !!event.isError;
   if (event.type === 'tool_execution_end') {
    tool.finishedAt = now;
    tool.durationMs = tool.startedAt ? Math.max(0, now - tool.startedAt) : undefined;
    this.active.reply.toolsFinishedAt = now;
    if (event.result !== undefined) tool.output = event.result;
    if (event.isError) {
     tool.detail = toolResultText(event.result);
     this.active.reply.status = tool.detail ? this.text(`工具调用失败：${tool.detail}`, `Tool call failed: ${tool.detail}`) : this.text('工具调用失败', 'Tool call failed');
    }
   }
   if (event.type === 'tool_execution_start') tool.startedAt ||= now;
   this.onChange('stream');
  }
  if (event.type === 'message_start' && event.message.role === 'user') {
   const entry = this.entries.get(event.message);
   if (!entry) throw new Error('Unknown conversation queue entry');
   this.queue = this.queue.filter((other) => other !== entry);
   const reply = { role: 'assistant', content: '', thinking: '' };
   this.active = { entry, reply, checkpoint: this.agent.state.messages.slice(), completed: false };
   this.messages.push({ role: 'user', content: entry.prompt, attachments: entry.attachments }, reply);
   this.notice = ''; this.onChange('turn');
  }
  if ((event.type === 'message_update' || event.type === 'message_end') && event.message.role === 'assistant' && this.active) {
   const { reply } = this.active;
   reply.content = event.message.content.filter((part) => part.type === 'text').map((part) => part.text).join('');
   reply.thinking = event.message.content.filter((part) => part.type === 'thinking').map((part) => part.thinking).join('');
   if (event.type === 'message_end') {
    // Pi emits an assistant message_end with stopReason=toolUse for every
    // intermediate tool round. It is not the end of the user turn and usually
    // has no text content yet.
    this.active.completed = !['error', 'aborted', 'toolUse'].includes(event.message.stopReason);
    if (event.message.stopReason === 'aborted') reply.status = this.interruptRequested && !this.stopRequested ? this.text('已转为引导', 'Interrupted by guidance') : this.text('已停止生成', 'Generation stopped');
    else if (event.message.stopReason === 'error') reply.status = this.text('发送失败', 'Request failed');
    else if (reply.content) reply.status = '';
    else if (event.message.stopReason !== 'toolUse') reply.status = this.text('模型未返回文本', 'The model returned no text');
   }
   this.onChange('stream');
  }
 }

 restoreFailedTurn(message, stopped) {
  this.error = stopped ? '' : message;
  this.notice = stopped ? this.text('已停止生成，待发送消息已暂停。', 'Generation stopped. Pending messages are paused.') : '';
  this.paused = true;
  if (this.active) {
   this.active.reply.status = stopped ? this.text('已停止生成', 'Generation stopped') : this.text('发送失败', 'Request failed');
   // Keep prior successful turns. Roll back only the current failed turn.
   this.agent.state.messages = this.active.checkpoint;
   this.active.completed = true;
   const entry = this.active.entry; entry.mode = 'followUp';
   if (!this.onRestore(entry)) this.queue.unshift(entry);
  }
  this.syncQueues();
 }

 async run(epoch, previousRun) {
  let unsubscribe;
  try {
   // A reset can replace the agent before the old IPC request has acknowledged
   // cancellation. Its run must settle before the replacement starts a request.
   await previousRun;
   if (epoch !== this.epoch) return;
   const agent = await this.getAgent(this.agent);
   if (epoch !== this.epoch || !agent) return;
   this.agent = agent;
   if (this.stopRequested || !this.queue.length) return;
   unsubscribe = agent.subscribe((event) => { if (epoch === this.epoch) this.handleEvent(event); });
   while (epoch === this.epoch && this.queue.length && !this.paused) {
    this.interruptRequested = false;
    if (agent.state.messages.every((message) => message.role === 'system')) {
     const entry = this.queue.shift(); this.syncQueues();
     await agent.prompt(entry.message);
    } else {
     this.syncQueues(); await agent.continue();
    }
    if (epoch !== this.epoch) return;
    const result = agent.handled ? null : agent.state.messages.at(-1);
    if (result?.stopReason === 'aborted' && this.interruptRequested && !this.stopRequested) {
     // The interrupted prompt and partial answer remain in the conversation.
     // Preserve text as a completed partial message for the next provider call;
     // incomplete reasoning blocks must not be replayed as signed thinking.
     const content = result.content.filter((part) => part.type === 'text' && part.text);
     if (content.length) agent.state.messages = [...agent.state.messages.slice(0, -1), { ...result, content, stopReason: 'stop', errorMessage: undefined }];
     this.notice = this.text('已转为引导。', 'Guidance applied.');
     continue;
    }
    if (['aborted', 'error'].includes(result?.stopReason) || agent.state.errorMessage) {
     this.restoreFailedTurn(result?.errorMessage || agent.state.errorMessage || this.text('模型请求失败', 'Model request failed'), result?.stopReason === 'aborted');
     break;
    }
    this.notice = this.active?.reply.status || this.text('模型已回复。', 'The model has replied.');
   }
  } catch (error) {
   if (epoch === this.epoch) this.restoreFailedTurn(error?.message || String(error), this.stopRequested);
  } finally {
   unsubscribe?.();
   if (epoch === this.epoch) { this.running = false; this.active = null; this.onChange(); }
  }
 }
}
