import assert from 'node:assert/strict';
import test from 'node:test';
import { formatJSON, renderResponse } from './response-format.mjs';

test('Markdown renders tables, nested lists and literal inline code', () => {
 const html = renderResponse('# 结果\n\n**成功**，`a_b **literal**`\n\n| 名称 | 状态 |\n| --- | --- |\n| agent | ready |\n\n1. 第一步\n   - 子步骤\n\n> 提示\n\n---');
 assert.match(html, /<h1>结果<\/h1>/);
 assert.match(html, /<strong>成功<\/strong>/);
 assert.match(html, /<code>a_b \*\*literal\*\*<\/code>/);
 assert.match(html, /class="response-table"><table>/);
 assert.match(html, /<th>名称<\/th>/);
 assert.match(html, /<ol>[\s\S]*<li>第一步[\s\S]*<ul>[\s\S]*子步骤/);
 assert.match(html, /<blockquote>/);
 assert.match(html, /<hr>/);
});

test('plain JSON objects and arrays are indented and highlighted', () => {
 const source = '{"name":"对话","enabled":true,"items":[1,{"empty":[],"value":null}]}';
 const formatted = formatJSON(source);
 assert.deepEqual(JSON.parse(formatted), JSON.parse(source));
 assert.match(formatted, /\n  "name": "对话",\n/);
 assert.match(formatted, /"items": \[\n    1,/);
 assert.match(formatted, /"empty": \[\]/);
 const html = renderResponse(source);
 assert.match(html, /class="language-json"/);
 assert.match(html, /class="json-key"/);
 assert.match(html, /class="json-string"/);
 assert.match(html, /class="json-number"/);
 assert.match(html, /class="json-literal"/);
 assert.match(renderResponse('[{"name":"one"},{}]'), /class="language-json"/);
});

test('JSON formatting preserves large integers, duplicate keys and literal notation', () => {
 const source = String.raw`{"id":900719925474099312345,"id":-9007199254740993,"decimal":1.2300e+45,"zero":-0,"text":"a\u0062\"\\ end"}`;
 const formatted = formatJSON(source);
 assert.match(formatted, /900719925474099312345/);
 assert.match(formatted, /-9007199254740993/);
 assert.match(formatted, /1\.2300e\+45/);
 assert.match(formatted, /"zero": -0/);
 assert.equal(formatted.match(/"id"/g).length, 2);
 assert.ok(formatted.includes(String.raw`"a\u0062\"\\ end"`));
});

test('JSON fences are formatted inside Markdown and unlabeled JSON is recognized', () => {
 const html = renderResponse('结果如下：\n\n```json\n{"nested":{"count":2}}\n```\n\n完成。');
 assert.match(html, /<p>结果如下：<\/p>/);
 assert.match(html, /class="language-json"/);
 assert.match(html, /\n    <span class="json-key">/);
 assert.match(html, /<p>完成。<\/p>/);
 assert.match(renderResponse('~~~\n[1,2]\n~~~'), /class="language-json"/);
});

test('incomplete or invalid JSON remains visible until the stream becomes valid', () => {
 assert.equal(formatJSON('{"name":'), null);
 assert.equal(formatJSON('{"name":"a",}'), null);
 const incomplete = renderResponse('```json\n{"name":');
 assert.match(incomplete, /class="language-json"/);
 assert.match(incomplete, /<span class="json-key">&quot;name&quot;<\/span>:/);
 const complete = renderResponse('```json\n{"name":"a"}');
 assert.match(complete, /\n  <span class="json-key">/);
 assert.match(renderResponse('[文档](https://example.com)'), /<a href="https:\/\/example.com"/);
 assert.match(renderResponse('普通文本\n下一行'), /普通文本<br>\n下一行/);
});

test('model HTML, unsafe links and JSON string markup cannot execute', () => {
 const html = renderResponse('<script>alert(1)</script>\n\n[bad](javascript:alert(1))\n\n<img src=x onerror=alert(1)>');
 assert.doesNotMatch(html, /<script|<img|href="javascript:/);
 assert.match(html, /&lt;script&gt;/);
 const json = renderResponse('{"text":"<img src=x onerror=alert(1)>"}');
 assert.doesNotMatch(json, /<img/);
 assert.match(json, /&lt;img src=x onerror=alert\(1\)&gt;/);
 assert.match(renderResponse('[文档](https://example.com)'), /rel="noopener noreferrer"/);
 const code = renderResponse('````html\n<script>**plain**</script>\n```\n````');
 assert.doesNotMatch(code, /<script|<strong>/);
 assert.match(code, /&lt;script&gt;\*\*plain\*\*&lt;\/script&gt;/);
});
