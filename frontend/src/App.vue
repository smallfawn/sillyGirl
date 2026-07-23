<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { Compartment, EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { javascript } from '@codemirror/lang-javascript';
import { oneDark } from '@codemirror/theme-one-dark';
import { basicSetup } from 'codemirror';
import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Layout,
  Menu,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'ant-design-vue';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import {
  Bot,
  Boxes,
  ClipboardList,
  Database,
  Download,
  Edit3,
  FileCode2,
  FolderOpen,
  Home,
  LogOut,
  MessageSquare,
  Package,
  Play,
  Plug,
  Plus,
  Radio,
  RefreshCw,
  Save,
  Search,
  Server,
  Settings,
  ShieldCheck,
  Trash2,
  User,
  Wand2,
} from 'lucide-vue-next';
import { ApiError, clearAuthToken, del, get, post, put, readStorage, saveStorage, setAuthToken } from './api';
import type { CarryGroup, CurrentUser, Master, PluginInfo, QinglongPanel, Reply, SmallcatPanel, Task } from './types';
import { asArray, splitTags, timestamp } from './utils';

type PageKey =
  | 'welcome'
  | 'scripts'
  | 'dependencies'
  | 'plugins'
  | 'storage'
  | 'reply'
  | 'tasks'
  | 'carry'
  | 'qinglong'
  | 'smallcat'
  | 'masters'
  | 'messages'
  | 'plugin-configs'
  | 'settings';

const starter = `/**
 * @title 新脚本
 * @rule raw ^ping$
 * @version v1.0.0
 * @author 自定义
 */

s.reply("pong");
`;

const user = ref<CurrentUser | null>(null);
const booting = ref(true);
const page = ref<PageKey>(pageFromPath());
const selectedScriptId = ref(scriptIdFromPath());
const loginModel = reactive({ username: 'admin', password: '' });
const setupRequired = ref(false);
const setupModel = reactive({ username: 'admin', password: '', confirm: '' });

type AuthResponse = {
  success?: boolean;
  status: string;
  token?: string;
  expiresIn?: number;
};

const scripts = computed(() => user.value?.plugins || []);
const realScripts = computed(() => scripts.value.filter((item) => item.path?.startsWith('/script/') && !item.name?.startsWith('+')));
const scriptKeyword = ref('');
const overviewAdapters = computed(() => {
  const defaults = [
    { platform: 'pagermaid', label: 'Pagermaid' },
    { platform: 'qq', label: 'QQ' },
    { platform: 'web', label: 'Web' },
    { platform: 'telegram', label: 'Telegram Bot' },
  ];
  const rows = new Map((user.value?.adapters || []).map((item) => [item.platform, item]));
  return defaults.map((item) => {
    const row = rows.get(item.platform);
    return {
      platform: item.platform,
      label: row?.label || item.label,
      online: !!row?.online,
      bots_id: row?.bots_id || [],
      count: row?.count || 0,
    };
  });
});
const overviewIntegrations = computed(() => {
  const defaults = [
    { key: 'qinglong', label: '青龙容器' },
    { key: 'smallcat', label: 'smallcat' },
  ];
  const rows = user.value?.integrations || {};
  return defaults.map((item) => {
    const row = rows[item.key];
    return {
      key: item.key,
      label: row?.label || item.label,
      count: row?.count || 0,
      online_count: row?.online_count || 0,
      online: !!row?.online,
    };
  });
});
const overviewVersion = computed(() => {
  const info = user.value?.version || {};
  return {
    local: info.local || 'dev',
    remote: info.remote || '待发布',
    source: info.source || 'reserved',
    repository: info.repository || 'https://github.com/smallfawn/sillyGirl',
  };
});

const menuItems = [
  { key: 'welcome', label: '概览', icon: () => h(Home, { size: 16 }) },
  { key: 'scripts', label: '脚本插件', icon: () => h(Bot, { size: 16 }) },
  { key: 'dependencies', label: '依赖管理', icon: () => h(Package, { size: 16 }) },
  { key: 'plugins', label: '插件市场', icon: () => h(Plug, { size: 16 }) },
  { key: 'plugin-configs', label: '插件配置', icon: () => h(Boxes, { size: 16 }) },
  { key: 'storage', label: '存储', icon: () => h(Database, { size: 16 }) },
  { key: 'reply', label: '回复', icon: () => h(MessageSquare, { size: 16 }) },
  { key: 'tasks', label: '定时任务', icon: () => h(ClipboardList, { size: 16 }) },
  { key: 'carry', label: '搬运', icon: () => h(Radio, { size: 16 }) },
  { key: 'qinglong', label: '青龙容器', icon: () => h(Server, { size: 16 }) },
  { key: 'smallcat', label: 'smallcat', icon: () => h(Server, { size: 16 }) },
  { key: 'masters', label: '管理员', icon: () => h(ShieldCheck, { size: 16 }) },
  { key: 'messages', label: '消息控制', icon: () => h(Boxes, { size: 16 }) },
  { key: 'settings', label: '基础设置', icon: () => h(Settings, { size: 16 }) },
];

function pageFromPath(): PageKey {
  const path = window.location.pathname.replace(/^\/admin\/?/, '/');
  if (path.startsWith('/script/')) return 'scripts';
  return ((path.split('/').filter(Boolean)[0] as PageKey) || 'welcome') as PageKey;
}

function scriptIdFromPath() {
  return window.location.pathname.match(/\/script\/([^/]+)/)?.[1];
}

function maskSecret(value?: string) {
  const text = `${value || ''}`.trim();
  if (!text) return '-';
  if (text.length <= 10) return '***';
  return `${text.slice(0, 4)}...${text.slice(-4)}`;
}

function navigate(next: PageKey, path?: string) {
  const url = path || `/admin/${next === 'welcome' ? '' : next}`;
  window.history.pushState({}, '', url);
  page.value = next;
  selectedScriptId.value = scriptIdFromPath();
}

async function loadSetupStatus() {
  const res = await get<{ success: boolean; data: { initialized: boolean } }>('/api/setup/status');
  setupRequired.value = !res.data?.initialized;
  return !!res.data?.initialized;
}

async function loadUser(setBooting = true) {
  if (setBooting) booting.value = true;
  try {
    const res = await get<{ success: boolean; data: CurrentUser }>('/api/currentUser');
    user.value = res.data || {};
    setupRequired.value = false;
  } catch (error) {
    if (error instanceof ApiError && error.status !== 401) message.error(error.message);
    user.value = null;
    if (error instanceof ApiError && error.status === 401) {
      clearAuthToken();
      await loadSetupStatus().catch(() => undefined);
    }
  } finally {
    if (setBooting) booting.value = false;
  }
}

async function login() {
  try {
    const res = await post<AuthResponse>('/api/login/account', loginModel);
    if (res.status === 'setup_required') {
      setupRequired.value = true;
      message.error('请先设置管理员账号和密码');
      return;
    }
    if (res.status !== 'ok' || !res.token) {
      message.error('账号或密码不正确');
      return;
    }
    setAuthToken(res.token);
    message.success('已登录');
    await loadUser();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '登录失败');
  }
}

async function setupAdmin() {
  if (!setupModel.username.trim()) {
    message.error('账号不能为空');
    return;
  }
  if (!setupModel.password.trim()) {
    message.error('密码不能为空');
    return;
  }
  if (setupModel.password !== setupModel.confirm) {
    message.error('两次输入的密码不一致');
    return;
  }
  try {
    const res = await post<AuthResponse>('/api/setup/admin', { username: setupModel.username.trim(), password: setupModel.password });
    if (res.status !== 'ok' || !res.token) {
      message.error('账号创建失败');
      return;
    }
    setAuthToken(res.token);
    message.success('账号已创建');
    setupRequired.value = false;
    loginModel.username = setupModel.username.trim();
    loginModel.password = '';
    setupModel.password = '';
    setupModel.confirm = '';
    await loadUser();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '创建账号失败');
  }
}

async function logout() {
  await post('/api/login/outLogin').catch(() => undefined);
  clearAuthToken();
  user.value = null;
}

async function bootApp() {
  booting.value = true;
  try {
    const initialized = await loadSetupStatus();
    if (initialized) {
      await loadUser(false);
    } else {
      user.value = null;
    }
  } finally {
    booting.value = false;
  }
}

onMounted(() => {
  bootApp();
  window.addEventListener('popstate', () => {
    page.value = pageFromPath();
    selectedScriptId.value = scriptIdFromPath();
  });
});

const scriptState = reactive({ content: '', loading: false });
const scriptEditorHost = ref<HTMLElement | null>(null);
const scriptEditorEditable = new Compartment();
let scriptEditorView: EditorView | null = null;
let syncingScriptFromEditor = false;
function scriptFileId(item?: { path?: string }) {
  return item?.path?.split('/').pop() || '';
}

function isNewScriptEntry(item?: { name?: string }) {
  return !!item?.name?.startsWith('+');
}

function scriptDisplayName(item?: { name?: string; path?: string }) {
  if (!item) return '未选择脚本';
  const name = item.name || scriptFileId(item);
  return isNewScriptEntry(item) ? '新增脚本' : name;
}

function scriptFileName(item?: { name?: string; path?: string }) {
  if (!item) return '-';
  if (isNewScriptEntry(item)) return 'new-script.js';
  if ('file' in item && item.file) return item.file.split(/[\\/]/).pop() || 'main.js';
  const title = scriptDisplayName(item)
    .replace(/[🔧💫🔒👑]/gu, '')
    .trim();
  return `${title || scriptFileId(item)}.js`;
}

function isNodeScript(item = currentScriptFile.value) {
  return item?.type === 'node';
}

const scriptFileRows = computed(() => {
  const keyword = scriptKeyword.value.trim().toLowerCase();
  const rows = scripts.value.filter((item) => item.path?.startsWith('/script/'));
  if (!keyword) return rows;
  return rows.filter((item) => `${item.name || ''} ${scriptFileName(item)} ${scriptFileId(item)}`.toLowerCase().includes(keyword));
});
const currentScriptId = computed(() => selectedScriptId.value || realScripts.value[0]?.path?.split('/').pop() || scripts.value.find((item) => item.path?.startsWith('/script/'))?.path?.split('/').pop());
const currentScriptFile = computed(() => scripts.value.find((item) => scriptFileId(item) === currentScriptId.value));

