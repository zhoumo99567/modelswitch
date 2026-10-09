import { ClipboardGetText, ClipboardSetText } from '../wailsjs/runtime/runtime.js';

export async function writeClipboardText(text) {
 if (window.runtime?.ClipboardSetText) {
  if (await ClipboardSetText(text) === false) throw new Error('无法写入剪贴板，请重试。');
  return;
 }
 if (navigator.clipboard?.writeText) {
  try { await navigator.clipboard.writeText(text); return; } catch {}
 }
 const focus = document.activeElement;
 const helper = document.createElement('textarea');
 helper.value = text;
 helper.style.position = 'fixed';
 helper.style.opacity = '0';
 document.body.appendChild(helper);
 try {
  helper.select();
  if (!document.execCommand('copy')) throw new Error('无法写入剪贴板，请重试。');
 } finally {
  helper.remove();
  focus?.focus({ preventScroll: true });
 }
}

export async function readClipboardText() {
 try {
  if (window.runtime?.ClipboardGetText) return await ClipboardGetText();
  if (navigator.clipboard?.readText) return await navigator.clipboard.readText();
 } catch {}
 throw new Error('无法读取剪贴板，请允许剪贴板访问后重试。');
}
