export const MAX_ATTACHMENT_COUNT = 8;
export const MAX_ATTACHMENT_BYTES = 8 * 1024 * 1024;
export const MAX_TOTAL_ATTACHMENT_BYTES = 12 * 1024 * 1024;
export const MAX_FILE_TEXT = 120000;

const imageTypes = { png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', webp: 'image/webp', gif: 'image/gif' };
const extension = (name) => name.toLowerCase().split('.').at(-1);
export const attachmentSize = (size) => size < 1024 * 1024 ? `${Math.max(1, Math.ceil(size / 1024))} KB` : `${(size / (1024 * 1024)).toFixed(1)} MB`;

const attachmentTranslations = {
 '每条消息最多添加 8 个附件': 'Attach up to 8 files per message',
 '单个附件不能超过 8 MB': 'Each attachment must be 8 MB or smaller',
 '本条消息的附件总大小不能超过 12 MB': 'Attachments must total 12 MB or less per message',
 '暂不支持这种二进制文件，请使用文本、代码、PDF 或 DOCX 文件': 'Unsupported binary file. Use text, code, PDF, or DOCX files',
 '该文件包含二进制内容，无法作为文本发送': 'This file contains binary data and cannot be sent as text',
 '此 PDF 没有可提取的文本，请粘贴页面截图': 'This PDF has no extractable text. Paste screenshots of its pages',
 'Word 文档正文过大，请拆分文件后发送': 'This Word document is too large. Split it into smaller files',
 '无法读取 Word 文档正文，请使用 DOCX 格式': 'Cannot read the Word document. Use the DOCX format',
 'Word 文档格式无效': 'Invalid Word document',
 '无法读取图片': 'Cannot read the image',
 '图片文件为空': 'The image file is empty',
 '图片内容无效或格式不支持': 'Invalid or unsupported image',
 '图片仅支持 PNG、JPEG、WebP 和 GIF 格式': 'Supported image formats: PNG, JPEG, WebP, and GIF',
 '每次最多粘贴 8 个文件': 'Paste up to 8 files at a time',
 '附件总大小不能超过 12 MB': 'Attachments must total 12 MB or less',
 '只能粘贴普通文件': 'Only regular files can be pasted',
 '无法读取文件': 'Cannot read the file',
 '剪贴板正被其他程序使用，请再次粘贴': 'The clipboard is busy. Try pasting again',
 '无法读取剪贴板文件': 'Cannot read files from the clipboard',
};
export const attachmentError = (error, language) => {
 let message = error?.message || String(error);
 if (language === 'en') for (const [zh, en] of Object.entries(attachmentTranslations)) message = message.replaceAll(zh, en);
 return message;
};

export function validateAttachmentBatch(files, existing = []) {
 if (existing.length + files.length > MAX_ATTACHMENT_COUNT) throw new Error('每条消息最多添加 8 个附件');
 if (files.some((file) => file.size > MAX_ATTACHMENT_BYTES)) throw new Error('单个附件不能超过 8 MB');
 if ([...existing, ...files].reduce((total, file) => total + file.size, 0) > MAX_TOTAL_ATTACHMENT_BYTES) throw new Error('本条消息的附件总大小不能超过 12 MB');
}

export function decodeTextFile(bytes) {
 let value;
 if (bytes[0] === 0xff && bytes[1] === 0xfe) value = new TextDecoder('utf-16le', { fatal: true }).decode(bytes);
 else if (bytes[0] === 0xfe && bytes[1] === 0xff) value = new TextDecoder('utf-16be', { fatal: true }).decode(bytes);
 else {
  if (bytes.subarray(0, 8192).includes(0) || (bytes[0] === 0x50 && bytes[1] === 0x4b) || (bytes[0] === 0x4d && bytes[1] === 0x5a)) throw new Error('暂不支持这种二进制文件，请使用文本、代码、PDF 或 DOCX 文件');
  try { value = new TextDecoder('utf-8', { fatal: true }).decode(bytes); }
  catch { value = new TextDecoder('gb18030', { fatal: true }).decode(bytes); }
 }
 if (/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(value)) throw new Error('该文件包含二进制内容，无法作为文本发送');
 return value.replace(/^\uFEFF/, '');
}

async function pdfText(bytes) {
 const [pdf, { default: workerURL }] = await Promise.all([
  import('pdfjs-dist/legacy/build/pdf.mjs'), import('pdfjs-dist/legacy/build/pdf.worker.mjs?url'),
 ]);
 pdf.GlobalWorkerOptions.workerSrc = workerURL;
 const task = pdf.getDocument({ data: bytes, isEvalSupported: false, useWasm: false, verbosity: 0 });
 let document;
 try {
  document = await task.promise;
  const pages = [];
  let length = 0;
  let truncated = false;
  for (let index = 1; index <= Math.min(document.numPages, 100); index++) {
   const page = await document.getPage(index);
   const content = await page.getTextContent();
   const value = content.items.map((item) => item.str ? item.str + (item.hasEOL ? '\n' : ' ') : '').join('');
   pages.push(`[Page ${index}]\n${value}`); length += value.length; page.cleanup();
   if (length > MAX_FILE_TEXT) { truncated = true; break; }
  }
  if (!pages.some((page) => page.replace(/^\[Page \d+\]\s*/, '').trim())) throw new Error('此 PDF 没有可提取的文本，请粘贴页面截图');
  return { text: pages.join('\n\n'), truncated: truncated || document.numPages > 100 };
 } finally { await task.destroy(); }
}

async function docxText(bytes) {
 const { unzipSync } = await import('fflate');
 let tooLarge = false;
 const files = unzipSync(bytes, { filter: (entry) => {
  if (entry.name !== 'word/document.xml') return false;
  tooLarge = entry.originalSize > 2 * 1024 * 1024;
  return !tooLarge;
 } });
 if (tooLarge) throw new Error('Word 文档正文过大，请拆分文件后发送');
 if (!files['word/document.xml']) throw new Error('无法读取 Word 文档正文，请使用 DOCX 格式');
 const xml = new DOMParser().parseFromString(new TextDecoder().decode(files['word/document.xml']), 'application/xml');
 if (xml.getElementsByTagName('parsererror').length) throw new Error('Word 文档格式无效');
 const content = (node) => {
  if (node.localName === 't') return node.textContent;
  if (node.localName === 'tab') return '\t';
  if (node.localName === 'br') return '\n';
  return [...node.childNodes].map(content).join('');
 };
 return { text: [...xml.getElementsByTagNameNS('*', 'p')].map(content).join('\n'), truncated: false };
}

const dataURL = (file) => new Promise((resolve, reject) => {
 const reader = new FileReader();
 reader.onload = () => resolve(reader.result);
 reader.onerror = () => reject(new Error('无法读取图片'));
 reader.readAsDataURL(file);
});

export async function readChatAttachment(file) {
 validateAttachmentBatch([file]);
 const item = { id: crypto.randomUUID(), name: file.name || 'clipboard.png', size: file.size };
 const ext = extension(item.name);
 const mimeType = imageTypes[ext] || (Object.values(imageTypes).includes(file.type) ? file.type : '');
 if (mimeType) {
  if (!file.size) throw new Error('图片文件为空');
  const preview = await dataURL(new Blob([file], { type: mimeType }));
  const image = new Image(); image.src = preview;
  await image.decode().catch(() => { throw new Error('图片内容无效或格式不支持'); });
  return { ...item, kind: 'image', mimeType, preview, data: preview.slice(preview.indexOf(',') + 1) };
 }
 if (file.type.startsWith('image/') && ext !== 'svg') throw new Error('图片仅支持 PNG、JPEG、WebP 和 GIF 格式');
 const bytes = new Uint8Array(await file.arrayBuffer());
 let extracted;
 if (ext === 'pdf' || file.type === 'application/pdf') extracted = await pdfText(bytes);
 else if (ext === 'docx') extracted = await docxText(bytes);
 else extracted = { text: decodeTextFile(bytes), truncated: false };
 return { ...item, kind: 'file', text: extracted.text.slice(0, MAX_FILE_TEXT), truncated: extracted.truncated || extracted.text.length > MAX_FILE_TEXT };
}

export function attachmentPrompt(prompt, files) {
 const documents = files.filter((file) => file.kind === 'file');
 const parts = documents.map((file) => `<attached_file name=${JSON.stringify(file.name)}>\n${file.text}${file.truncated ? '\n[Only an excerpt of this file was attached.]' : ''}\n</attached_file>`);
 return [prompt || (files.length ? 'Please review the attached files and images.' : ''), ...parts].join('\n\n');
}

export function attachmentImages(files) {
 return files.filter((file) => file.kind === 'image').map(({ data, mimeType }) => ({ type: 'image', data, mimeType }));
}