const canEditScript = computed(() => !scriptState.loading && !!currentScriptId.value);

function scriptEditableExtension() {
  return [EditorView.editable.of(canEditScript.value), EditorState.readOnly.of(!canEditScript.value)];
}

function syncScriptEditorEditable() {
  if (!scriptEditorView) return;
  scriptEditorView.dispatch({
    effects: scriptEditorEditable.reconfigure(scriptEditableExtension()),
  });
}

function destroyScriptEditor() {
  scriptEditorView?.destroy();
  scriptEditorView = null;
}

function createScriptEditor() {
  if (scriptEditorView || !scriptEditorHost.value) return;
  const updateListener = EditorView.updateListener.of((update) => {
    if (!update.docChanged) return;
    syncingScriptFromEditor = true;
    scriptState.content = update.state.doc.toString();
    syncingScriptFromEditor = false;
  });
  scriptEditorView = new EditorView({
    parent: scriptEditorHost.value,
    state: EditorState.create({
      doc: scriptState.content,
      extensions: [
        basicSetup,
        javascript(),
        oneDark,
        EditorView.lineWrapping,
        scriptEditorEditable.of(scriptEditableExtension()),
        updateListener,
      ],
    }),
  });
}

async function ensureScriptEditor() {
  await nextTick();
  if (page.value === 'scripts') {
    createScriptEditor();
    syncScriptEditorEditable();
  } else {
    destroyScriptEditor();
  }
}

async function loadScript(id = currentScriptId.value) {
  if (!id) return;
  scriptState.loading = true;
  try {
    if (isNodeScript()) {
      const res = await get<{ data: { content: string } }>(`/api/node/script?id=${encodeURIComponent(id)}`);
      scriptState.content = res.data.content || '';
    } else {
      const res = await readStorage<Record<string, string>>(`plugins.${id}`);
      scriptState.content = res.data[`plugins.${id}`] || starter;
    }
  } finally {
    scriptState.loading = false;
  }
}

async function saveScript(value = scriptState.content) {
  if (!currentScriptId.value) return;
  if (isNodeScript()) {
    await put('/api/node/script', { id: currentScriptId.value, content: value });
  } else {
    await saveStorage({ [`plugins.${currentScriptId.value}`]: value }, currentScriptId.value);
  }
  message.success('脚本已保存');
  await loadUser();
}

async function formatScript() {
  if (!scriptState.content.trim()) return;
  try {
    const [{ default: prettier }, { default: parserBabel }, { default: parserEstree }] = await Promise.all([
      import('prettier/standalone'),
      import('prettier/plugins/babel'),
      import('prettier/plugins/estree'),
    ]);
    const formatted = await prettier.format(scriptState.content, {
      parser: 'babel',
      plugins: [parserBabel, parserEstree],
      singleQuote: true,
      semi: true,
      trailingComma: 'es5',
      printWidth: 100,
    });
    scriptState.content = formatted.trimEnd() + '\n';
    message.success('格式化完成');
  } catch (error) {
    message.error(`格式化失败：${error instanceof Error ? error.message : String(error)}`);
  }
}

async function removeScript() {
  if (!currentScriptId.value) return;
  if (isNodeScript()) {
    await del('/api/node/script', { id: currentScriptId.value });
  } else {
    await saveStorage({ [`plugins.${currentScriptId.value}`]: 'uninstall' });
  }
  message.success('脚本已卸载');
  await loadUser();
  navigate('scripts');
}

async function createScript() {
  const res = await post<{ data: { id: string } }>('/api/node/script', { name: '新脚本' });
  await loadUser();
  if (res.data.id) navigate('scripts', `/admin/script/${res.data.id}`);
}

function selectScriptFile(item: { path?: string; name?: string }) {
  const id = scriptFileId(item);
  if (!id) return;
  navigate('scripts', `/admin/script/${id}`);
}

watch(currentScriptId, (id) => loadScript(id), { immediate: true });
watch([page, () => booting.value, () => user.value], () => ensureScriptEditor(), { immediate: true });
watch([currentScriptId, () => scriptState.loading], () => syncScriptEditorEditable());
watch(
  () => scriptState.content,
  (content) => {
    if (!scriptEditorView || syncingScriptFromEditor) return;
    const current = scriptEditorView.state.doc.toString();
    if (current === content) return;
    scriptEditorView.dispatch({
      changes: { from: 0, to: scriptEditorView.state.doc.length, insert: content },
    });
  }
);

onBeforeUnmount(() => {
  destroyScriptEditor();
});

const storageState = reactive({
  keys: 'sillyGirl',
  newBucketName: '',
  rows: [] as any[],
  buckets: [] as Array<{ value: string; label: string }>,
  loading: false,
  loadingBuckets: false,
  creatingBucket: false,
  deletingBucket: false,
});
const protectedStorageBuckets = new Set(['plugins', 'sillyGirl', 'auths']);
const selectedStorageBucket = computed(() => {
  const value = storageState.keys.trim();
  if (!value || value.includes('.') || value.includes(',')) return '';
  return value;
});
const canRemoveStorageBucket = computed(() => !!selectedStorageBucket.value && !protectedStorageBuckets.has(selectedStorageBucket.value));
async function loadStorageBuckets() {
  storageState.loadingBuckets = true;
  try {
    const res = await get<{ data: Array<{ value: string; text?: string }> }>('/api/storage');
    storageState.buckets = (res.data || []).map((item) => ({
      value: item.value,
      label: item.text || item.value,
    }));
  } finally {
    storageState.loadingBuckets = false;
  }
}
async function loadStorage() {
  storageState.loading = true;
  try {
    const res = await get<{ data: any[] }>(`/api/storage/list?keys=${encodeURIComponent(storageState.keys)}`);
    storageState.rows = res.data || [];
  } finally {
    storageState.loading = false;
  }
}
async function saveStorageRow(row: any) {
  await saveStorage({ [`${row.bucket}.${row.key}`]: row.value });
  message.success('已保存');
}
async function selectStorageBucket(bucket?: string) {
  if (!bucket) return;
  storageState.keys = bucket;
  await loadStorage();
}
async function createStorageBucket() {
  const bucket = storageState.newBucketName.trim();
  if (!bucket) {
    message.error('请输入存储桶名称');
    return;
  }
  storageState.creatingBucket = true;
  try {
    await post('/api/storage/bucket', { bucket });
    message.success('存储桶已创建');
    storageState.newBucketName = '';
    storageState.keys = bucket;
    await loadStorageBuckets();
    await loadStorage();
  } finally {
    storageState.creatingBucket = false;
  }
}
async function removeStorageBucket() {
  const bucket = selectedStorageBucket.value;
  if (!bucket) {
    message.error('请选择单个存储桶');
    return;
  }
  storageState.deletingBucket = true;
  try {
    await del('/api/storage/bucket', { bucket });
    message.success('存储桶已删除');
    storageState.keys = 'sillyGirl';
    await loadStorageBuckets();
    await loadStorage();
  } finally {
    storageState.deletingBucket = false;
  }
}

const replies = reactive({ rows: [] as Reply[], total: 0, editing: null as Reply | null, form: {} as Reply });
async function loadReplies(current = 1, pageSize = 20) {
  const res = await get<{ data: Reply[]; total: number }>(`/api/reply/list?current=${current}&pageSize=${pageSize}`);
  replies.rows = res.data || [];
  replies.total = res.total || 0;
}
function openReply(row?: Reply) {
  replies.editing = row || { id: 0, priority: 0, platforms: [] };
  replies.form = { ...replies.editing };
}
async function saveReply() {
  await post('/api/reply', replies.form);
  replies.editing = null;
  message.success('已保存');
  loadReplies();
}
async function removeReply(row: Reply) {
  await del(`/api/reply?id=${row.id}`);
  message.success('已删除');
  loadReplies();
}

const masters = reactive({ rows: [] as Master[], platforms: [] as any[], editing: false, form: {} as Master });
async function loadMasters() {
  const res = await get<{ data: Master[]; platforms: any[] }>('/api/master/list');
  masters.rows = res.data || [];
  masters.platforms = res.platforms || [];
}
async function saveMaster() {
  await post('/api/master', masters.form);
  masters.editing = false;
  message.success('已保存');
  loadMasters();
}
async function removeMaster(row: Master) {
  await del('/api/master', row);
  message.success('已删除');
  loadMasters();
}

