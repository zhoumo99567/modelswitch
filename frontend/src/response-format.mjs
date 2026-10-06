import MarkdownIt from 'markdown-it';

const markdown = new MarkdownIt({ html: false, breaks: true });
const escapeHTML = (value) => markdown.utils.escapeHtml(value);

// Validate with JSON.parse, but format the original tokens so large integers,
// numeric notation, duplicate keys and string escapes retain their exact values.
export function formatJSON(value) {
 const source = value.trim();
 try { JSON.parse(source); } catch { return null; }
 const tokens = source.match(/"(?:\\[\s\S]|[^"\\])*"|[{}\[\],:]|[^\s{}\[\],:]+/g);
 let output = '', depth = 0;
 const newline = () => '\n' + '  '.repeat(Math.min(depth, 128));
 for (let index = 0; index < tokens.length; index++) {
  const token = tokens[index];
  if (token === '{' || token === '[') {
   output += token; depth++;
   if (tokens[index + 1] !== (token === '{' ? '}' : ']')) output += newline();
  } else if (token === '}' || token === ']') {
   depth--;
   if (tokens[index - 1] !== (token === '}' ? '{' : '[')) output += newline();
   output += token;
  } else if (token === ',') output += ',' + newline();
  else if (token === ':') output += ': ';
  else output += token;
 }
 return output;
}

function highlightJSON(source) {
 const pattern = /"(?:\\[\s\S]|[^"\\])*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|\b(?:true|false|null)\b/g;
 let output = '', offset = 0;
 for (const match of source.matchAll(pattern)) {
  const token = match[0];
  const kind = token.startsWith('"') ? (/^\s*:/.test(source.slice(match.index + token.length)) ? 'key' : 'string') : /^-?\d/.test(token) ? 'number' : 'literal';
  output += escapeHTML(source.slice(offset, match.index)) + `<span class="json-${kind}">${escapeHTML(token)}</span>`;
  offset = match.index + token.length;
 }
 return output + escapeHTML(source.slice(offset));
}

function codeBlock(source, language = '') {
 const formatted = language === 'json' ? formatJSON(source) : null;
 const body = language === 'json' ? highlightJSON(formatted ?? source) : escapeHTML(source);
 const label = language ? `<div class="response-code-head">${escapeHTML(language === 'json' ? 'JSON' : language)}</div>` : '';
 return `<div class="response-code">${label}<pre><code${language ? ` class="language-${escapeHTML(language)}"` : ''}>${body}</code></pre></div>\n`;
}

markdown.renderer.rules.fence = (tokens, index) => {
 const token = tokens[index];
 let language = token.info.trim().split(/\s+/)[0].toLowerCase();
 if (!/^[\w+-]{1,40}$/.test(language)) language = '';
 if (!language && /^[\[{]/.test(token.content.trim()) && formatJSON(token.content) !== null) language = 'json';
 return codeBlock(token.content, language);
};
markdown.renderer.rules.table_open = () => '<div class="response-table"><table>\n';
markdown.renderer.rules.table_close = () => '</table></div>\n';
markdown.renderer.rules.link_open = (tokens, index, options, env, renderer) => {
 tokens[index].attrSet('target', '_blank');
 tokens[index].attrSet('rel', 'noopener noreferrer');
 return renderer.renderToken(tokens, index, options);
};

export function renderResponse(value = '') {
 const source = String(value);
 if (/^[\[{]/.test(source.trim()) && formatJSON(source) !== null) return codeBlock(source, 'json');
 return markdown.render(source);
}
