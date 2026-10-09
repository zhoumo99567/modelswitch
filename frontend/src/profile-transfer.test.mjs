import assert from 'node:assert/strict';
import test from 'node:test';
import { parseProfile, serializeProfile } from './profile-transfer.mjs';

const DEFAULT_CONTEXT_WINDOW = 256 * 1024;

test('copy and paste round-trip Unicode connection settings, exact key, and multiple model capabilities', () => {
 const draft = {
  id: 'local-only-id', name: '本地模型 · 中文 🔑', baseUrl: 'https://api.example.test/自定义/v1',
  apiKey: '  key with "quotes", \'single\', 中文\nkeep whitespace  ', selectedModel: '视觉模型/α',
  hasApiKey: true, clearApiKey: false, active: true,
  models: [
   { id: '视觉模型/α', owned_by: '团队 A', supportsImages: true, contextWindow: 131072, unrelated: 'omit' },
   { id: '文本模型/β', owned_by: '团队 B', supportsImages: false, contextWindow: 786432 },
  ],
 };
 const before = structuredClone(draft);
 const text = serializeProfile(draft);
 assert.match(text, /^\{\n  "name":/);
 const copied = JSON.parse(text);
 assert.deepEqual(Object.keys(copied), ['name', 'baseUrl', 'apiKey', 'selectedModel', 'models']);
 assert.deepEqual(copied.models.map((model) => Object.keys(model)), [
  ['id', 'owned_by', 'supportsImages', 'contextWindow'],
  ['id', 'owned_by', 'supportsImages', 'contextWindow'],
 ]);
 const pasted = parseProfile(text);
 assert.deepEqual(pasted, { id: '', ...copied, clearApiKey: false, hasApiKey: true });
 assert.equal(pasted.apiKey, draft.apiKey);
 assert.equal(pasted.models[0].supportsImages, true);
 assert.equal(pasted.models[1].contextWindow, 786432);
 assert.deepEqual(draft, before, 'copying must not change the saved profile or source draft');
 pasted.models[0].contextWindow = 524288;
 assert.equal(draft.models[0].contextWindow, 131072, 'pasting must create independent model objects');
});

test('copy honors removal of a saved key while leaving the source draft untouched', () => {
 const draft = { name: 'remove key', apiKey: 'saved-key', clearApiKey: true, hasApiKey: true };
 const copied = JSON.parse(serializeProfile(draft));
 assert.equal(copied.apiKey, '');
 assert.equal(draft.apiKey, 'saved-key');
 const pasted = parseProfile(JSON.stringify(copied));
 assert.equal(pasted.hasApiKey, false);
 assert.equal(pasted.clearApiKey, false);
});

test('paste accepts ProfileView but strips local ID, hidden secret, and stored status', () => {
 const pasted = parseProfile(JSON.stringify({
  id: 'existing-id', name: 'service', baseUrl: 'http://localhost:8080/v1', apiKey: 'visible-key',
  selectedModel: 'model-a', models: [{ id: 'model-a', supportsImages: true, contextWindow: 262144 }],
  hasApiKey: false, clearApiKey: true, secret: 'legacy-secret', activeProfileId: 'existing-id',
 }));
 assert.equal(pasted.id, '');
 assert.equal(pasted.apiKey, 'visible-key');
 assert.equal(pasted.hasApiKey, true);
 assert.equal(pasted.clearApiKey, false);
 assert.equal('secret' in pasted, false);
 assert.equal('activeProfileId' in pasted, false);
 assert.deepEqual(Object.keys(pasted), ['id', 'name', 'baseUrl', 'apiKey', 'selectedModel', 'models', 'clearApiKey', 'hasApiKey']);
});

test('legacy models get default context and a selected model missing from the list is added', () => {
 const pasted = parseProfile(JSON.stringify({
  name: 'older profile', selectedModel: 'missing-selected',
  models: [{ id: 'old-model', owned_by: 'legacy', supportsImages: true }, { id: 'unset-context', contextWindow: 0 }],
 }));
 assert.deepEqual(pasted.models, [
  { id: 'old-model', owned_by: 'legacy', supportsImages: true, contextWindow: DEFAULT_CONTEXT_WINDOW },
  { id: 'unset-context', owned_by: '', supportsImages: false, contextWindow: DEFAULT_CONTEXT_WINDOW },
  { id: 'missing-selected', owned_by: '', supportsImages: false, contextWindow: DEFAULT_CONTEXT_WINDOW },
 ]);
 assert.equal(pasted.apiKey, '');
 assert.equal(pasted.hasApiKey, false);
});

test('incomplete but recognizable drafts remain editable without requiring URL, key, or selected model', () => {
 assert.deepEqual(parseProfile('{"name":"draft only"}'), {
  id: '', name: 'draft only', baseUrl: '', apiKey: '', selectedModel: '', models: [], clearApiKey: false, hasApiKey: false,
 });
 const emptyDraft = { id: '', name: '', baseUrl: '', apiKey: '', selectedModel: '', models: [], clearApiKey: false, hasApiKey: false };
 assert.deepEqual(parseProfile(serializeProfile(emptyDraft)), emptyDraft);
 const selectedOnly = parseProfile('{"selectedModel":"manual-model"}');
 assert.deepEqual(selectedOnly.models, [{ id: 'manual-model', owned_by: '', supportsImages: false, contextWindow: DEFAULT_CONTEXT_WINDOW }]);
});

test('empty clipboard and invalid JSON have distinct actionable errors', () => {
 for (const text of ['', '  \n\t ']) {
  assert.throws(() => parseProfile(text), { code: 'PROFILE_EMPTY', message: '剪贴板为空，请先复制一个配置' });
 }
 for (const text of ['not json', '{"name":', '```json\n{"name":"profile"}\n```']) {
  assert.throws(() => parseProfile(text), { code: 'PROFILE_JSON', message: '配置不是有效的 JSON，请检查复制内容' });
 }
 assert.throws(() => parseProfile(undefined), { code: 'PROFILE_TEXT' });
});

test('paste rejects full stores, lists, primitive values, and unrelated objects', () => {
 for (const value of [[], [{ name: 'one' }], { profiles: [{ name: 'one' }] }, { name: 'not a single profile', profiles: [] }]) {
  assert.throws(() => parseProfile(JSON.stringify(value)), { code: 'PROFILE_SINGLE_ONLY', message: '请粘贴单个配置，不支持配置列表或 profiles.json' });
 }
 for (const value of [null, true, 3, 'profile']) {
  assert.throws(() => parseProfile(JSON.stringify(value)), { code: 'PROFILE_OBJECT' });
 }
 for (const value of [{}, { message: 'hello' }, { id: 'local-id', hasApiKey: true }, { first: { name: 'one' }, second: { name: 'two' } }]) {
  assert.throws(() => parseProfile(JSON.stringify(value)), { code: 'PROFILE_UNRECOGNIZED' });
 }
});

test('typed profile and model fields reject lossy coercion with a precise field path', () => {
 const cases = [
  [{ name: 12 }, 'PROFILE_FIELD_TYPE', 'name'],
  [{ baseUrl: [] }, 'PROFILE_FIELD_TYPE', 'baseUrl'],
  [{ apiKey: false }, 'PROFILE_FIELD_TYPE', 'apiKey'],
  [{ selectedModel: null }, 'PROFILE_FIELD_TYPE', 'selectedModel'],
  [{ models: {} }, 'PROFILE_MODELS_ARRAY', 'models'],
  [{ models: [null] }, 'PROFILE_MODEL_OBJECT', 'models[0]'],
  [{ models: [{}] }, 'PROFILE_MODEL_ID', 'models[0].id'],
  [{ models: [{ id: '   ' }] }, 'PROFILE_MODEL_ID', 'models[0].id'],
  [{ models: [{ id: 'a', owned_by: 7 }] }, 'PROFILE_FIELD_TYPE', 'models[0].owned_by'],
  [{ models: [{ id: 'a', supportsImages: 'true' }] }, 'PROFILE_FIELD_TYPE', 'models[0].supportsImages'],
  [{ models: [{ id: 'a', contextWindow: '256K' }] }, 'PROFILE_CONTEXT_WINDOW', 'models[0].contextWindow'],
  [{ models: [{ id: 'a', contextWindow: -1 }] }, 'PROFILE_CONTEXT_WINDOW', 'models[0].contextWindow'],
  [{ models: [{ id: 'a', contextWindow: 1.5 }] }, 'PROFILE_CONTEXT_WINDOW', 'models[0].contextWindow'],
  [{ models: [{ id: 'a', contextWindow: Number.MAX_SAFE_INTEGER + 1 }] }, 'PROFILE_CONTEXT_WINDOW', 'models[0].contextWindow'],
 ];
 for (const [value, code, field] of cases) {
  assert.throws(() => parseProfile(JSON.stringify(value)), { code, field });
  assert.throws(() => serializeProfile(value), { code, field });
 }
});