const tasks = reactive({ rows: [] as Task[], total: 0, editing: null as Task | null, form: {} as any, scripts: [] as any[] });
async function loadTasks(current = 1, pageSize = 20) {
  const res = await get<{ data: Task[]; total: number }>(`/api/tasks?current=${current}&pageSize=${pageSize}`);
  tasks.rows = res.data || [];
  tasks.total = res.total || 0;
}
async function loadTaskSelects(taskId = '') {
  const res = await get<{ data: { scripts?: Record<string, string> } }>(`/api/task/selects?task_id=${encodeURIComponent(taskId)}`);
  tasks.scripts = Object.entries(res.data?.scripts || {})
    .filter(([, label]) => String(label).endsWith('.js'))
    .map(([, label]) => {
      const name = String(label).replace(/\.js$/, '');
      return { value: `node ${name}.js`, label: `node ${name}.js` };
    });
}
async function openTask(row?: Task) {
  const data = row || { enable: true, command: '' };
  tasks.editing = data;
  await loadTaskSelects(data.task_id || '');
  tasks.form = { ...data };
}
function validateTaskCron(schedule?: string) {
  const value = `${schedule || ''}`.trim();
  if (!value) return false;
  const parts = value.split(/\s+/);
  if (parts.length !== 5 && parts.length !== 6) return false;
  return parts.every((part) => /^[\d*,/?#LW\-\u0041-\u005A\u0061-\u007A]+$/.test(part));
}
async function saveTask() {
  if (!`${tasks.form.title || ''}`.trim()) {
    message.error('定时任务标题不能为空');
    return;
  }
  if (!validateTaskCron(tasks.form.schedule)) {
    message.error('Cron表达式格式错误，例如：0 * * * *');
    return;
  }
  const payload = {
    task_id: tasks.form.task_id,
    title: `${tasks.form.title || ''}`.trim(),
    schedule: `${tasks.form.schedule || ''}`.trim(),
    command: tasks.form.command,
    enable: tasks.form.enable,
  };
  await post('/api/tasks', payload);
  tasks.editing = null;
  message.success('已保存');
  loadTasks();
}
async function removeTask(row: Task) {
  await del('/api/tasks', row);
  message.success('已删除');
  loadTasks();
}
async function runTask(row: Task) {
  await get(`/api/tasks/run?task_id=${encodeURIComponent(row.task_id)}`);
  message.success('已触发');
}

const carry = reactive({ rows: [] as CarryGroup[], total: 0, editing: null as CarryGroup | null, form: {} as any, selects: {} as any });
async function loadCarry(current = 1, pageSize = 20) {
  const res = await get<{ data: CarryGroup[]; total: number }>(`/api/carry/groups?current=${current}&pageSize=${pageSize}`);
  carry.rows = res.data || [];
  carry.total = res.total || 0;
}
async function loadCarrySelects(row?: CarryGroup) {
  const res = await get<{ data: any }>(
    `/api/carry/group_selects?chat_id=${encodeURIComponent(row?.chat_id || '')}&platform=${encodeURIComponent(row?.platform || '')}`,
  );
  carry.selects = res.data || {};
}
async function openCarry(row?: CarryGroup) {
  const data = row || { chat_id: '', enable: true, in: true, out: false };
  carry.editing = data;
  await loadCarrySelects(data);
  carry.form = {
    ...data,
    includeText: asArray(data.include).join('\n'),
    excludeText: asArray(data.exclude).join('\n'),
    allowedText: asArray(data.allowed).join('\n'),
    prohibitedText: asArray(data.prohibited).join('\n'),
  };
}
async function saveCarry() {
  const payload = {
    ...carry.form,
    include: splitTags(carry.form.includeText || ''),
    exclude: splitTags(carry.form.excludeText || ''),
    allowed: splitTags(carry.form.allowedText || ''),
    prohibited: splitTags(carry.form.prohibitedText || ''),
  };
  delete payload.includeText;
  delete payload.excludeText;
  delete payload.allowedText;
  delete payload.prohibitedText;
  await post('/api/carry/group', payload);
  carry.editing = null;
  message.success('已保存');
  loadCarry();
}
async function removeCarry(row: CarryGroup) {
  await del('/api/carry/group', row);
  message.success('已删除');
  loadCarry();
}

const qinglong = reactive({
  rows: [] as QinglongPanel[],
  total: 0,
  loading: false,
  editing: null as QinglongPanel | null,
  form: {} as QinglongPanel,
  testing: false,
  saving: false,
});
async function loadQinglongPanels() {
  qinglong.loading = true;
  try {
    const res = await get<{ data: QinglongPanel[]; total: number }>('/api/qinglong/panels');
    qinglong.rows = res.data || [];
    qinglong.total = res.total || 0;
  } finally {
    qinglong.loading = false;
  }
}
function openQinglongPanel(row?: QinglongPanel) {
  const data = row || { name: '', address: '', client_id: '', client_secret: '' };
  qinglong.editing = data;
  qinglong.form = { ...data };
}
async function testQinglongPanel(panel = qinglong.form) {
  qinglong.testing = true;
  try {
    await post('/api/qinglong/panel/test', panel);
    message.success('青龙接口连接成功');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '青龙接口连接失败');
  } finally {
    qinglong.testing = false;
  }
}
async function saveQinglongPanel() {
  qinglong.saving = true;
  try {
    await post('/api/qinglong/panel', qinglong.form);
    qinglong.editing = null;
    message.success('青龙面板已添加');
    await loadQinglongPanels();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '青龙面板添加失败');
  } finally {
    qinglong.saving = false;
  }
}
async function removeQinglongPanel(row: QinglongPanel) {
  await del('/api/qinglong/panel', row);
  message.success('已删除');
  loadQinglongPanels();
}

const smallcat = reactive({
  rows: [] as SmallcatPanel[],
  total: 0,
  loading: false,
  editing: null as SmallcatPanel | null,
  form: {} as SmallcatPanel,
  testing: false,
  saving: false,
});
async function loadSmallcatPanels() {
  smallcat.loading = true;
  try {
    const res = await get<{ data: SmallcatPanel[]; total: number }>('/api/smallcat/panels');
    smallcat.rows = res.data || [];
    smallcat.total = res.total || 0;
  } finally {
    smallcat.loading = false;
  }
}
function openSmallcatPanel(row?: SmallcatPanel) {
  const data = row || { name: '', address: '', api_auth: '' };
  smallcat.editing = data;
  smallcat.form = { ...data };
}
async function testSmallcatPanel(panel = smallcat.form) {
  smallcat.testing = true;
  try {
    await post('/api/smallcat/panel/test', panel);
    message.success('smallcat API AUTH 验证通过');
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'smallcat 验证失败');
  } finally {
    smallcat.testing = false;
  }
}
async function saveSmallcatPanel() {
  smallcat.saving = true;
  try {
    await post('/api/smallcat/panel', smallcat.form);
    smallcat.editing = null;
    message.success('smallcat 已添加');
    await loadSmallcatPanels();
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'smallcat 添加失败');
  } finally {
    smallcat.saving = false;
  }
}
async function removeSmallcatPanel(row: SmallcatPanel) {
  await del('/api/smallcat/panel', row);
  message.success('已删除');
  loadSmallcatPanels();
}

const plugins = reactive({
  rows: [] as PluginInfo[],
  total: 0,
  tab: 'all',
  keyword: '',
  klass: '全部',
  meta: {} as any,
  loading: false,
  sources: [] as string[],
  sourceAddress: '',
  sourceSaving: false,
  githubProxy: '',
  githubProxyOptions: [] as string[],
  githubProxySaving: false,
  sourceModal: false,
  sourceRemoving: {} as Record<string, boolean>,
  installing: {} as Record<string, boolean>,
});
async function openPluginSourceManager() {
  plugins.sourceModal = true;
  await Promise.all([loadPluginSources(), loadGithubProxy()]);
}
async function loadPluginSources() {
  try {
    const res = await get<{ data: string[] }>('/api/plugins/sources');
    plugins.sources = res.data || [];
  } catch {
    plugins.sources = [];
  }
}
async function loadGithubProxy() {
  try {
    const res = await get<{ data: { proxy: string; options: string[] } }>('/api/plugins/github-proxy');
    plugins.githubProxy = res.data?.proxy || '';
    plugins.githubProxyOptions = res.data?.options || [];
  } catch {
    plugins.githubProxy = '';
    plugins.githubProxyOptions = [];
  }
}
async function saveGithubProxy() {
  plugins.githubProxySaving = true;
  try {
    const res = await put<{ data?: { proxy?: string } }>('/api/plugins/github-proxy', { proxy: plugins.githubProxy.trim() });
    plugins.githubProxy = res.data?.proxy || '';
    message.success(plugins.githubProxy ? '加速链接已生成' : 'GitHub 加速已关闭');
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'GitHub 代理保存失败');
  } finally {
    plugins.githubProxySaving = false;
  }
}
async function loadPlugins(current = 1, pageSize = 12) {
  plugins.loading = true;
  try {
    const params = new URLSearchParams({
      current: String(current),
      pageSize: String(pageSize),
      activeKey: plugins.tab,
      keyword: plugins.keyword,
      class: plugins.klass,
    });
    const res = await get<any>(`/api/plugins/list.json?${params.toString()}`);
    plugins.rows = res.data || [];
    plugins.total = res.total || 0;
    plugins.meta = res;
  } finally {
    plugins.loading = false;
  }
}
async function addPluginSource() {
  const address = plugins.sourceAddress.trim();
  if (!address) {
    message.error('请输入 GitHub 仓库地址或 link:// 地址');
    return;
  }
  plugins.sourceSaving = true;
  try {
    const res = await post<{ data?: { count?: number } }>('/api/plugins/source', { address });
    plugins.sourceAddress = '';
    plugins.tab = 'all';
    message.success(`插件源已新增${res.data?.count ? `，发现 ${res.data.count} 个插件` : ''}`);
    await Promise.all([loadPluginSources(), loadPlugins(1)]);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '插件源新增失败');
  } finally {
    plugins.sourceSaving = false;
  }
}
async function removePluginSource(address: string) {
  plugins.sourceRemoving[address] = true;
  try {
    await del('/api/plugins/source', { address });
    message.success('插件源已删除');
    await Promise.all([loadPluginSources(), loadPlugins(1)]);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '插件源删除失败');
  } finally {
    plugins.sourceRemoving[address] = false;
  }
}
async function installPlugin(row: PluginInfo) {
  plugins.installing[row.id] = true;
  try {
    const res = await put<{ success: boolean; errors?: Record<string, string>; messages?: Record<string, string> }>('/api/storage', {
      [`plugins.${row.id}`]: 'install',
    });
    const firstError = Object.values(res.errors || {}).find(Boolean);
    if (firstError) {
      throw new ApiError(200, firstError);
    }
    const firstMessage = Object.values(res.messages || {}).find(Boolean);
    message.success(firstMessage || (row.status === 1 ? '已更新' : '已安装'));
    await Promise.all([loadPlugins(), loadUser()]);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '插件安装失败');
  } finally {
    plugins.installing[row.id] = false;
  }
}
function pluginStatusLabel(row: PluginInfo) {
  if (row.status === 1) return '可更新';
  if (row.status === 2 || row.status === 6) return '已安装';
  return '未安装';
}
function pluginStatusColor(row: PluginInfo) {
  if (row.status === 1) return 'green';
  if (row.status === 2 || row.status === 6) return 'green';
  return 'default';
}
function pluginActionLabel(row: PluginInfo) {
  if (row.status === 1) return '更新';
  if (row.status === 2 || row.status === 6) return '已安装';
  return '安装';
}
function pluginActionDisabled(row: PluginInfo) {
  return row.status === 2 || row.status === 6;
}

type NodeDependencyPlugin = {
  name: string;
  title?: string;
  file?: string;
  path: string;
};

type NodeDependencyRow = {
  name: string;
  version: string;
  dev: boolean;
  installed: boolean;
  source?: string;
  plugin: string;
  plugin_title?: string;
  plugin_file?: string;
};

