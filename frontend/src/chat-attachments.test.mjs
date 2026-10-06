import assert from 'node:assert/strict';
import test from 'node:test';
import { attachmentImages, attachmentPrompt, decodeTextFile, validateAttachmentBatch, MAX_ATTACHMENT_BYTES, MAX_TOTAL_ATTACHMENT_BYTES } from './chat-attachments.mjs';

test('clipboard text preserves Unicode, UTF-16, and Chinese Windows encodings', () => {
 assert.equal(decodeTextFile(new TextEncoder().encode('你好\nconst x = 1;')), '你好\nconst x = 1;');
 assert.equal(decodeTextFile(Uint8Array.from([255, 254, 96, 79, 125, 89])), '你好');
 assert.equal(decodeTextFile(Uint8Array.from([196, 227, 186, 195])), '你好');
});

test('unknown binary formats are rejected instead of being sent as garbled text', () => {
 for (const bytes of [[0, 1, 2], [80, 75, 3, 4], [77, 90, 65], [65, 1, 66]]) assert.throws(() => decodeTextFile(Uint8Array.from(bytes)));
});

test('draft limits include files already attached', () => {
 validateAttachmentBatch([{ size: MAX_ATTACHMENT_BYTES }]);
 assert.throws(() => validateAttachmentBatch([{ size: MAX_ATTACHMENT_BYTES + 1 }]));
 assert.throws(() => validateAttachmentBatch([{ size: 1 }], Array.from({ length: 8 }, () => ({ size: 1 }))));
 assert.throws(() => validateAttachmentBatch([{ size: 7 << 20 }], [{ size: MAX_TOTAL_ATTACHMENT_BYTES - (6 << 20) }]));
});

test('documents become text while images use native agent image content', () => {
 const files = [{ kind: 'file', name: 'notes.txt', text: 'source text', truncated: true }, { kind: 'image', data: 'AQID', mimeType: 'image/png' }];
 const prompt = attachmentPrompt('Summarize', files);
 assert.match(prompt, /Summarize.*<attached_file name="notes.txt">.*source text/s);
 assert.match(prompt, /Only an excerpt/);
 assert.deepEqual(attachmentImages(files), [{ type: 'image', data: 'AQID', mimeType: 'image/png' }]);
 assert.match(attachmentPrompt('', files), /Please review/);
});
