const DEFAULT_CONTEXT_WINDOW = 256 * 1024;
const CONFIG_FIELDS = ['name', 'baseUrl', 'apiKey', 'selectedModel', 'models'];
const owns = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
const isObject = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);

function fail(code, message, details = {}) {
 const error = new Error(message);
 Object.assign(error, { code, ...details });
 throw error;
}

function stringField(value, field) {
 if (!owns(value, field)) return '';
 if (typeof value[field] !== 'string') fail('PROFILE_FIELD_TYPE', `${field} 必须是字符串`, { field });
 return value[field];
}

function normalizeModel(model, index) {
 const field = `models[${index}]`;
 if (!isObject(model)) fail('PROFILE_MODEL_OBJECT', `第 ${index + 1} 个模型必须是对象`, { field, modelIndex: index });
 if (typeof model.id !== 'string' || !model.id.trim()) fail('PROFILE_MODEL_ID', `第 ${index + 1} 个模型必须包含非空字符串 id`, { field: `${field}.id`, modelIndex: index });
 if (owns(model, 'owned_by') && typeof model.owned_by !== 'string') fail('PROFILE_FIELD_TYPE', `${field}.owned_by 必须是字符串`, { field: `${field}.owned_by` });
 if (owns(model, 'supportsImages') && typeof model.supportsImages !== 'boolean') fail('PROFILE_FIELD_TYPE', `${field}.supportsImages 必须是布尔值`, { field: `${field}.supportsImages` });
 let contextWindow = DEFAULT_CONTEXT_WINDOW;
 if (owns(model, 'contextWindow')) {
  if (!Number.isSafeInteger(model.contextWindow) || model.contextWindow < 0) fail('PROFILE_CONTEXT_WINDOW', `${field}.contextWindow 必须是正整数，未设置时可省略或填写 0`, { field: `${field}.contextWindow` });
  if (model.contextWindow > 0) contextWindow = model.contextWindow;
 }
 return { id: model.id, owned_by: model.owned_by ?? '', supportsImages: model.supportsImages ?? false, contextWindow };
}

function normalizeProfile(profile) {
 if (Array.isArray(profile) || (isObject(profile) && owns(profile, 'profiles'))) fail('PROFILE_SINGLE_ONLY', '请粘贴单个配置，不支持配置列表或 profiles.json');
 if (!isObject(profile)) fail('PROFILE_OBJECT', '配置必须是 JSON 对象');
 if (!CONFIG_FIELDS.some((field) => owns(profile, field))) fail('PROFILE_UNRECOGNIZED', 'JSON 中没有模型配置字段，请复制一个模型配置');
 const name = stringField(profile, 'name');
 const baseUrl = stringField(profile, 'baseUrl');
 const apiKey = stringField(profile, 'apiKey');
 const selectedModel = stringField(profile, 'selectedModel');
 if (owns(profile, 'models') && !Array.isArray(profile.models)) fail('PROFILE_MODELS_ARRAY', 'models 必须是模型数组', { field: 'models' });
 const models = (profile.models ?? []).map(normalizeModel);
 if (selectedModel && !models.some((model) => model.id === selectedModel)) {
  models.push({ id: selectedModel, owned_by: '', supportsImages: false, contextWindow: DEFAULT_CONTEXT_WINDOW });
 }
 return { name, baseUrl, apiKey, selectedModel, models };
}

// Clipboard data contains connection settings only. Local identity and runtime
// status never travel with a copied profile, and the source draft is untouched.
export function serializeProfile(profile) {
 const transferable = normalizeProfile(profile);
 if (profile.clearApiKey === true) transferable.apiKey = '';
 return JSON.stringify(transferable, null, 2);
}

export function parseProfile(text) {
 if (typeof text !== 'string') fail('PROFILE_TEXT', '配置内容必须是文本');
 if (!text.trim()) fail('PROFILE_EMPTY', '剪贴板为空，请先复制一个配置');
 let parsed;
 try {
  parsed = JSON.parse(text);
 } catch {
  fail('PROFILE_JSON', '配置不是有效的 JSON，请检查复制内容');
 }
 const profile = normalizeProfile(parsed);
 return { id: '', ...profile, clearApiKey: false, hasApiKey: Boolean(profile.apiKey) };
}