const nodeDeps = reactive({
  plugins: [] as NodeDependencyPlugin[],
  plugin: '',
  rows: [] as NodeDependencyRow[],
  packageName: '',
  registry: 'https://registry.npmmirror.com',
  dev: false,
  loading: false,
  saving: false,
  savingRegistry: false,
  removing: {} as Record<string, boolean>,
  pnpm: { available: false, path: '', message: '', registry: '' } as { available: boolean; path?: string; message?: string; registry?: string },
});
async function loadNodeDependencies(plugin = '') {
  nodeDeps.loading = true;
  try {
    const query = plugin ? `?plugin=${encodeURIComponent(plugin)}` : '';
    const res = await get<{ data: { plugins: NodeDependencyPlugin[]; plugin: string; dependencies: NodeDependencyRow[]; pnpm: typeof nodeDeps.pnpm } }>(`/api/node/dependencies${query}`);
    nodeDeps.plugins = res.data.plugins || [];
    nodeDeps.plugin = res.data.plugin || '';
    nodeDeps.rows = res.data.dependencies || [];
    nodeDeps.pnpm = res.data.pnpm || { available: false };
    nodeDeps.registry = nodeDeps.pnpm.registry || 'https://registry.npmmirror.com';
  } finally {
    nodeDeps.loading = false;
  }
}
async function saveNodeDependencyRegistry() {
  nodeDeps.savingRegistry = true;
  try {
    const res = await put<{ data: { registry: string } }>('/api/node/dependency/registry', { registry: nodeDeps.registry });
    nodeDeps.registry = res.data?.registry || nodeDeps.registry;
    nodeDeps.pnpm.registry = nodeDeps.registry;
    message.success('pnpm 镜像已保存');
  } finally {
    nodeDeps.savingRegistry = false;
  }
}
async function installNodeDependency() {
  await installNodeDependencyPackage(nodeDeps.packageName.trim(), () => {
    nodeDeps.packageName = '';
  });
}
async function installNodeDependencyPackage(pkg: string, after?: () => void) {
  if (!nodeDeps.plugins.length) {
    message.error('暂无 NodeJS 脚本插件');
    return;
  }
  if (!pkg) {
    message.error('请输入依赖名称');
    return;
  }
  if (nodeDeps.plugins.length !== 1) {
    message.error('当前存在多个插件，请在表格对应依赖行点击安装');
    return;
  }
  nodeDeps.saving = true;
  try {
    await post('/api/node/dependency', { plugin: nodeDeps.plugins[0].name, package: pkg, dev: nodeDeps.dev });
    after?.();
    message.success('依赖已安装');
    await loadNodeDependencies();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '依赖安装失败');
  } finally {
    nodeDeps.saving = false;
  }
}
async function installNodeDependencyRow(row: NodeDependencyRow) {
  if (!row.plugin) {
    message.error('缺少插件信息');
    return;
  }
  nodeDeps.saving = true;
  try {
    await post('/api/node/dependency', { plugin: row.plugin, package: row.name, dev: row.dev });
    message.success('依赖已安装');
    await loadNodeDependencies();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '依赖安装失败');
  } finally {
    nodeDeps.saving = false;
  }
}
async function removeNodeDependency(row: NodeDependencyRow) {
  const key = `${row.plugin}.${row.name}`;
  nodeDeps.removing[key] = true;
  try {
    await del('/api/node/dependency', { plugin: row.plugin, package: row.name });
    message.success('依赖已卸载');
    await loadNodeDependencies();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '依赖卸载失败');
  } finally {
    nodeDeps.removing[key] = false;
  }
}

const pluginConfigs = reactive({
  rows: [] as any[],
  selected: null as any,
  form: {} as Record<string, any>,
  text: {} as Record<string, string>,
  loading: false,
});
const schemaFields = computed(() => {
  const props = pluginConfigs.selected?.schema?.properties || {};
  return Object.entries(props).map(([key, prop]) => ({ key, prop: prop as any }));
});
const pluginConfigOptions = computed(() =>
  pluginConfigs.rows.map((row) => ({
    value: row.uuid,
    label: `${row.plugin || row.title || row.uuid}${row.file ? ` / ${row.file}` : ''}`,
  })),
);
async function loadPluginConfigs() {
  pluginConfigs.loading = true;
  try {
    const res = await get<{ data: any[] }>('/api/plugin/configs');
    pluginConfigs.rows = res.data || [];
    if (pluginConfigs.selected) {
      const next = pluginConfigs.rows.find((item) => item.uuid === pluginConfigs.selected?.uuid);
      if (next) openPluginConfig(next);
      else pluginConfigs.selected = null;
    }
  } finally {
    pluginConfigs.loading = false;
  }
}
function selectPluginConfig(uuid?: string) {
  const row = pluginConfigs.rows.find((item) => item.uuid === uuid);
  if (row) {
    openPluginConfig(row);
    return;
  }
  pluginConfigs.selected = null;
  pluginConfigs.form = {};
  pluginConfigs.text = {};
}
function openPluginConfig(row: any) {
  pluginConfigs.selected = row;
  const values = { ...(row.user_config || {}) };
  for (const [key, prop] of Object.entries(row.schema?.properties || {}) as Array<[string, any]>) {
    if (values[key] === undefined && prop.default !== undefined) values[key] = prop.default;
  }
  pluginConfigs.form = values;
  pluginConfigs.text = {};
  for (const [key, value] of Object.entries(values)) {
    if (typeof value === 'object' && value !== null) {
      pluginConfigs.text[key] = JSON.stringify(value, null, 2);
    }
  }
}
function fieldOptions(prop: any) {
  const values = prop?.enum || [];
  const names = prop?.enumNames || [];
  return values.map((value: any, index: number) => ({ value, label: names[index] || String(value) }));
}
function fieldType(prop: any) {
  if (Array.isArray(prop?.enum)) return 'enum';
  return prop?.type || 'string';
}
async function savePluginConfig() {
  if (!pluginConfigs.selected) return;
  const value = { ...pluginConfigs.form };
  for (const field of schemaFields.value) {
    const type = fieldType(field.prop);
    if ((type === 'object' || type === 'array') && pluginConfigs.text[field.key] !== undefined) {
      try {
        value[field.key] = JSON.parse(pluginConfigs.text[field.key] || (type === 'array' ? '[]' : '{}'));
      } catch {
        message.error(`${field.prop.title || field.key} JSON 格式错误`);
        return;
      }
    }
  }
  await putPluginConfig(pluginConfigs.selected.uuid, value);
  message.success('插件配置已保存');
  await loadPluginConfigs();
}
async function putPluginConfig(uuid: string, value: Record<string, any>) {
  await fetch('/api/plugin/config', {
    method: 'PUT',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ uuid, value }),
  }).then(async (res) => {
    const data = await res.json();
    if (!res.ok || data.success === false) throw new Error(data.errorMessage || '保存失败');
  });
}

const settings = reactive({ form: {} as any });
const storageBackendOptions = [
  { label: 'BoltDB', value: 'boltdb' },
  { label: 'Redis', value: 'redis' },
];
const settingsKeys = [
  'sillyGirl.name',
  'sillyGirl.password',
  'sillyGirl.port',
  'sillyGirl.api_key',
  'sillyGirl.debug',
  'sillyGirl.listen_admin',
  'sillyGirl.recall',
  'sillyGirl.storage',
  'sillyGirl.redis_addr',
  'sillyGirl.redis_password',
];
async function loadSettings() {
  const res = await readStorage<Record<string, any>>(settingsKeys.join(','));
  const data = res.data || {};
  settings.form = {
    name: data['sillyGirl.name'],
    password: '',
    port: Number(data['sillyGirl.port'] || 8080),
    api_key: data['sillyGirl.api_key'],
    debug: data['sillyGirl.debug'] === true || data['sillyGirl.debug'] === 'true',
    listen_admin: data['sillyGirl.listen_admin'] !== false && data['sillyGirl.listen_admin'] !== 'false',
    recall: data['sillyGirl.recall'],
    storage: data['sillyGirl.storage'] === 'redis' ? 'redis' : 'boltdb',
    redis_addr: data['sillyGirl.redis_addr'],
    redis_password: data['sillyGirl.redis_password'],
  };
}
async function saveSettings() {
  const v = settings.form;
  const updates: Record<string, unknown> = {
    'sillyGirl.name': v.name || '',
    'sillyGirl.port': v.port || 8080,
    'sillyGirl.api_key': v.api_key || '',
    'sillyGirl.debug': !!v.debug,
    'sillyGirl.listen_admin': !!v.listen_admin,
    'sillyGirl.recall': v.recall || '',
    'sillyGirl.storage': v.storage || 'boltdb',
    'sillyGirl.redis_addr': v.redis_addr || '',
    'sillyGirl.redis_password': v.redis_password || '',
  };
  if (v.password) updates['sillyGirl.password'] = v.password;
  await saveStorage(updates);
  message.success('设置已保存');
  loadUser();
}

const messageBuckets = {
  listen: { label: '监听群组', bucket: 'listenOnGroups' },
  noreply: { label: '禁言群组', bucket: 'noReplyGroups' },
  private: { label: '屏蔽用户', bucket: 'noListenUsers' },
};
const msgState = reactive({ active: 'listen' as keyof typeof messageBuckets, rows: [] as any[], editing: null as any, form: {} as any, platforms: [] as any[] });
async function loadMessages() {
  const bucket = messageBuckets[msgState.active].bucket;
  const res = await get<{ data: any[] }>(`/api/storage/list?keys=${bucket}`);
  msgState.rows = (res.data || []).map((row) => {
    try {
      return { ...row, ...JSON.parse(row.value || '{}') };
    } catch {
      return row;
    }
  });
  const master = await get<{ platforms?: any[] }>('/api/master/list').catch(() => ({ platforms: [] }));
  msgState.platforms = master.platforms || [];
}
function openMessage(row?: any) {
  msgState.editing = row || { key: '', enable: true };
  msgState.form = { ...msgState.editing };
}
async function saveMessageRow() {
  const bucket = messageBuckets[msgState.active].bucket;
  await saveStorage({
    [`${bucket}.${msgState.form.key}`]: JSON.stringify({
      platform: msgState.form.platform || '',
      enable: !!msgState.form.enable,
      desc: msgState.form.desc || '',
    }),
  });
  msgState.editing = null;
  message.success('已保存');
  loadMessages();
}
async function removeMessageRow(row: any) {
  const bucket = messageBuckets[msgState.active].bucket;
  await saveStorage({ [`${bucket}.${row.key}`]: '' });
  message.success('已删除');
  loadMessages();
}

watch([page, user], ([p]) => {
  if (!user.value) return;
  if (p === 'reply') loadReplies();
  if (p === 'masters') loadMasters();
  if (p === 'tasks') loadTasks();
  if (p === 'carry') loadCarry();
  if (p === 'qinglong') loadQinglongPanels();
  if (p === 'smallcat') loadSmallcatPanels();
  if (p === 'dependencies') loadNodeDependencies();
  if (p === 'plugins') {
    loadPluginSources();
    loadPlugins();
  }
  if (p === 'plugin-configs') loadPluginConfigs();
  if (p === 'storage') {
    loadStorageBuckets();
    loadStorage();
  }
  if (p === 'settings') loadSettings();
  if (p === 'messages') loadMessages();
}, { immediate: true });
watch(() => plugins.tab, () => loadPlugins());
watch(() => plugins.klass, () => loadPlugins());
watch(() => nodeDeps.plugin, (plugin) => {
  if (page.value === 'dependencies' && plugin) loadNodeDependencies(plugin);
});
watch(() => msgState.active, () => loadMessages());

function optionMap(values?: string[]) {
  return (values || []).map((value) => ({ value, label: value }));
}
function recordOptions(record?: Record<string, string>) {
  return Object.entries(record || {}).map(([value, label]) => ({ value, label }));
}
</script>

<template>
  <a-config-provider :locale="zhCN">
    <AntApp>
      <div v-if="!booting && !user" class="login-page">
        <div class="login-card">
          <template v-if="setupRequired">
            <Typography.Title :level="3" style="margin-top: 0">初始化管理员</Typography.Title>
            <Typography.Paragraph class="muted">首次使用需要创建后台账号和密码。</Typography.Paragraph>
            <Form layout="vertical" @finish="setupAdmin">
              <Form.Item label="账号" required>
                <Input v-model:value="setupModel.username">
                  <template #prefix><User :size="16" /></template>
                </Input>
              </Form.Item>
              <Form.Item label="密码" required>
                <Input.Password v-model:value="setupModel.password" />
              </Form.Item>
              <Form.Item label="确认密码" required>
                <Input.Password v-model:value="setupModel.confirm" />
              </Form.Item>
              <Button type="primary" html-type="button" block @click="setupAdmin">创建账号</Button>
            </Form>
          </template>
          <template v-else>
            <Typography.Title :level="3" style="margin-top: 0">SillyGirl Admin</Typography.Title>
            <Typography.Paragraph class="muted">使用后台账号和密码登录。</Typography.Paragraph>
            <Form layout="vertical" @finish="login">
              <Form.Item label="账号" required>
                <Input v-model:value="loginModel.username">
                  <template #prefix><User :size="16" /></template>
                </Input>
              </Form.Item>
              <Form.Item label="密码" required>
                <Input.Password v-model:value="loginModel.password" />
              </Form.Item>
              <Button type="primary" html-type="button" block @click="login">登录</Button>
            </Form>
          </template>
        </div>
      </div>

      <Layout v-else class="shell">
        <Layout.Sider :width="220" breakpoint="lg" :collapsed-width="0" theme="light">
          <div class="brand"><span class="brand-mark">S</span><span>SillyGirl</span></div>
          <Menu mode="inline" :selected-keys="[page]" :items="menuItems" style="border-inline-end: 0; padding-top: 8px" @click="(e:any) => navigate(e.key)" />
        </Layout.Sider>
        <Layout>
          <div class="topbar">
            <div>
              <Typography.Text strong>{{ menuItems.find((item) => item.key === page)?.label || '后台' }}</Typography.Text>
              <Typography.Text class="muted" style="margin-left: 10px">{{ user?.name || '傻妞' }}</Typography.Text>
            </div>
            <Button @click="logout"><template #icon><LogOut :size="16" /></template>退出</Button>
          </div>
          <main class="content">
            <section v-if="page === 'welcome'" class="panel">
              <Typography.Title :level="3" style="margin-top: 0">{{ user?.name || '傻妞' }}</Typography.Title>
              <Space wrap style="margin-bottom: 14px">
                <Tag color="blue">本地版本 {{ overviewVersion.local }}</Tag>
                <Tag :color="overviewVersion.remote === '待发布' ? 'default' : 'green'">远程版本 {{ overviewVersion.remote }}</Tag>
                <Typography.Link :href="overviewVersion.repository" target="_blank">GitHub</Typography.Link>
              </Space>
              <a-row :gutter="[12, 12]">
                <a-col :xs="24" :sm="12" :md="8"><Card><Statistic title="脚本数量" :value="realScripts.length" /></Card></a-col>
                <a-col :xs="24" :sm="12" :md="8"><Card><Statistic title="青龙容器" :value="overviewIntegrations.find((item) => item.key === 'qinglong')?.count || 0" /></Card></a-col>
                <a-col :xs="24" :sm="12" :md="8"><Card><Statistic title="smallcat" :value="overviewIntegrations.find((item) => item.key === 'smallcat')?.count || 0" /></Card></a-col>
              </a-row>
              <div style="margin-top: 16px">
                <div class="toolbar">
                  <div class="toolbar-left">
                    <Server :size="16" />
                    <Typography.Text strong>服务连接状态</Typography.Text>
                  </div>
                </div>
                <Table :row-key="(row:any) => row.key" :data-source="overviewIntegrations" :pagination="false" size="small">
                  <Table.Column title="服务" data-index="label" />
                  <Table.Column title="状态" :width="120">
                    <template #default="{ record }">
                      <Tag :color="record.count > 0 && record.online ? 'green' : 'default'">{{ record.count > 0 && record.online ? '已连接' : '未连接' }}</Tag>
                    </template>
                  </Table.Column>
                  <Table.Column title="总数" data-index="count" :width="100" />
                  <Table.Column title="在线数" data-index="online_count" :width="100" />
                </Table>
              </div>
              <div style="margin-top: 16px">
                <div class="toolbar">
                  <div class="toolbar-left">
                    <Radio :size="16" />
                    <Typography.Text strong>机器人连接状态</Typography.Text>
                  </div>
                  <Button @click="loadUser"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
                <Table :row-key="(row:any) => row.platform" :data-source="overviewAdapters" :pagination="false" size="small">
                  <Table.Column title="平台" data-index="label" />
                  <Table.Column title="状态" :width="120">
                    <template #default="{ record }">
                      <Tag :color="record.online ? 'green' : 'default'">{{ record.online ? '已连接' : '未连接' }}</Tag>
                    </template>
                  </Table.Column>
                  <Table.Column title="实例数" data-index="count" :width="100" />
                  <Table.Column title="Bot ID">
                    <template #default="{ record }">
                      <Typography.Text class="mono">{{ record.bots_id?.length ? record.bots_id.join(', ') : '-' }}</Typography.Text>
                    </template>
                  </Table.Column>
                </Table>
              </div>
            </section>

            <section v-if="page === 'scripts'" class="panel">
              <div class="script-workbench">
                <aside class="script-file-panel">
                  <div class="script-file-header">
                    <Space size="small">
                      <FolderOpen :size="16" />
                      <Typography.Text strong>文件管理</Typography.Text>
                    </Space>
                    <Tag>{{ realScripts.length }}</Tag>
                  </div>
                  <Input v-model:value="scriptKeyword" allow-clear placeholder="搜索脚本文件">
                    <template #prefix><Search :size="15" /></template>
                  </Input>
                  <div class="script-file-actions">
                    <Button type="primary" block @click="createScript"><template #icon><Plus :size="16" /></template>新增脚本</Button>
                    <Button block @click="loadUser"><template #icon><RefreshCw :size="16" /></template>刷新列表</Button>
                  </div>
                  <div class="script-file-list">
                    <button
                      v-for="item in scriptFileRows"
                      :key="item.path"
                      type="button"
                      class="script-file-row"
                      :class="{ active: scriptFileId(item) === currentScriptId, pending: isNewScriptEntry(item) }"
                      @click="selectScriptFile(item)"
                    >
                      <FileCode2 :size="16" />
                      <span class="script-file-main">
                        <span class="script-file-name">{{ scriptDisplayName(item) }}</span>
                        <span class="script-file-meta">{{ scriptFileName(item) }}</span>
                      </span>
                      <Tag v-if="isNewScriptEntry(item)" color="blue">新建</Tag>
                    </button>
                    <a-empty v-if="scriptFileRows.length === 0" description="暂无脚本文件" />
                  </div>
                </aside>

                <div class="script-editor-panel">
                  <div class="script-editor-header">
                    <div class="script-editor-title">
                      <Typography.Text strong>{{ scriptDisplayName(currentScriptFile) }}</Typography.Text>
                      <Typography.Text class="muted mono">{{ scriptFileName(currentScriptFile) }}</Typography.Text>
                    </div>
                    <div class="script-editor-actions">
                      <Button @click="loadScript()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                      <Button @click="formatScript" :disabled="scriptState.loading || !currentScriptId">
                        <template #icon><Wand2 :size="16" /></template>格式化
                      </Button>
                      <Button type="primary" @click="saveScript()" :disabled="!currentScriptId">
                        <template #icon><Save :size="16" /></template>保存
                      </Button>
                      <Popconfirm title="确认卸载这个脚本？" @confirm="removeScript">
                        <Button danger :disabled="!currentScriptId"><template #icon><Trash2 :size="16" /></template>卸载</Button>
                      </Popconfirm>
                    </div>
                  </div>
                  <div ref="scriptEditorHost" class="code-editor script-code-editor" />
                  <div class="script-editor-status">
                    <span>{{ isNodeScript() ? 'NodeJS' : 'Goja' }}</span>
                    <span>{{ scriptState.content.split('\n').length }} 行</span>
                    <span>{{ scriptState.content.length }} 字符</span>
                  </div>
                </div>
              </div>
            </section>

            <section v-if="page === 'dependencies'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Typography.Text class="muted">共 {{ nodeDeps.plugins.length }} 个 NodeJS 脚本插件</Typography.Text>
                  <Typography.Text v-if="nodeDeps.pnpm.message" type="danger">{{ nodeDeps.pnpm.message }}</Typography.Text>
                </div>
                <div class="toolbar-right">
                  <Button @click="loadNodeDependencies()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
              </div>
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Input
                  v-model:value="nodeDeps.packageName"
                  style="width: 320px"
                  placeholder="依赖名，例如 axios 或 ipp@latest"
                  @press-enter="installNodeDependency"
                />
                <Switch v-model:checked="nodeDeps.dev" checked-children="Dev" un-checked-children="Prod" />
                <Button type="primary" :disabled="!nodeDeps.pnpm.available || nodeDeps.plugins.length !== 1" :loading="nodeDeps.saving" @click="installNodeDependency">
                  <template #icon><Download :size="16" /></template>安装依赖
                </Button>
                <Space.Compact>
                  <Input v-model:value="nodeDeps.registry" style="width: 300px" placeholder="pnpm 镜像地址" @press-enter="saveNodeDependencyRegistry" />
                  <Button :loading="nodeDeps.savingRegistry" @click="saveNodeDependencyRegistry">
                    <template #icon><Save :size="16" /></template>保存镜像
                  </Button>
                </Space.Compact>
              </div>
              <Table :row-key="(row:any) => `${row.plugin}.${row.name}`" :loading="nodeDeps.loading" :data-source="nodeDeps.rows" :pagination="{ pageSize: 20 }">
                <Table.Column title="#" :width="64">
                  <template #default="{ index }">{{ index + 1 }}</template>
                </Table.Column>
                <Table.Column title="插件" :width="180">
                  <template #default="{ record }"><Typography.Text>{{ record.plugin_title || record.plugin }}</Typography.Text></template>
                </Table.Column>
                <Table.Column title="文件名" :width="140">
                  <template #default="{ record }"><Typography.Text class="mono">{{ record.plugin_file || 'main.js' }}</Typography.Text></template>
                </Table.Column>
                <Table.Column title="依赖名称" data-index="name" />
                <Table.Column title="版本" data-index="version" :width="180">
                  <template #default="{ text }"><Typography.Text class="mono">{{ text || '-' }}</Typography.Text></template>
                </Table.Column>
                <Table.Column title="状态" :width="110">
                  <template #default="{ record }"><Tag :color="record.installed ? 'green' : 'orange'">{{ record.installed ? '已安装' : '未安装' }}</Tag></template>
                </Table.Column>
                <Table.Column title="来源" data-index="source" :width="150" />
                <Table.Column title="类型" :width="100">
                  <template #default="{ record }"><Tag :color="record.dev ? 'blue' : 'green'">{{ record.dev ? 'dev' : 'prod' }}</Tag></template>
                </Table.Column>
                <Table.Column title="操作" :width="130">
                  <template #default="{ record }">
                    <Button v-if="!record.installed" type="link" :disabled="!nodeDeps.pnpm.available" :loading="nodeDeps.saving" @click="installNodeDependencyRow(record)">安装</Button>
                    <Popconfirm v-else title="确认卸载这个依赖？" @confirm="removeNodeDependency(record)">
                      <Button type="text" danger :loading="nodeDeps.removing[`${record.plugin}.${record.name}`]"><Trash2 :size="16" /></Button>
                    </Popconfirm>
                  </template>
                </Table.Column>
              </Table>
              <a-empty v-if="!nodeDeps.loading && nodeDeps.rows.length === 0" description="暂未识别到插件需要依赖。" />
            </section>

            <section v-if="page === 'storage'" class="panel">
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Select
                  :value="storageState.keys"
                  style="width: 220px"
                  show-search
                  allow-clear
                  placeholder="选择存储桶"
                  :loading="storageState.loadingBuckets"
                  :options="storageState.buckets"
                  @change="selectStorageBucket"
                />
                <Input v-model:value="storageState.keys" style="width: 360px" placeholder="bucket 或 bucket.key，多个用逗号分隔" />
                <Button type="primary" @click="loadStorage"><template #icon><Search :size="16" /></template>查询</Button>
                <Button @click="loadStorage"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                <Space.Compact>
                  <Input v-model:value="storageState.newBucketName" style="width: 180px" placeholder="新存储桶名称" @press-enter="createStorageBucket" />
                  <Button :loading="storageState.creatingBucket" @click="createStorageBucket">
                    <template #icon><Plus :size="16" /></template>新建桶
                  </Button>
                </Space.Compact>
                <Popconfirm
                  :title="`确认删除存储桶 ${selectedStorageBucket || storageState.keys}？`"
                  description="删除后该桶内所有键值都会被移除，无法恢复。"
                  ok-text="确认删除"
                  cancel-text="取消"
                  @confirm="removeStorageBucket"
                >
                  <Button danger :disabled="!canRemoveStorageBucket" :loading="storageState.deletingBucket">
                    <template #icon><Trash2 :size="16" /></template>删除桶
                  </Button>
                </Popconfirm>
              </div>
              <Table :row-key="(row:any) => `${row.bucket}.${row.key}`" :loading="storageState.loading" :data-source="storageState.rows" :pagination="{ pageSize: 20 }">
                <Table.Column title="#" data-index="index" :width="64" />
                <Table.Column title="Bucket" data-index="bucket" :width="160" />
                <Table.Column title="Key" data-index="key" :width="220" />
                <Table.Column title="Value">
                  <template #default="{ record }">
                    <Space.Compact style="width: 100%">
                      <Input.TextArea v-model:value="record.value" :auto-size="{ minRows: 1, maxRows: 6 }" />
                      <Button @click="saveStorageRow(record)"><Save :size="16" /></Button>
                    </Space.Compact>
                  </template>
                </Table.Column>
              </Table>
            </section>

            <section v-if="page === 'reply'" class="panel">
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Button type="primary" @click="openReply()"><template #icon><Plus :size="16" /></template>新增回复</Button>
                <Button @click="loadReplies()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Table :row-key="(row:any) => String(row.id)" :data-source="replies.rows" :pagination="{ total: replies.total, pageSize: 20, onChange: loadReplies }">
                <Table.Column title="#" data-index="index" :width="64" />
                <Table.Column title="关键词" data-index="keyword" :width="220" />
                <Table.Column title="回复内容" data-index="value" ellipsis />
                <Table.Column title="对象" data-index="number" :width="140" />
                <Table.Column title="优先级" data-index="priority" :width="90" />
                <Table.Column title="创建时间" data-index="created_at" :width="180">
                  <template #default="{ text }">{{ timestamp(text) }}</template>
                </Table.Column>
                <Table.Column title="操作" :width="150">
                  <template #default="{ record }">
                    <Button type="text" @click="openReply(record)"><Edit3 :size="16" /></Button>
                    <Popconfirm title="确认删除？" @confirm="removeReply(record)"><Button type="text" danger><Trash2 :size="16" /></Button></Popconfirm>
                  </template>
                </Table.Column>
              </Table>
            </section>

            <section v-if="page === 'masters'" class="panel">
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Button type="primary" @click="masters.editing = true; masters.form = {}"><template #icon><Plus :size="16" /></template>新增管理员</Button>
                <Button @click="loadMasters"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Table :row-key="(row:any) => `${row.platform}.${row.number}`" :data-source="masters.rows">
                <Table.Column title="#" data-index="id" :width="64" />
                <Table.Column title="平台" data-index="platform" :width="140" />
                <Table.Column title="账号" data-index="number" :width="180" />
                <Table.Column title="昵称" data-index="nickname" />
                <Table.Column title="记录时间" data-index="unix" :width="180"><template #default="{ text }">{{ timestamp(text) }}</template></Table.Column>
                <Table.Column title="操作" :width="100"><template #default="{ record }"><Popconfirm title="确认删除？" @confirm="removeMaster(record)"><Button type="text" danger><Trash2 :size="16" /></Button></Popconfirm></template></Table.Column>
              </Table>
            </section>

            <section v-if="page === 'tasks'" class="panel">
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Button type="primary" @click="openTask()"><template #icon><Plus :size="16" /></template>新增定时任务</Button>
                <Button @click="loadTasks()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Table row-key="task_id" :data-source="tasks.rows" :pagination="{ total: tasks.total, pageSize: 20, onChange: loadTasks }">
                <Table.Column title="#" data-index="id" :width="64" />
                <Table.Column title="标题" data-index="title" :width="180" />
                <Table.Column title="Cron" data-index="schedule" :width="180" />
                <Table.Column title="命令" data-index="command" ellipsis />
                <Table.Column title="启用" data-index="enable" :width="80"><template #default="{ text }">{{ text ? '是' : '否' }}</template></Table.Column>
                <Table.Column title="创建时间" data-index="created_at" :width="180"><template #default="{ text }">{{ timestamp(text) }}</template></Table.Column>
                <Table.Column title="操作" :width="180"><template #default="{ record }"><Button type="text" @click="runTask(record)"><Play :size="16" /></Button><Button type="text" @click="openTask(record)">编辑</Button><Popconfirm title="确认删除？" @confirm="removeTask(record)"><Button type="text" danger><Trash2 :size="16" /></Button></Popconfirm></template></Table.Column>
              </Table>
            </section>

            <section v-if="page === 'carry'" class="panel">
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Button type="primary" @click="openCarry()"><template #icon><Plus :size="16" /></template>新增群组</Button>
                <Button @click="loadCarry()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Table row-key="chat_id" :data-source="carry.rows" :pagination="{ total: carry.total, pageSize: 20, onChange: loadCarry }">
                <Table.Column title="#" data-index="id" :width="64" />
                <Table.Column title="群号" data-index="chat_id" :width="160" />
                <Table.Column title="群名" data-index="chat_name" :width="180" />
                <Table.Column title="平台" data-index="platform" :width="100" />
                <Table.Column title="方向" :width="120"><template #default="{ record }">{{ `${record.in ? '采集 ' : ''}${record.out ? '转发' : ''}` }}</template></Table.Column>
                <Table.Column title="启用" data-index="enable" :width="80"><template #default="{ text }">{{ text ? '是' : '否' }}</template></Table.Column>
                <Table.Column title="操作" :width="150"><template #default="{ record }"><Button type="text" @click="openCarry(record)">编辑</Button><Popconfirm title="确认删除？" @confirm="removeCarry(record)"><Button type="text" danger><Trash2 :size="16" /></Button></Popconfirm></template></Table.Column>
              </Table>
            </section>

            <section v-if="page === 'qinglong'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Button type="primary" @click="openQinglongPanel()"><template #icon><Plus :size="16" /></template>添加青龙面板</Button>
                  <Button @click="loadQinglongPanels"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
                <Typography.Text class="muted">保存前会检测 /open/auth/token 是否可用。</Typography.Text>
              </div>
              <Table row-key="id" :loading="qinglong.loading" :data-source="qinglong.rows" :pagination="{ total: qinglong.total, pageSize: 20 }">
                <Table.Column title="#" :width="72">
                  <template #default="{ index }">{{ index + 1 }}</template>
                </Table.Column>
                <Table.Column title="名称" data-index="name" :width="180">
                  <template #default="{ record }">
                    <Typography.Text strong>{{ record.name || record.address }}</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="地址" data-index="address" ellipsis />
                <Table.Column title="Client ID" data-index="client_id" :width="220" ellipsis />
                <Table.Column title="状态" data-index="status" :width="120">
                  <template #default="{ record }">
                    <Tag :color="record.status === 'online' ? 'green' : 'default'">{{ record.status === 'online' ? '在线' : '未检测' }}</Tag>
                  </template>
                </Table.Column>
                <Table.Column title="最后检测" data-index="last_checked_at" :width="180">
                  <template #default="{ text }">{{ timestamp(text) }}</template>
                </Table.Column>
                <Table.Column title="操作" :width="210">
                  <template #default="{ record }">
                    <Button type="text" @click="testQinglongPanel(record)">检测</Button>
                    <Button type="text" @click="openQinglongPanel(record)">编辑</Button>
                    <Popconfirm title="确认删除这个青龙面板？" @confirm="removeQinglongPanel(record)">
                      <Button type="text" danger><Trash2 :size="16" /></Button>
                    </Popconfirm>
                  </template>
                </Table.Column>
              </Table>
            </section>

            <section v-if="page === 'smallcat'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Button type="primary" @click="openSmallcatPanel()"><template #icon><Plus :size="16" /></template>添加 smallcat</Button>
                  <Button @click="loadSmallcatPanels"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
                <Typography.Text class="muted">保存前会调用 /api/auth/validate，使用页面 API AUTH 一致的 auth 请求头验证。</Typography.Text>
              </div>
              <Table row-key="id" :loading="smallcat.loading" :data-source="smallcat.rows" :pagination="{ total: smallcat.total, pageSize: 20 }">
                <Table.Column title="#" :width="72">
                  <template #default="{ index }">{{ index + 1 }}</template>
                </Table.Column>
                <Table.Column title="名称" data-index="name" :width="180">
                  <template #default="{ record }">
                    <Typography.Text strong>{{ record.name || record.address }}</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="地址" data-index="address" ellipsis />
                <Table.Column title="API AUTH" data-index="api_auth" :width="180">
                  <template #default="{ text }">
                    <Typography.Text code>{{ maskSecret(text) }}</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="状态" data-index="status" :width="120">
                  <template #default="{ record }">
                    <Tag :color="record.status === 'online' ? 'green' : 'default'">{{ record.status === 'online' ? '验证通过' : '未检测' }}</Tag>
                  </template>
                </Table.Column>
                <Table.Column title="最后检测" data-index="last_checked_at" :width="180">
                  <template #default="{ text }">{{ timestamp(text) }}</template>
                </Table.Column>
                <Table.Column title="操作" :width="210">
                  <template #default="{ record }">
                    <Button type="text" @click="testSmallcatPanel(record)">检测</Button>
                    <Button type="text" @click="openSmallcatPanel(record)">编辑</Button>
                    <Popconfirm title="确认删除这个 smallcat？" @confirm="removeSmallcatPanel(record)">
                      <Button type="text" danger><Trash2 :size="16" /></Button>
                    </Popconfirm>
                  </template>
                </Table.Column>
              </Table>
            </section>

            <section v-if="page === 'plugins'" class="panel">
              <Tabs v-model:active-key="plugins.tab" :items="[{ key: 'all', label: `全部 ${plugins.meta.all ?? ''}` }, { key: 'tab1', label: `已安装 ${plugins.meta.tab1 ?? ''}` }, { key: 'tab2', label: `未安装 ${plugins.meta.tab2 ?? ''}` }, { key: 'tab3', label: `可更新 ${plugins.meta.tab3 ?? ''}` }]" />
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Input v-model:value="plugins.keyword" allow-clear style="width: 260px" placeholder="搜索插件或来源" @press-enter="loadPlugins()" />
                <Select v-model:value="plugins.klass" style="width: 140px" :options="Object.keys(plugins.meta.classes || { 全部: 0 }).map((value) => ({ value, label: value }))" />
                <Button type="primary" @click="loadPlugins()"><template #icon><Search :size="16" /></template>搜索</Button>
                <Button type="primary" @click="openPluginSourceManager">
                  <template #icon><Settings :size="16" /></template>管理插件源
                </Button>
                <Button @click="loadPlugins()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Table row-key="id" :loading="plugins.loading" :data-source="plugins.rows" :pagination="{ total: plugins.total, pageSize: 12, onChange: loadPlugins }">
                <Table.Column title="插件">
                  <template #default="{ record }">
                    <Space direction="vertical" size="small">
                      <Space wrap>
                        <Typography.Text strong>{{ record.title || record.id }}</Typography.Text>
                        <Tag v-if="record.status === 1" color="green">可更新</Tag>
                      </Space>
                      <Typography.Text class="muted">{{ record.description || '无描述' }}</Typography.Text>
                      <Typography.Text v-if="record.status === 1 && record.update_content" type="success">更新内容：{{ record.update_content }}</Typography.Text>
                      <Space wrap>
                        <Tag :color="pluginStatusColor(record)">{{ pluginStatusLabel(record) }}</Tag>
                        <Tag v-if="record.status === 1" color="green">新版本 {{ record.latest_version || record.version || '-' }} / 当前 {{ record.current_version || '-' }}</Tag>
                        <Tag v-else-if="record.version">{{ record.version }}</Tag>
                        <Tag v-if="record.author">{{ record.author }}</Tag>
                        <Tag v-if="record.organization" color="blue">{{ record.organization }}</Tag>
                        <Tag v-if="record.running" color="green">运行中</Tag>
                        <Tag v-if="record.disable" color="red">禁用</Tag>
                      </Space>
                    </Space>
                  </template>
                </Table.Column>
                <Table.Column title="操作" :width="140"><template #default="{ record }"><Button type="primary" :disabled="pluginActionDisabled(record)" :loading="plugins.installing[record.id]" @click="installPlugin(record)"><template #icon><Download :size="16" /></template>{{ pluginActionLabel(record) }}</Button></template></Table.Column>
              </Table>
            </section>

            <section v-if="page === 'plugin-configs'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Select
                    :value="pluginConfigs.selected?.uuid"
                    show-search
                    allow-clear
                    style="width: 360px"
                    placeholder="选择插件"
                    :options="pluginConfigOptions"
                    :filter-option="(input:any, option:any) => String(option?.label || '').toLowerCase().includes(String(input).toLowerCase())"
                    @change="selectPluginConfig"
                  />
                  <Button @click="loadPluginConfigs"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
                <div class="toolbar-right">
                  <Button type="primary" :disabled="!pluginConfigs.selected" @click="savePluginConfig"><template #icon><Save :size="16" /></template>保存配置</Button>
                </div>
              </div>
              <Spin :spinning="pluginConfigs.loading">
                <div v-if="pluginConfigs.selected" class="config-form">
                  <Typography.Title :level="4" style="margin-top: 0">{{ pluginConfigs.selected.plugin || pluginConfigs.selected.title }}</Typography.Title>
                  <Typography.Text class="muted mono">{{ pluginConfigs.selected.file || 'main.js' }}</Typography.Text>
                  <Form layout="vertical" style="margin-top: 16px">
                    <template v-for="field in schemaFields" :key="field.key">
                      <Form.Item :label="field.prop.title || field.key" :extra="field.prop.description">
                        <Select
                          v-if="fieldType(field.prop) === 'enum'"
                          v-model:value="pluginConfigs.form[field.key]"
                          :options="fieldOptions(field.prop)"
                        />
                        <Switch
                          v-else-if="fieldType(field.prop) === 'boolean'"
                          v-model:checked="pluginConfigs.form[field.key]"
                        />
                        <InputNumber
                          v-else-if="fieldType(field.prop) === 'number' || fieldType(field.prop) === 'integer'"
                          v-model:value="pluginConfigs.form[field.key]"
                          style="width: 100%"
                          :min="field.prop.minimum"
                          :max="field.prop.maximum"
                        />
                        <Input.TextArea
                          v-else-if="fieldType(field.prop) === 'object' || fieldType(field.prop) === 'array'"
                          v-model:value="pluginConfigs.text[field.key]"
                          :rows="6"
                          class="mono"
                        />
                        <Input.Password
                          v-else-if="field.prop.format === 'password' || field.prop['ui:widget'] === 'password'"
                          v-model:value="pluginConfigs.form[field.key]"
                        />
                        <Input.TextArea
                          v-else-if="field.prop.format === 'textarea' || field.prop['ui:widget'] === 'textarea'"
                          v-model:value="pluginConfigs.form[field.key]"
                          :rows="4"
                        />
                        <Input v-else v-model:value="pluginConfigs.form[field.key]" />
                      </Form.Item>
                    </template>
                  </Form>
                </div>
                <a-empty v-else :description="pluginConfigs.rows.length ? '请选择一个插件查看配置。' : '暂无插件配置。插件需运行一次并调用 new SillyGirlPluginConfig(schema) 或 Form(schema) 注册。'" />
              </Spin>
            </section>

            <section v-if="page === 'settings'" class="panel">
              <Form layout="vertical" style="max-width: 860px">
                <Form.Item label="后台账号名"><Input v-model:value="settings.form.name" /></Form.Item>
                <Form.Item label="修改密码"><Input.Password v-model:value="settings.form.password" placeholder="留空表示不修改" /></Form.Item>
                <Form.Item label="HTTP 端口"><InputNumber v-model:value="settings.form.port" style="width: 100%" :min="1" :max="65535" /></Form.Item>
                <Form.Item label="API Key"><Input v-model:value="settings.form.api_key" /></Form.Item>
                <Form.Item label="自动撤回正则"><Input.TextArea v-model:value="settings.form.recall" :rows="2" /></Form.Item>
                <Form.Item label="存储后端"><Select v-model:value="settings.form.storage" :options="storageBackendOptions" /></Form.Item>
                <template v-if="settings.form.storage === 'redis'">
                  <Form.Item label="Redis 地址"><Input v-model:value="settings.form.redis_addr" placeholder="127.0.0.1:6379" /></Form.Item>
                  <Form.Item label="Redis 密码"><Input.Password v-model:value="settings.form.redis_password" /></Form.Item>
                </template>
                <Form.Item label="调试模式"><Switch v-model:checked="settings.form.debug" /></Form.Item>
                <Form.Item label="未监听群允许管理员触发"><Switch v-model:checked="settings.form.listen_admin" /></Form.Item>
                <Button type="primary" @click="saveSettings"><template #icon><Save :size="16" /></template>保存设置</Button>
              </Form>
            </section>

            <section v-if="page === 'messages'" class="panel">
              <Tabs v-model:active-key="msgState.active" :items="Object.entries(messageBuckets).map(([key, item]) => ({ key, label: item.label }))" />
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Button type="primary" @click="openMessage()"><template #icon><Plus :size="16" /></template>新增</Button>
                <Button @click="loadMessages"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Table row-key="key" :data-source="msgState.rows">
                <Table.Column title="号码" data-index="key" :width="220" />
                <Table.Column title="平台" data-index="platform" :width="140" />
                <Table.Column title="说明" data-index="desc" />
                <Table.Column title="启用" data-index="enable" :width="90"><template #default="{ text }">{{ text ? '是' : '否' }}</template></Table.Column>
                <Table.Column title="操作" :width="150"><template #default="{ record }"><Button type="text" @click="openMessage(record)">编辑</Button><Popconfirm title="确认删除？" @confirm="removeMessageRow(record)"><Button type="text" danger><Trash2 :size="16" /></Button></Popconfirm></template></Table.Column>
              </Table>
            </section>
          </main>
        </Layout>
      </Layout>

      <Modal :open="!!replies.editing" title="回复规则" @cancel="replies.editing = null" @ok="saveReply">
        <Form layout="vertical"><Form.Item label="关键词/正则"><Input v-model:value="replies.form.keyword" /></Form.Item><Form.Item label="回复内容"><Input.TextArea v-model:value="replies.form.value" :rows="6" /></Form.Item><Form.Item label="限定用户/群号"><Input v-model:value="replies.form.number" /></Form.Item><Form.Item label="平台"><Select v-model:value="replies.form.platforms" mode="tags" /></Form.Item><Form.Item label="优先级"><InputNumber v-model:value="replies.form.priority" style="width: 100%" /></Form.Item></Form>
      </Modal>

      <Modal v-model:open="masters.editing" title="管理员" @cancel="masters.editing = false" @ok="saveMaster">
        <Form layout="vertical"><Form.Item label="平台"><Select v-model:value="masters.form.platform" :options="masters.platforms" /></Form.Item><Form.Item label="账号"><Input v-model:value="masters.form.number" /></Form.Item></Form>
      </Modal>

      <Modal :open="!!tasks.editing" title="定时任务" width="640px" @cancel="tasks.editing = null" @ok="saveTask">
        <Form layout="vertical"><Form.Item label="标题" required help="定时任务标题不能为空"><Input v-model:value="tasks.form.title" placeholder="例如：每小时检查 IP" /></Form.Item><Form.Item label="Cron 表达式" required help="例如：0 * * * *，也支持带秒字段的 6 段表达式"><Input v-model:value="tasks.form.schedule" placeholder="0 * * * *" /></Form.Item><Form.Item label="触发命令"><Select v-model:value="tasks.form.command" show-search :options="tasks.scripts" placeholder="node xxx.js" /></Form.Item><Form.Item label="启用"><Switch v-model:checked="tasks.form.enable" /></Form.Item></Form>
      </Modal>

      <Modal :open="!!carry.editing" title="搬运群组" width="820px" @cancel="carry.editing = null" @ok="saveCarry">
        <Form layout="vertical"><Form.Item label="群号"><Input v-model:value="carry.form.chat_id" /></Form.Item><Form.Item label="群名"><Input v-model:value="carry.form.chat_name" /></Form.Item><Form.Item label="平台"><Select v-model:value="carry.form.platform" :options="optionMap(carry.selects.platforms)" /></Form.Item><Form.Item label="工作机器人"><Select v-model:value="carry.form.bots_id" mode="multiple" :options="optionMap(carry.selects.bots_id)" /></Form.Item><Form.Item label="采集来源"><Select v-model:value="carry.form.from" mode="multiple" :options="recordOptions(carry.selects.group_names)" /></Form.Item><Form.Item label="处理脚本"><Select v-model:value="carry.form.scripts" mode="multiple" :options="recordOptions(carry.selects.scripts)" /></Form.Item><Form.Item label="包含词"><Input.TextArea v-model:value="carry.form.includeText" :rows="2" /></Form.Item><Form.Item label="排除词"><Input.TextArea v-model:value="carry.form.excludeText" :rows="2" /></Form.Item><Form.Item label="用户白名单"><Input.TextArea v-model:value="carry.form.allowedText" :rows="2" /></Form.Item><Form.Item label="用户黑名单"><Input.TextArea v-model:value="carry.form.prohibitedText" :rows="2" /></Form.Item><Form.Item label="备注"><Input.TextArea v-model:value="carry.form.remark" :rows="2" /></Form.Item><Form.Item label="采集"><Switch v-model:checked="carry.form.in" /></Form.Item><Form.Item label="转发"><Switch v-model:checked="carry.form.out" /></Form.Item><Form.Item label="启用"><Switch v-model:checked="carry.form.enable" /></Form.Item><Form.Item label="文本去重"><Switch v-model:checked="carry.form.deduplication" /></Form.Item></Form>
      </Modal>

      <Modal :open="plugins.sourceModal" title="管理插件源" width="820px" :footer="null" @cancel="plugins.sourceModal = false">
        <Space direction="vertical" style="width: 100%" size="middle">
          <Form layout="vertical">
            <Form.Item label="GitHub 加速" extra="用于读取 GitHub 插件源和下载 GitHub 插件；选择关闭表示直连。" style="margin-bottom: 12px">
              <Space.Compact style="width: 100%">
                <Select
                  v-model:value="plugins.githubProxy"
                  style="width: 100%"
                  :options="[
                    { value: '', label: '关闭加速' },
                    ...plugins.githubProxyOptions.map((value) => ({ value, label: value })),
                  ]"
                />
                <Button :loading="plugins.githubProxySaving" @click="saveGithubProxy">
                  保存
                </Button>
              </Space.Compact>
            </Form.Item>
            <Form.Item label="新增插件源" required style="margin-bottom: 0">
              <Space.Compact style="width: 100%">
                <Input
                  v-model:value="plugins.sourceAddress"
                  placeholder="https://github.com/smallfawn/sillyGirl_Plugins 或 link://..."
                  @press-enter="addPluginSource"
                />
                <Button type="primary" :loading="plugins.sourceSaving" @click="addPluginSource">
                  <template #icon><Plus :size="16" /></template>新增
                </Button>
              </Space.Compact>
            </Form.Item>
          </Form>

          <Table
            row-key="address"
            size="small"
            :data-source="plugins.sources.map((address) => ({ address }))"
            :pagination="false"
          >
            <Table.Column title="现有插件源" data-index="address" ellipsis>
              <template #default="{ text }">
                <Typography.Text>{{ text }}</Typography.Text>
              </template>
            </Table.Column>
            <Table.Column title="操作" :width="120">
              <template #default="{ record }">
                <Popconfirm title="确认删除这个插件源？" @confirm="removePluginSource(record.address)">
                  <Button type="text" danger :loading="plugins.sourceRemoving[record.address]">
                    <Trash2 :size="16" />
                  </Button>
                </Popconfirm>
              </template>
            </Table.Column>
          </Table>

          <div class="toolbar-right">
            <Button @click="plugins.sourceModal = false">关闭</Button>
          </div>
        </Space>
      </Modal>

      <Modal :open="!!qinglong.editing" title="青龙面板" width="720px" :confirm-loading="qinglong.saving" @cancel="qinglong.editing = null" @ok="saveQinglongPanel">
        <Form layout="vertical">
          <Form.Item label="名称">
            <Input v-model:value="qinglong.form.name" placeholder="例如：主青龙" />
          </Form.Item>
          <Form.Item label="青龙地址" required>
            <Input v-model:value="qinglong.form.address" placeholder="http://127.0.0.1:5700" />
          </Form.Item>
          <Form.Item label="Client ID" required>
            <Input v-model:value="qinglong.form.client_id" />
          </Form.Item>
          <Form.Item label="Client Secret" required>
            <Input.Password v-model:value="qinglong.form.client_secret" />
          </Form.Item>
          <Button @click="testQinglongPanel()" :loading="qinglong.testing">
            <template #icon><RefreshCw :size="16" /></template>检测连接
          </Button>
        </Form>
      </Modal>

      <Modal :open="!!smallcat.editing" title="smallcat" width="720px" :confirm-loading="smallcat.saving" @cancel="smallcat.editing = null" @ok="saveSmallcatPanel">
        <Form layout="vertical">
          <Form.Item label="名称">
            <Input v-model:value="smallcat.form.name" placeholder="例如：主 smallcat" />
          </Form.Item>
          <Form.Item label="smallcat 地址" required>
            <Input v-model:value="smallcat.form.address" placeholder="http://127.0.0.1:18787" />
          </Form.Item>
          <Form.Item label="API AUTH" required>
            <Input.Password v-model:value="smallcat.form.api_auth" />
          </Form.Item>
          <Button @click="testSmallcatPanel()" :loading="smallcat.testing">
            <template #icon><RefreshCw :size="16" /></template>检测连接
          </Button>
        </Form>
      </Modal>

      <Modal :open="!!msgState.editing" :title="messageBuckets[msgState.active].label" @cancel="msgState.editing = null" @ok="saveMessageRow">
        <Form layout="vertical"><Form.Item :label="msgState.active === 'private' ? '用户 ID' : '群号'"><Input v-model:value="msgState.form.key" :disabled="!!msgState.editing?.value" /></Form.Item><Form.Item label="平台"><Select v-model:value="msgState.form.platform" :options="msgState.platforms" /></Form.Item><Form.Item label="说明"><Input v-model:value="msgState.form.desc" /></Form.Item><Form.Item label="启用"><Switch v-model:checked="msgState.form.enable" /></Form.Item></Form>
      </Modal>
    </AntApp>
  </a-config-provider>
</template>
