<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { Compartment, EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { javascript } from '@codemirror/lang-javascript';
import { python } from '@codemirror/lang-python';
import { oneDark } from '@codemirror/theme-one-dark';
import { basicSetup } from 'codemirror';
import Alert from 'ant-design-vue/es/alert';
import AntApp from 'ant-design-vue/es/app';
import Button from 'ant-design-vue/es/button';
import Card from 'ant-design-vue/es/card';
import Col from 'ant-design-vue/es/col';
import ConfigProvider from 'ant-design-vue/es/config-provider';
import Drawer from 'ant-design-vue/es/drawer';
import Empty from 'ant-design-vue/es/empty';
import Form from 'ant-design-vue/es/form';
import Input from 'ant-design-vue/es/input';
import InputNumber from 'ant-design-vue/es/input-number';
import Layout from 'ant-design-vue/es/layout';
import Menu from 'ant-design-vue/es/menu';
import message from 'ant-design-vue/es/message';
import Modal from 'ant-design-vue/es/modal';
import Pagination from 'ant-design-vue/es/pagination';
import Popconfirm from 'ant-design-vue/es/popconfirm';
import Progress from 'ant-design-vue/es/progress';
import Row from 'ant-design-vue/es/row';
import Segmented from 'ant-design-vue/es/segmented';
import Select from 'ant-design-vue/es/select';
import Space from 'ant-design-vue/es/space';
import Spin from 'ant-design-vue/es/spin';
import Statistic from 'ant-design-vue/es/statistic';
import Switch from 'ant-design-vue/es/switch';
import Table from 'ant-design-vue/es/table';
import Tabs from 'ant-design-vue/es/tabs';
import Tag from 'ant-design-vue/es/tag';
import Typography from 'ant-design-vue/es/typography';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import QRCode from 'qrcode';
import {
  Bot,
  Boxes,
  ClipboardList,
  CloudDownload,
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
  QrCode,
  Radio,
  RefreshCw,
  Save,
  Search,
  Server,
  Settings,
  ShieldCheck,
  Trash2,
  User,
  Menu as MenuIcon,
  Wand2,
} from 'lucide-vue-next';
import { ApiError, clearAuthToken, del, get, post, put, readStorage, saveStorage, setAuthToken } from './api';
import type { AdminUserRow, CarryGroup, CurrentUser, DaidaiPanel, Master, PluginInfo, QinglongPanel, Reply, SmallcatPanel, Task } from './types';
import { timestamp } from './utils';

type ApiEnvelope<T> = {
  status?: boolean;
  message?: string;
  data: T;
};

function apiData<T>(res: ApiEnvelope<T> | T): T {
  if (res && typeof res === 'object' && 'data' in (res as Record<string, unknown>)) {
    return (res as ApiEnvelope<T>).data;
  }
  return res as T;
}

type PageKey =
  | 'welcome'
  | 'bots'
  | 'scripts'
  | 'dependencies'
  | 'plugins'
  | 'storage'
  | 'users'
  | 'tasks'
  | 'message-tools'
  | 'containers'
  | 'masters'
  | 'plugin-configs'
  | 'settings';

type ContainerKind = 'qinglong' | 'daidai' | 'smallcat';
type MessageToolKind = 'carry' | 'reply' | 'messages';

const validPages: PageKey[] = [
  'welcome',
  'bots',
  'scripts',
  'dependencies',
  'plugins',
  'storage',
  'users',
  'tasks',
  'message-tools',
  'containers',
  'masters',
  'plugin-configs',
  'settings',
];
const legacyContainerPages: ContainerKind[] = ['qinglong', 'daidai', 'smallcat'];
const legacyMessageToolPages: MessageToolKind[] = ['carry', 'reply', 'messages'];

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
const containerKind = ref<ContainerKind>(containerKindFromPath());
const messageToolKind = ref<MessageToolKind>(messageToolKindFromPath());
const selectedScriptId = ref(scriptIdFromPath());
const mobileMenuOpen = ref(false);
const loginModel = reactive({ username: '', password: '' });
const setupRequired = ref(false);
const setupModel = reactive({ username: '', password: '', confirm: '' });

type AuthResponse = {
  status: string;
  token?: string;
  expiresIn?: number;
};

const scripts = computed(() => user.value?.plugins || []);
const realScripts = computed(() => scripts.value.filter((item) => item.path?.startsWith('/script/') && !item.name?.startsWith('+')));
const scriptKeyword = ref('');
const overviewAdapters = computed(() => {
  const defaults = [
    { platform: 'clawbot', label: '微信 ClawBot' },
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
      enabled: row?.enabled !== false,
      manageable: row?.manageable !== false,
      bots_id: row?.bots_id || [],
      count: row?.count || 0,
    };
  });
});
const overviewIntegrations = computed(() => {
  const defaults = [
    { key: 'qinglong', label: '青龙容器' },
    { key: 'smallcat', label: 'smallcat' },
    { key: 'daidai', label: '呆呆容器' },
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
    local: info.local || '1.0.1',
    remote: info.remote || info.local || '1.0.1',
    source: info.source || 'reserved',
    repository: info.repository || 'https://github.com/smallfawn/sillyGirl',
  };
});
const overviewUserStats = computed(() => ({
  total: user.value?.user_stats?.total || 0,
  today: user.value?.user_stats?.today || 0,
}));

type SystemUpdateResult = {
  mode?: string;
  repo?: string;
  before?: string;
  after?: string;
  changed?: boolean;
  asset?: string;
  output?: string;
};

type SystemUpdateSnapshot = {
  running?: boolean;
  status?: 'idle' | 'running' | 'done' | 'error';
  percent?: number;
  message?: string;
  error?: string;
  result?: SystemUpdateResult | null;
};

const systemUpdate = reactive({
  open: false,
  running: false,
  restarting: false,
  restartChecking: false,
  percent: 0,
  status: 'idle' as 'idle' | 'running' | 'done' | 'error',
  message: '',
  result: null as SystemUpdateResult | null,
  timer: 0,
  restartTimer: 0,
});

function applySystemUpdateSnapshot(snapshot: SystemUpdateSnapshot) {
  systemUpdate.running = !!snapshot.running;
  systemUpdate.status = snapshot.status || (snapshot.running ? 'running' : 'idle');
  systemUpdate.percent = Math.max(0, Math.min(100, Number(snapshot.percent || 0)));
  systemUpdate.message = snapshot.error || snapshot.message || '';
  systemUpdate.result = snapshot.result || null;
  if (!systemUpdate.running) {
    window.clearInterval(systemUpdate.timer);
  }
}

async function startOnlineUpdate() {
  if (systemUpdate.running) return;
  systemUpdate.open = true;
  systemUpdate.running = true;
  systemUpdate.restarting = false;
  systemUpdate.status = 'running';
  systemUpdate.percent = 6;
  systemUpdate.result = null;
  systemUpdate.message = '正在连接 GitHub Release';
  window.clearInterval(systemUpdate.timer);
  try {
    const res = await post<ApiEnvelope<SystemUpdateSnapshot>>('/api/admin/system/update', {});
    applySystemUpdateSnapshot(apiData(res));
    systemUpdate.timer = window.setInterval(() => {
      pollSystemUpdateStatus().catch((error) => {
        systemUpdate.status = 'error';
        systemUpdate.message = error instanceof Error ? error.message : '读取更新状态失败';
        systemUpdate.running = false;
        window.clearInterval(systemUpdate.timer);
      });
    }, 1000);
    await pollSystemUpdateStatus();
  } catch (error) {
    systemUpdate.status = 'error';
    systemUpdate.running = false;
    systemUpdate.message = error instanceof Error ? error.message : '更新失败';
    message.error(systemUpdate.message);
    window.clearInterval(systemUpdate.timer);
  }
}

async function pollSystemUpdateStatus() {
  const res = await get<ApiEnvelope<SystemUpdateSnapshot>>('/api/admin/system/update/status');
  const snapshot = apiData(res);
  applySystemUpdateSnapshot(snapshot);
  if (snapshot.status === 'done') {
    await loadUser(false);
  }
}

async function restartAfterUpdate() {
  systemUpdate.restarting = true;
  systemUpdate.restartChecking = false;
  try {
    await post('/api/admin/system/restart', {});
    systemUpdate.restarting = false;
    systemUpdate.restartChecking = true;
    systemUpdate.status = 'running';
    systemUpdate.percent = 0;
    systemUpdate.message = '重启已触发，正在等待服务恢复';
    await waitForRestartReady();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '重启失败');
    systemUpdate.restartChecking = false;
    systemUpdate.status = 'error';
    systemUpdate.message = error instanceof Error ? error.message : '重启失败';
  } finally {
    systemUpdate.restarting = false;
  }
}

async function waitForRestartReady() {
  window.clearInterval(systemUpdate.restartTimer);
  const startedAt = Date.now();
  let attempts = 0;
  return new Promise<void>((resolve) => {
    systemUpdate.restartTimer = window.setInterval(async () => {
      attempts += 1;
      const elapsed = Math.floor((Date.now() - startedAt) / 1000);
      systemUpdate.percent = Math.min(95, 10 + attempts * 5);
      systemUpdate.message = `正在等待服务恢复，已等待 ${elapsed} 秒`;
      try {
        const res = await fetch(`/api/health?t=${Date.now()}`, { cache: 'no-store' });
        if (!res.ok) return;
        const body = await res.json().catch(() => null);
        if (!body || body.status === false) return;
        window.clearInterval(systemUpdate.restartTimer);
        systemUpdate.restartChecking = false;
        systemUpdate.status = 'done';
        systemUpdate.percent = 100;
        systemUpdate.message = '重启成功，服务已恢复';
        message.success('重启成功');
        await loadUser(false).catch(() => undefined);
        resolve();
      } catch (_) {
        if (elapsed >= 180) {
          window.clearInterval(systemUpdate.restartTimer);
          systemUpdate.restartChecking = false;
          systemUpdate.status = 'error';
          systemUpdate.message = '重启等待超时，请手动刷新页面确认服务状态';
          resolve();
        }
      }
    }, 1000);
  });
}

const menuItems = [
  { key: 'welcome', label: '概览', icon: () => h(Home, { size: 16 }) },
  { key: 'bots', label: 'BOT', icon: () => h(Bot, { size: 16 }) },
  { key: 'scripts', label: '脚本插件', icon: () => h(FileCode2, { size: 16 }) },
  { key: 'dependencies', label: '依赖管理', icon: () => h(Package, { size: 16 }) },
  { key: 'plugins', label: '插件市场', icon: () => h(Plug, { size: 16 }) },
  { key: 'plugin-configs', label: '插件配置', icon: () => h(Boxes, { size: 16 }) },
  { key: 'storage', label: '存储', icon: () => h(Database, { size: 16 }) },
  { key: 'users', label: '用户管理', icon: () => h(User, { size: 16 }) },
  { key: 'message-tools', label: '转发/回复/监听', icon: () => h(MessageSquare, { size: 16 }) },
  { key: 'tasks', label: '定时任务', icon: () => h(ClipboardList, { size: 16 }) },
  { key: 'containers', label: '容器管理', icon: () => h(Server, { size: 16 }) },
  { key: 'masters', label: '管理员', icon: () => h(ShieldCheck, { size: 16 }) },
  { key: 'settings', label: '基础设置', icon: () => h(Settings, { size: 16 }) },
];

function pageFromPath(): PageKey {
  const path = window.location.pathname.replace(/^\/admin\/?/, '/');
  if (path.startsWith('/script/')) return 'scripts';
  const key = path.split('/').filter(Boolean)[0] || 'welcome';
  if (legacyContainerPages.includes(key as ContainerKind)) return 'containers';
  if (legacyMessageToolPages.includes(key as MessageToolKind)) return 'message-tools';
  return validPages.includes(key as PageKey) ? (key as PageKey) : 'welcome';
}

function containerKindFromPath(): ContainerKind {
  const path = window.location.pathname.replace(/^\/admin\/?/, '/');
  const parts = path.split('/').filter(Boolean);
  const key = parts[0] === 'containers' ? parts[1] : parts[0];
  return legacyContainerPages.includes(key as ContainerKind) ? (key as ContainerKind) : 'qinglong';
}

function messageToolKindFromPath(): MessageToolKind {
  const path = window.location.pathname.replace(/^\/admin\/?/, '/');
  const parts = path.split('/').filter(Boolean);
  const key = parts[0] === 'message-tools' ? parts[1] : parts[0];
  return legacyMessageToolPages.includes(key as MessageToolKind) ? (key as MessageToolKind) : 'carry';
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
  const url = path || (next === 'welcome' ? '/admin/' : next === 'containers' ? `/admin/containers/${containerKind.value}` : next === 'message-tools' ? `/admin/message-tools/${messageToolKind.value}` : `/admin/${next}`);
  window.history.pushState({}, '', url);
  page.value = next;
  selectedScriptId.value = scriptIdFromPath();
  mobileMenuOpen.value = false;
}

async function loadSetupStatus() {
  const res = await get<ApiEnvelope<{ initialized: boolean }>>('/api/admin/setup/status');
  const data = apiData(res);
  setupRequired.value = !data?.initialized;
  return !!data?.initialized;
}

async function loadUser(setBooting = true, reloadSetupOnUnauthorized = true) {
  if (setBooting) booting.value = true;
  try {
    const res = await get<ApiEnvelope<CurrentUser>>('/api/admin/currentUser');
    user.value = apiData(res) || {};
    setupRequired.value = false;
  } catch (error) {
    if (error instanceof ApiError && error.status !== 401) message.error(error.message);
    user.value = null;
    if (error instanceof ApiError && error.status === 401) {
      clearAuthToken();
      if (reloadSetupOnUnauthorized) {
        await loadSetupStatus().catch(() => undefined);
      }
    }
  } finally {
    if (setBooting) booting.value = false;
  }
}

async function login() {
  try {
    const res = await post<ApiEnvelope<AuthResponse>>('/api/admin/login', loginModel);
    const auth = apiData(res);
    if (auth.status === 'setup_required') {
      setupRequired.value = true;
      message.error('请先设置管理员账号和密码');
      return;
    }
    if (auth.status !== 'ok' || !auth.token) {
      message.error('账号或密码不正确');
      return;
    }
    setAuthToken(auth.token, auth.expiresIn);
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
    const res = await post<ApiEnvelope<AuthResponse>>('/api/admin/register', { username: setupModel.username.trim(), password: setupModel.password });
    const auth = apiData(res);
    if (auth.status !== 'ok' || !auth.token) {
      message.error('账号创建失败');
      return;
    }
    setAuthToken(auth.token, auth.expiresIn);
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
  await post('/api/admin/outlogin').catch(() => undefined);
  clearAuthToken();
  user.value = null;
}

async function bootApp() {
  booting.value = true;
  try {
    const initialized = await loadSetupStatus();
    if (initialized) {
      await loadUser(false, false);
    } else {
      user.value = null;
    }
  } finally {
    booting.value = false;
  }
}

function handleAdminAuthExpired() {
  user.value = null;
  booting.value = false;
  window.clearInterval(systemUpdate.timer);
  window.clearInterval(systemUpdate.restartTimer);
  systemUpdate.running = false;
  systemUpdate.restarting = false;
  systemUpdate.restartChecking = false;
}

onMounted(() => {
  bootApp();
  window.addEventListener('popstate', () => {
    page.value = pageFromPath();
    containerKind.value = containerKindFromPath();
    messageToolKind.value = messageToolKindFromPath();
    selectedScriptId.value = scriptIdFromPath();
  });
  window.addEventListener('sillygirl:admin-auth-expired', handleAdminAuthExpired);
});

onBeforeUnmount(() => {
  window.removeEventListener('sillygirl:admin-auth-expired', handleAdminAuthExpired);
});

const scriptState = reactive({ content: '', loading: false });
const scriptCreateState = reactive({ open: false, fileName: '新脚本', suffix: '.js', saving: false });
const scriptEditorHost = ref<HTMLElement | null>(null);
const scriptEditorEditable = new Compartment();
const scriptEditorLanguage = new Compartment();
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

function scriptFileName(item?: { name?: string; path?: string; type?: string; file?: string }) {
  if (!item) return '-';
  if (isNewScriptEntry(item)) return 'new-script.js';
  if ('file' in item && item.file) return item.file.split(/[\\/]/).pop() || 'main.js';
  const title = scriptDisplayName(item)
    .replace(/[🔧💫🔒👑]/gu, '')
    .trim();
  const suffix = item.type === 'python' ? '.py' : '.js';
  return `${title || scriptFileId(item)}${suffix}`;
}

function isFileScript(item = currentScriptFile.value) {
  return item?.type === 'node' || item?.type === 'python';
}

function isPythonScript(item = currentScriptFile.value) {
  return item?.type === 'python' || /\.py$/i.test(scriptFileName(item));
}

function scriptRuntimeLabel(item = currentScriptFile.value) {
  if (item?.type === 'python' || isPythonScript(item)) return 'Python 3.12';
  if (item?.type === 'node') return 'NodeJS';
  return '旧脚本';
}

function scriptLanguageExtension() {
  return isPythonScript() ? python() : javascript();
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

function syncScriptEditorLanguage() {
  if (!scriptEditorView) return;
  scriptEditorView.dispatch({
    effects: scriptEditorLanguage.reconfigure(scriptLanguageExtension()),
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
        scriptEditorLanguage.of(scriptLanguageExtension()),
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
    syncScriptEditorLanguage();
  } else {
    destroyScriptEditor();
  }
}

async function loadScript(id = currentScriptId.value) {
  if (!id) return;
  scriptState.loading = true;
  try {
    if (isFileScript()) {
      const res = await get<ApiEnvelope<{ content: string }>>(`/api/admin/node/script?id=${encodeURIComponent(id)}`);
      scriptState.content = apiData(res)?.content || '';
    } else {
      const res = await readStorage<Record<string, string>>(`plugins.${id}`);
      scriptState.content = apiData(res)[`plugins.${id}`] || starter;
    }
  } finally {
    scriptState.loading = false;
  }
}

async function saveScript(value = scriptState.content) {
  if (!currentScriptId.value) return;
  if (isFileScript()) {
    await put('/api/admin/node/script', { id: currentScriptId.value, content: value });
  } else {
    await saveStorage({ [`plugins.${currentScriptId.value}`]: value }, currentScriptId.value);
  }
  message.success('脚本已保存');
  await loadUser();
}

async function formatScript() {
  if (!scriptState.content.trim()) return;
  if (isPythonScript()) {
    message.warning('Python 格式化暂未内置，请保存前自行格式化。');
    return;
  }
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
  if (isFileScript()) {
    await del('/api/admin/node/script', { id: currentScriptId.value });
  } else {
    await saveStorage({ [`plugins.${currentScriptId.value}`]: 'uninstall' });
  }
  message.success('脚本已卸载');
  await loadUser();
  navigate('scripts');
}

function openCreateScriptModal() {
  scriptCreateState.fileName = '新脚本';
  scriptCreateState.suffix = '.js';
  scriptCreateState.open = true;
}

function normalizeCreateScriptFileName() {
  const fileName = scriptCreateState.fileName.trim().replace(/\.(js|py)$/i, '');
  if (!fileName) return '';
  if (/[\\/:<>"|?*]/.test(fileName) || fileName.includes('..')) return fileName;
  return `${fileName}${scriptCreateState.suffix}`;
}

async function createScript() {
  const fileName = normalizeCreateScriptFileName();
  if (!fileName) {
    message.error('请输入脚本文件名');
    return;
  }
  if (/[\\/:<>"|?*]/.test(fileName) || fileName.includes('..')) {
    message.error('脚本文件名不合法');
    return;
  }
  scriptCreateState.saving = true;
  try {
    const res = await post<ApiEnvelope<{ id: string }>>('/api/admin/node/script', { name: fileName });
    const data = apiData(res);
    scriptCreateState.open = false;
    await loadUser();
    if (data.id) navigate('scripts', `/admin/script/${data.id}`);
  } finally {
    scriptCreateState.saving = false;
  }
}

function selectScriptFile(item: { path?: string; name?: string; type?: string; file?: string }) {
  const id = scriptFileId(item);
  if (!id) return;
  navigate('scripts', `/admin/script/${id}`);
  syncScriptEditorLanguage();
}

watch(currentScriptId, (id) => loadScript(id), { immediate: true });
watch([page, () => booting.value, () => user.value], () => ensureScriptEditor(), { immediate: true });
watch([currentScriptId, () => scriptState.loading], () => syncScriptEditorEditable());
watch(currentScriptFile, () => syncScriptEditorLanguage());
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
  clearClawbotLoginPoll();
  window.clearInterval(systemUpdate.timer);
  window.clearInterval(systemUpdate.restartTimer);
});

const storageState = reactive({
  keys: 'sillyGirl',
  newBucketName: '',
  createBucketOpen: false,
  entryBucket: 'sillyGirl',
  entryKey: '',
  entryValue: '',
  rows: [] as any[],
  buckets: [] as Array<{ value: string; label: string }>,
  loading: false,
  loadingBuckets: false,
  creatingBucket: false,
  savingEntry: false,
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
    const res = await get<ApiEnvelope<Array<{ value: string; text?: string }>>>('/api/admin/storage');
    storageState.buckets = (apiData(res) || []).map((item) => ({
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
    const res = await get<ApiEnvelope<{ list: any[]; total: number }>>(`/api/admin/storage/list?keys=${encodeURIComponent(storageState.keys)}`);
    storageState.rows = apiData(res)?.list || [];
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
async function openCreateStorageBucket() {
  storageState.newBucketName = '';
  storageState.createBucketOpen = true;
  if (!storageState.buckets.length) await loadStorageBuckets();
}
async function createStorageBucket() {
  const bucket = storageState.newBucketName.trim();
  if (!bucket) {
    message.error('请输入存储桶名称');
    return;
  }
  storageState.creatingBucket = true;
  try {
    await post('/api/admin/storage/bucket', { bucket });
    message.success('存储桶已创建');
    storageState.newBucketName = '';
    storageState.createBucketOpen = false;
    storageState.keys = bucket;
    await loadStorageBuckets();
    await loadStorage();
  } finally {
    storageState.creatingBucket = false;
  }
}
async function createStorageEntry() {
  const bucket = selectedStorageBucket.value || storageState.entryBucket.trim();
  const key = storageState.entryKey.trim();
  if (!bucket) {
    message.error('请先选择单个存储桶');
    return;
  }
  if (!key) {
    message.error('请输入 Key');
    return;
  }
  if (storageState.entryValue === '') {
    message.error('请输入 Value');
    return;
  }
  storageState.savingEntry = true;
  try {
    await saveStorage({ [`${bucket}.${key}`]: storageState.entryValue });
    message.success('Key/Value 已添加');
    storageState.entryBucket = bucket;
    storageState.entryKey = '';
    storageState.entryValue = '';
    storageState.keys = bucket;
    await loadStorageBuckets();
    await loadStorage();
  } finally {
    storageState.savingEntry = false;
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
    await del('/api/admin/storage/bucket', { bucket });
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
  const res = await get<ApiEnvelope<{ list: Reply[]; total: number }>>(`/api/admin/reply/list?current=${current}&pageSize=${pageSize}`);
  const data = apiData(res);
  replies.rows = data?.list || [];
  replies.total = data?.total || 0;
}
function openReply(row?: Reply) {
  replies.editing = row || { id: 0, priority: 0, platforms: [] };
  replies.form = { ...replies.editing };
}
async function saveReply() {
  await post('/api/admin/reply', replies.form);
  replies.editing = null;
  message.success('已保存');
  loadReplies();
}
async function removeReply(row: Reply) {
  await del(`/api/admin/reply?id=${row.id}`);
  message.success('已删除');
  loadReplies();
}

const masters = reactive({ rows: [] as Master[], platforms: [] as any[], editing: false, form: {} as Master });
async function loadMasters() {
  const res = await get<ApiEnvelope<{ list: Master[]; platforms: any[] }>>('/api/admin/master/list');
  const data = apiData(res);
  masters.rows = data?.list || [];
  masters.platforms = data?.platforms || [];
}
async function saveMaster() {
  await post('/api/admin/master', masters.form);
  masters.editing = false;
  message.success('已保存');
  loadMasters();
}
async function removeMaster(row: Master) {
  await del('/api/admin/master', row);
  message.success('已删除');
  loadMasters();
}

const normalUsers = reactive({ rows: [] as AdminUserRow[], total: 0, loading: false });
async function loadNormalUsers() {
  normalUsers.loading = true;
  try {
    const res = await get<ApiEnvelope<{ list: AdminUserRow[]; total: number }>>('/api/admin/users');
    const data = apiData(res);
    normalUsers.rows = data?.list || [];
    normalUsers.total = data?.total || normalUsers.rows.length;
  } finally {
    normalUsers.loading = false;
  }
}

const tasks = reactive({ rows: [] as Task[], total: 0, editing: null as Task | null, form: {} as any, scripts: [] as any[] });
async function loadTasks(current = 1, pageSize = 20) {
  const res = await get<ApiEnvelope<{ list: Task[]; total: number }>>(`/api/admin/tasks?current=${current}&pageSize=${pageSize}`);
  const data = apiData(res);
  tasks.rows = data?.list || [];
  tasks.total = data?.total || 0;
}
async function loadTaskSelects(taskId = '') {
  const res = await get<ApiEnvelope<{ scripts?: Record<string, string> }>>(`/api/admin/task/selects?task_id=${encodeURIComponent(taskId)}`);
  tasks.scripts = Object.entries(apiData(res)?.scripts || {})
    .filter(([, label]) => /\.(js|py)$/i.test(String(label)))
    .map(([, label]) => {
      const text = String(label);
      const runtime = /\.py$/i.test(text) ? 'python' : 'node';
      return { value: `${runtime} ${text}`, label: `${runtime} ${text}` };
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
  await post('/api/admin/tasks', payload);
  tasks.editing = null;
  message.success('已保存');
  loadTasks();
}
async function removeTask(row: Task) {
  await del('/api/admin/tasks', row);
  message.success('已删除');
  loadTasks();
}
async function runTask(row: Task) {
  await get(`/api/admin/tasks/run?task_id=${encodeURIComponent(row.task_id)}`);
  message.success('已触发');
}

const carry = reactive({ rows: [] as CarryGroup[], total: 0, editing: null as CarryGroup | null, form: {} as any, selects: {} as any });
async function loadCarry(current = 1, pageSize = 20) {
  const res = await get<ApiEnvelope<{ list: CarryGroup[]; total: number }>>(`/api/admin/carry/groups?current=${current}&pageSize=${pageSize}`);
  const data = apiData(res);
  carry.rows = data?.list || [];
  carry.total = data?.total || 0;
}
async function loadCarrySelects(row?: CarryGroup) {
  const res = await get<ApiEnvelope<any>>(
    `/api/admin/carry/group_selects?chat_id=${encodeURIComponent(row?.chat_id || '')}&platform=${encodeURIComponent(row?.platform || '')}`,
  );
  carry.selects = apiData(res) || {};
}
async function changeCarryPlatform(platform: string) {
  carry.form.platform = platform;
  carry.form.bots_id = [];
  await loadCarrySelects({ ...(carry.form as CarryGroup), platform });
}
async function openCarry(row?: CarryGroup) {
  const data = row || { chat_id: '', platform: '', remark: '', bots_id: [], scripts: [] };
  carry.editing = data;
  await loadCarrySelects(data);
  carry.form = {
    ...data,
    bots_id: data.bots_id || [],
    scripts: data.scripts || [],
  };
}
async function saveCarry() {
  if (!carry.form.chat_id?.trim()) {
    message.error('请输入群号');
    return;
  }
  if (!carry.form.platform) {
    message.error('请选择平台');
    return;
  }
  const payload = {
    chat_id: carry.form.chat_id.trim(),
    platform: carry.form.platform,
    remark: carry.form.remark || '',
    bots_id: carry.form.bots_id || [],
    scripts: carry.form.scripts || [],
  };
  await post('/api/admin/carry/group', payload);
  carry.editing = null;
  message.success('已保存');
  loadCarry();
}
async function removeCarry(row: CarryGroup) {
  await del('/api/admin/carry/group', row);
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
    const res = await get<ApiEnvelope<{ list: QinglongPanel[]; total: number }>>('/api/admin/qinglong/panels');
    const data = apiData(res);
    qinglong.rows = data?.list || [];
    qinglong.total = data?.total || 0;
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
    await post('/api/admin/qinglong/panel/test', panel);
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
    await post('/api/admin/qinglong/panel', qinglong.form);
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
  await del('/api/admin/qinglong/panel', row);
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
    const res = await get<ApiEnvelope<{ list: SmallcatPanel[]; total: number }>>('/api/admin/smallcat/panels');
    const data = apiData(res);
    smallcat.rows = data?.list || [];
    smallcat.total = data?.total || 0;
  } finally {
    smallcat.loading = false;
  }
}
function smallcatQuotaText(record: SmallcatPanel) {
  const used = `${record.account_used || ''}`.trim();
  const limit = `${record.account_limit || ''}`.trim();
  if (used && limit) return `${used} / ${limit}`;
  return used || limit || '-';
}
function openSmallcatPanel(row?: SmallcatPanel) {
  const data = row || { name: '', address: '', api_auth: '' };
  smallcat.editing = data;
  smallcat.form = { ...data };
}
async function testSmallcatPanel(panel = smallcat.form) {
  smallcat.testing = true;
  try {
    await post('/api/admin/smallcat/panel/test', panel);
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
    await post('/api/admin/smallcat/panel', smallcat.form);
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
  await del('/api/admin/smallcat/panel', row);
  message.success('已删除');
  loadSmallcatPanels();
}

const daidai = reactive({
  rows: [] as DaidaiPanel[],
  total: 0,
  loading: false,
  editing: null as DaidaiPanel | null,
  form: {} as DaidaiPanel,
  testing: false,
  saving: false,
});
async function loadDaidaiPanels() {
  daidai.loading = true;
  try {
    const res = await get<ApiEnvelope<{ list: DaidaiPanel[]; total: number }>>('/api/admin/daidai/panels');
    const data = apiData(res);
    daidai.rows = data?.list || [];
    daidai.total = data?.total || 0;
  } finally {
    daidai.loading = false;
  }
}
function openDaidaiPanel(row?: DaidaiPanel) {
  const data = row || { name: '', address: '', app_key: '', app_secret: '' };
  daidai.editing = data;
  daidai.form = { ...data };
}
async function testDaidaiPanel(panel = daidai.form) {
  daidai.testing = true;
  try {
    await post('/api/admin/daidai/panel/test', panel);
    message.success('呆呆面板连接成功');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '呆呆面板连接失败');
  } finally {
    daidai.testing = false;
  }
}
async function saveDaidaiPanel() {
  daidai.saving = true;
  try {
    await post('/api/admin/daidai/panel', daidai.form);
    daidai.editing = null;
    message.success('呆呆面板已添加');
    await loadDaidaiPanels();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '呆呆面板添加失败');
  } finally {
    daidai.saving = false;
  }
}
async function removeDaidaiPanel(row: DaidaiPanel) {
  await del('/api/admin/daidai/panel', row);
  message.success('已删除');
  loadDaidaiPanels();
}

const containerOptions = [
  { label: '青龙', value: 'qinglong' },
  { label: '呆呆', value: 'daidai' },
  { label: 'smallcat', value: 'smallcat' },
] as { label: string; value: ContainerKind }[];
const containerHelpText = computed(() => {
  if (containerKind.value === 'qinglong') return '保存前会检测 /open/auth/token 是否可用。';
  if (containerKind.value === 'daidai') return '保存前会调用 /api/open-api/token，使用 app_key/app_secret 验证 Open API。';
  return '保存前会调用 /api/auth/validate，使用页面 API AUTH 一致的 auth 请求头验证。';
});
const containerAddLabel = computed(() => {
  if (containerKind.value === 'qinglong') return '添加青龙面板';
  if (containerKind.value === 'daidai') return '添加呆呆面板';
  return '添加 smallcat';
});

function loadActiveContainerPanels() {
  if (containerKind.value === 'qinglong') return loadQinglongPanels();
  if (containerKind.value === 'daidai') return loadDaidaiPanels();
  return loadSmallcatPanels();
}

function openActiveContainerPanel() {
  if (containerKind.value === 'qinglong') {
    openQinglongPanel();
    return;
  }
  if (containerKind.value === 'daidai') {
    openDaidaiPanel();
    return;
  }
  openSmallcatPanel();
}

const plugins = reactive({
  rows: [] as PluginInfo[],
  total: 0,
  current: 1,
  pageSize: 12,
  tab: 'all',
  keyword: '',
  klass: '全部',
  meta: {} as any,
  loading: false,
  sources: [] as string[],
  sourceAddress: '',
  sourceSaving: false,
  sourceModal: false,
  sourceRemoving: {} as Record<string, boolean>,
  installing: {} as Record<string, boolean>,
});
const pluginClassOptions = computed(() => {
  const classes = (plugins.meta.class || {}) as Record<string, number>;
  const names = Object.keys(classes).filter(Boolean);
  if (!names.includes('全部')) {
    names.unshift('全部');
  }
  return names
    .sort((a, b) => {
      if (a === '全部') return -1;
      if (b === '全部') return 1;
      return a.localeCompare(b, 'zh-Hans-CN');
    })
    .map((value) => ({
      value,
      label: classes[value] === undefined ? value : `${value} (${classes[value]})`,
    }));
});
function filterPluginClassOption(input: string, option?: { label?: string; value?: string }) {
  const keyword = String(input || '').toLowerCase();
  return String(option?.label || option?.value || '').toLowerCase().includes(keyword);
}
function pluginClassTags(row: PluginInfo) {
  return String(row.class || '')
    .split(/[,，\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}
function pluginTriggerText(row: PluginInfo) {
  const rule = String(row.rule || '').trim();
  if (!rule) return '';
  return rule
    .replace(/^\^\\s\*\(?/, '')
    .replace(/\)?\\s\*\$$/, '')
    .replace(/^\^/, '')
    .replace(/\$$/, '')
    .replace(/\(\?:/g, '(')
    .replace(/^\((.*)\)$/, '$1')
    .replace(/\[Jj\]/g, 'J')
    .replace(/\[Dd\]/g, 'D')
    .replace(/\|/g, ' / ')
    .replace(/\\s\*/g, ' ')
    .replace(/\\s\+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim() || rule;
}
function pluginIconIsImage(row: PluginInfo) {
  const icon = String(row.icon || '').trim();
  return /^https?:\/\//i.test(icon) || icon.startsWith('/') || icon.startsWith('data:image/');
}
function pluginInitial(row: PluginInfo) {
  const text = String(row.title || row.id || 'P').trim();
  return (text ? text.slice(0, 1) : 'P').toUpperCase();
}
async function openPluginSourceManager() {
  plugins.sourceModal = true;
  await loadPluginSources();
}
async function loadPluginSources() {
  try {
    const res = await get<ApiEnvelope<string[]>>('/api/admin/plugins/sources');
    plugins.sources = apiData(res) || [];
  } catch {
    plugins.sources = [];
  }
}
async function loadPlugins(current = 1, pageSize = 12) {
  plugins.loading = true;
  try {
    plugins.current = current;
    plugins.pageSize = pageSize;
    const params = new URLSearchParams({
      current: String(current),
      pageSize: String(pageSize),
      activeKey: plugins.tab,
      keyword: plugins.keyword,
      class: plugins.klass,
    });
    const res = await get<ApiEnvelope<any>>(`/api/plugins/list.json?${params.toString()}`);
    const data = apiData(res) || {};
    plugins.rows = data.data || data.list || [];
    plugins.total = data.total || 0;
    plugins.meta = data;
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
    const res = await post<ApiEnvelope<{ count?: number }>>('/api/admin/plugins/source', { address });
    const data = apiData(res);
    plugins.sourceAddress = '';
    plugins.tab = 'all';
    message.success(`插件源已新增${data?.count ? `，发现 ${data.count} 个插件` : ''}`);
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
    await del('/api/admin/plugins/source', { address });
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
    const res = await put<ApiEnvelope<{ errors?: Record<string, string>; messages?: Record<string, string> }>>('/api/admin/storage', {
      [`plugins.${row.id}`]: 'install',
    });
    const data = apiData(res) || {};
    const firstError = Object.values(data.errors || {}).find(Boolean);
    if (firstError) {
      throw new ApiError(200, firstError);
    }
    const firstMessage = Object.values(data.messages || {}).find(Boolean);
    message.success(firstMessage || (row.status === 1 ? '已更新' : '已安装'));
    await Promise.all([loadPlugins(), loadUser(), loadPluginConfigs()]);
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
  type?: string;
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
  type?: string;
};

type DependencyRuntime = 'node' | 'python';
type DependencyToolStatus = { available: boolean; path?: string; message?: string; registry?: string; target?: string };

const dependencyRuntimeOptions = [
  { label: 'NodeJS', value: 'node' },
  { label: 'Python', value: 'python' },
];

const nodeDeps = reactive({
  runtime: 'node' as DependencyRuntime,
  plugins: [] as NodeDependencyPlugin[],
  plugin: '',
  rows: [] as NodeDependencyRow[],
  packageName: '',
  registry: 'https://registry.npmmirror.com',
  dev: false,
  loading: false,
  saving: false,
  removing: {} as Record<string, boolean>,
  pnpm: { available: false, path: '', message: '', registry: '' } as DependencyToolStatus,
  pipx: { available: false, path: '', message: '', registry: '', target: '' } as DependencyToolStatus,
});
const currentDependencyTool = computed(() => nodeDeps.runtime === 'python' ? nodeDeps.pipx : nodeDeps.pnpm);
const dependencyRuntimeLabel = computed(() => nodeDeps.runtime === 'python' ? 'Python' : 'NodeJS');
const dependencySharedPath = computed(() => nodeDeps.runtime === 'python' ? '/data/plugins/python_packages/venvs/sillygirl-python-runtime' : '/data/plugins/node_modules');
const dependencyRegistryLabel = computed(() => nodeDeps.runtime === 'python' ? 'pipx 源' : 'pnpm 镜像');
const dependencyPackagePlaceholder = computed(() => nodeDeps.runtime === 'python' ? '依赖名，例如 requests 或 requests==2.32.0' : '依赖名，例如 axios 或 ipp@latest');
const dependencyPluginOptions = computed(() => [
  { label: `全部 ${dependencyRuntimeLabel.value} 插件`, value: '' },
  ...nodeDeps.plugins.map((item) => ({ label: `${item.title || item.name} / ${item.file || scriptFileName(item)}`, value: item.name })),
]);
function showDependencyInstallResult(output: unknown) {
  const text = String(output || '').trim();
  if (text.includes('插件配置表单重载失败：')) {
    message.warning(text.slice(text.lastIndexOf('插件配置表单重载失败：')));
    return;
  }
  message.success('依赖已安装，插件配置表单已自动重载');
}
async function loadNodeDependencies(plugin = nodeDeps.plugin) {
  nodeDeps.loading = true;
  try {
    const query = new URLSearchParams({ runtime: nodeDeps.runtime });
    if (plugin) query.set('plugin', plugin);
    const res = await get<ApiEnvelope<{
      runtime: DependencyRuntime;
      plugins: NodeDependencyPlugin[];
      plugin: string;
      dependencies: NodeDependencyRow[];
      pnpm: DependencyToolStatus;
      pipx: DependencyToolStatus;
    }>>(`/api/admin/plugin/dependencies?${query.toString()}`);
    const data = apiData(res) || {};
    nodeDeps.plugins = data.plugins || [];
    nodeDeps.plugin = data.plugin || '';
    nodeDeps.rows = data.dependencies || [];
    nodeDeps.pnpm = data.pnpm || { available: false };
    nodeDeps.pipx = data.pipx || { available: false };
    nodeDeps.registry = currentDependencyTool.value.registry || (nodeDeps.runtime === 'python' ? 'https://pypi.tuna.tsinghua.edu.cn/simple' : 'https://registry.npmmirror.com');
  } finally {
    nodeDeps.loading = false;
  }
}
async function installNodeDependency() {
  await installNodeDependencyPackage(nodeDeps.packageName.trim(), () => {
    nodeDeps.packageName = '';
  });
}
async function installNodeDependencyPackage(pkg: string, after?: () => void) {
  if (!pkg) {
    message.error('请输入依赖名称');
    return;
  }
  nodeDeps.saving = true;
  try {
    const res = await post<ApiEnvelope<string>>('/api/admin/plugin/dependency', { runtime: nodeDeps.runtime, plugin: nodeDeps.plugin || '__shared__', package: pkg, dev: nodeDeps.dev });
    after?.();
    showDependencyInstallResult(apiData(res));
    await loadNodeDependencies(nodeDeps.plugin);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '依赖安装失败');
  } finally {
    nodeDeps.saving = false;
  }
}
async function installNodeDependencyRow(row: NodeDependencyRow) {
  nodeDeps.saving = true;
  try {
    const res = await post<ApiEnvelope<string>>('/api/admin/plugin/dependency', { runtime: nodeDeps.runtime, plugin: row.plugin || '__shared__', package: row.name, dev: row.dev });
    showDependencyInstallResult(apiData(res));
    await loadNodeDependencies(nodeDeps.plugin);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '依赖安装失败');
  } finally {
    nodeDeps.saving = false;
  }
}
async function removeNodeDependency(row: NodeDependencyRow) {
  const key = `${nodeDeps.runtime}.${row.plugin}.${row.name}`;
  nodeDeps.removing[key] = true;
  try {
    await del('/api/admin/plugin/dependency', { runtime: nodeDeps.runtime, plugin: row.plugin || '__shared__', package: row.name });
    message.success('依赖已卸载');
    await loadNodeDependencies(nodeDeps.plugin);
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
    const res = await get<ApiEnvelope<any[]>>('/api/admin/plugin/configs');
    pluginConfigs.rows = apiData(res) || [];
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
  await put('/api/admin/plugin/config', { uuid, value });
}

type BotSettingsForm = {
  clawbot_enable: boolean;
  clawbot_token: string;
  clawbot_api_base: string;
  clawbot_debug: boolean;
  qq_enable: boolean;
  qq_token: string;
  qq_debug: boolean;
  telegram_token: string;
  telegram_enable: boolean;
  telegram_api_base: string;
  telegram_debug: boolean;
  dingtalk_enable: boolean;
  dingtalk_client_id: string;
  dingtalk_client_secret: string;
  dingtalk_debug: boolean;
  qqguild_enable: boolean;
  qqguild_app_id: string;
  qqguild_app_secret: string;
  qqguild_sandbox: boolean;
  qqguild_debug: boolean;
  pagermaid_enable: boolean;
  pagermaid_token: string;
  pagermaid_debug: boolean;
};

type ClawbotLoginStart = {
  session: string;
  qrcode_url: string;
  expires_at: number;
  status: string;
  message: string;
};

type ClawbotLoginStatus = {
  session: string;
  status: string;
  message: string;
  need_verify: boolean;
  connected: boolean;
  already_bound: boolean;
  bot_id?: string;
  user_id?: string;
  base_url?: string;
};

const botSettings = reactive({
  loading: false,
  saving: false,
  form: {
    clawbot_enable: true,
    clawbot_token: '',
    clawbot_api_base: 'https://ilinkai.weixin.qq.com',
    clawbot_debug: false,
    qq_enable: true,
    qq_token: '',
    qq_debug: false,
    telegram_token: '',
    telegram_enable: true,
    telegram_api_base: 'https://api.telegram.org',
    telegram_debug: false,
    dingtalk_enable: true,
    dingtalk_client_id: '',
    dingtalk_client_secret: '',
    dingtalk_debug: false,
    qqguild_enable: true,
    qqguild_app_id: '',
    qqguild_app_secret: '',
    qqguild_sandbox: false,
    qqguild_debug: false,
    pagermaid_enable: true,
    pagermaid_token: '',
    pagermaid_debug: false,
  } as BotSettingsForm,
});
const clawbotLogin = reactive({
  open: false,
  starting: false,
  polling: false,
  session: '',
  qrcodeUrl: '',
  qrcodeImg: '',
  message: '',
  status: '',
  needVerify: false,
  verifyCode: '',
  expiresAt: 0,
});
let clawbotLoginPollTimer: ReturnType<typeof window.setTimeout> | null = null;

function clearClawbotLoginPoll() {
  if (clawbotLoginPollTimer) {
    window.clearTimeout(clawbotLoginPollTimer);
    clawbotLoginPollTimer = null;
  }
}

function scheduleClawbotLoginPoll(delay = 1000) {
  clearClawbotLoginPoll();
  clawbotLoginPollTimer = window.setTimeout(() => {
    void pollClawbotLogin();
  }, delay);
}

function closeClawbotLogin() {
  clawbotLogin.open = false;
  clearClawbotLoginPoll();
}

async function startClawbotLogin() {
  clearClawbotLoginPoll();
  clawbotLogin.open = true;
  clawbotLogin.starting = true;
  try {
    const res = await post<ApiEnvelope<ClawbotLoginStart>>('/api/admin/clawbot/login/start', {});
    const data = apiData(res);
    const qrcodeImg = data.qrcode_url
      ? await QRCode.toDataURL(data.qrcode_url, {
          errorCorrectionLevel: 'M',
          margin: 1,
          width: 260,
        })
      : '';
    Object.assign(clawbotLogin, {
      session: data.session,
      qrcodeUrl: data.qrcode_url,
      qrcodeImg,
      expiresAt: data.expires_at,
      status: data.status || 'wait',
      message: data.message || '请使用微信扫码',
      needVerify: false,
      verifyCode: '',
    });
    scheduleClawbotLoginPoll(300);
  } catch (error) {
    clawbotLogin.message = error instanceof Error ? error.message : '生成二维码失败';
    message.error(clawbotLogin.message);
  } finally {
    clawbotLogin.starting = false;
  }
}

async function pollClawbotLogin(verifyCode = '') {
  if (!clawbotLogin.session) return;
  clawbotLogin.polling = true;
  try {
    const query = new URLSearchParams({ session: clawbotLogin.session });
    if (verifyCode.trim()) query.set('verify_code', verifyCode.trim());
    const res = await get<ApiEnvelope<ClawbotLoginStatus>>(`/api/admin/clawbot/login/status?${query.toString()}`);
    const data = apiData(res);
    clawbotLogin.status = data.status || 'wait';
    clawbotLogin.message = data.message || '等待扫码';
    clawbotLogin.needVerify = !!data.need_verify;
    if (!clawbotLogin.open && !data.connected) {
      clearClawbotLoginPoll();
      return;
    }
    if (data.connected) {
      message.success('ClawBot Token 已保存');
      clearClawbotLoginPoll();
      await loadBots();
      await loadUser(false);
      return;
    }
    if (data.already_bound || ['expired', 'verify_code_blocked'].includes(data.status)) {
      clearClawbotLoginPoll();
      return;
    }
    if (!data.need_verify && clawbotLogin.open) {
      scheduleClawbotLoginPoll(1000);
    }
  } catch (error) {
    clearClawbotLoginPoll();
    clawbotLogin.message = error instanceof Error ? error.message : '轮询扫码状态失败';
    message.error(clawbotLogin.message);
  } finally {
    clawbotLogin.polling = false;
  }
}

async function submitClawbotVerifyCode() {
  const code = clawbotLogin.verifyCode.trim();
  if (!code) {
    message.error('请输入验证码');
    return;
  }
  clawbotLogin.needVerify = false;
  clawbotLogin.verifyCode = '';
  await pollClawbotLogin(code);
}

const botSettingsKeys = [
  'clawbot.enable',
  'clawbot.token',
  'clawbot.api_base',
  'clawbot.debug',
  'qq.enable',
  'qq.token',
  'qq.debug',
  'telegram.token',
  'telegram.enable',
  'telegram.api_base',
  'telegram.debug',
  'dingtalk.enable',
  'dingtalk.client_id',
  'dingtalk.client_secret',
  'dingtalk.debug',
  'qqguild.enable',
  'qqguild.app_id',
  'qqguild.app_secret',
  'qqguild.sandbox',
  'qqguild.debug',
  'pagermaid.enable',
  'pagermaid.token',
  'pagermaid.debug',
];
const botStatusRows = computed(() => overviewAdapters.value.filter((item) => item.platform !== 'web'));
const oneBotReceiveURL = computed(() => {
  const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const host = window.location.port === '5173' ? `${window.location.hostname}:8080` : window.location.host;
  return `${wsProtocol}://${host}/qq/receive`;
});
const qqGuildWebhookURL = computed(() => {
  const host = window.location.port === '5173' ? `${window.location.hostname}:8080` : window.location.host;
  return `${window.location.protocol}//${host}/qqguild/webhook`;
});
const pagermaidBridgeURL = computed(() => {
  const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const host = window.location.port === '5173' ? `${window.location.hostname}:8080` : window.location.host;
  const token = botSettings.form.pagermaid_token.trim();
  const query = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${wsProtocol}://${host}/pagermaid/receive${query}`;
});

function boolSetting(value: unknown, fallback = false) {
  if (value === undefined || value === null || value === '') return fallback;
  return value === true || String(value).toLowerCase() === 'true';
}

async function loadBots() {
  botSettings.loading = true;
  try {
    const res = await readStorage<Record<string, any>>(botSettingsKeys.join(','));
    const data = apiData(res) || {};
    Object.assign(botSettings.form, {
      clawbot_enable: boolSetting(data['clawbot.enable'], true),
      clawbot_token: data['clawbot.token'] || '',
      clawbot_api_base: data['clawbot.api_base'] || 'https://ilinkai.weixin.qq.com',
      clawbot_debug: boolSetting(data['clawbot.debug']),
      qq_enable: boolSetting(data['qq.enable'], true),
      qq_token: data['qq.token'] || '',
      qq_debug: boolSetting(data['qq.debug']),
      telegram_token: data['telegram.token'] || '',
      telegram_enable: boolSetting(data['telegram.enable'], true),
      telegram_api_base: data['telegram.api_base'] || 'https://api.telegram.org',
      telegram_debug: boolSetting(data['telegram.debug']),
      dingtalk_enable: boolSetting(data['dingtalk.enable'], true),
      dingtalk_client_id: data['dingtalk.client_id'] || '',
      dingtalk_client_secret: data['dingtalk.client_secret'] || '',
      dingtalk_debug: boolSetting(data['dingtalk.debug']),
      qqguild_enable: boolSetting(data['qqguild.enable'], true),
      qqguild_app_id: data['qqguild.app_id'] || '',
      qqguild_app_secret: data['qqguild.app_secret'] || '',
      qqguild_sandbox: boolSetting(data['qqguild.sandbox']),
      qqguild_debug: boolSetting(data['qqguild.debug']),
      pagermaid_enable: boolSetting(data['pagermaid.enable'], true),
      pagermaid_token: data['pagermaid.token'] || '',
      pagermaid_debug: boolSetting(data['pagermaid.debug']),
    });
  } finally {
    botSettings.loading = false;
  }
}

async function refreshBots() {
  await Promise.all([loadUser(false), loadBots()]);
}

async function saveBots() {
  const v = botSettings.form;
  botSettings.saving = true;
  try {
    await saveStorage({
      'clawbot.enable': !!v.clawbot_enable,
      'clawbot.token': v.clawbot_token || '',
      'clawbot.api_base': v.clawbot_api_base || 'https://ilinkai.weixin.qq.com',
      'clawbot.debug': !!v.clawbot_debug,
      'qq.enable': !!v.qq_enable,
      'qq.token': v.qq_token || '',
      'qq.debug': !!v.qq_debug,
      'telegram.token': v.telegram_token || '',
      'telegram.enable': !!v.telegram_enable,
      'telegram.api_base': v.telegram_api_base || 'https://api.telegram.org',
      'telegram.debug': !!v.telegram_debug,
      'dingtalk.enable': !!v.dingtalk_enable,
      'dingtalk.client_id': v.dingtalk_client_id || '',
      'dingtalk.client_secret': v.dingtalk_client_secret || '',
      'dingtalk.debug': !!v.dingtalk_debug,
      'qqguild.enable': !!v.qqguild_enable,
      'qqguild.app_id': v.qqguild_app_id || '',
      'qqguild.app_secret': v.qqguild_app_secret || '',
      'qqguild.sandbox': !!v.qqguild_sandbox,
      'qqguild.debug': !!v.qqguild_debug,
      'pagermaid.enable': !!v.pagermaid_enable,
      'pagermaid.token': v.pagermaid_token || '',
      'pagermaid.debug': !!v.pagermaid_debug,
    });
    message.success('BOT 配置已保存');
    await refreshBots();
  } finally {
    botSettings.saving = false;
  }
}

function botFormEnableKey(platform: string) {
  if (platform === 'clawbot') return 'clawbot_enable';
  if (platform === 'qq') return 'qq_enable';
  if (platform === 'telegram') return 'telegram_enable';
  if (platform === 'dingtalk') return 'dingtalk_enable';
  if (platform === 'qqguild') return 'qqguild_enable';
  if (platform === 'pagermaid') return 'pagermaid_enable';
  return '';
}

function botEnabled(row: { platform: string; enabled?: boolean }) {
  const key = botFormEnableKey(row.platform);
  if (key) return !!botSettings.form[key as keyof BotSettingsForm];
  return row.enabled !== false;
}

async function setBotEnabled(row: { platform: string; label: string; enabled?: boolean }, enabled: boolean) {
  const key = botFormEnableKey(row.platform);
  if (!key) return;
  const previous = botSettings.form[key as keyof BotSettingsForm];
  (botSettings.form as Record<string, unknown>)[key] = enabled;
  try {
    await saveStorage({ [`${row.platform}.enable`]: enabled });
    message.success(`${row.label}${enabled ? '已开启' : '已关闭'}`);
    await refreshBots();
  } catch (error) {
    (botSettings.form as Record<string, unknown>)[key] = previous;
    message.error(error instanceof Error ? error.message : `${row.label}操作失败`);
  }
}

const settings = reactive({ form: {} as any, githubProxyOptions: [] as string[] });
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
  const [res, githubProxyRes, pnpmRegistryRes, pipxRegistryRes] = await Promise.all([
    readStorage<Record<string, any>>(settingsKeys.join(',')),
    get<ApiEnvelope<{ proxy: string; options: string[] }>>('/api/admin/plugins/github-proxy').catch(() => ({ data: { proxy: '', options: [] } })),
    get<ApiEnvelope<{ registry: string }>>('/api/admin/node/dependency/registry').catch(() => ({ data: { registry: 'https://registry.npmmirror.com' } })),
    get<ApiEnvelope<{ registry: string }>>('/api/admin/plugin/dependency/registry?runtime=python').catch(() => ({ data: { registry: 'https://pypi.tuna.tsinghua.edu.cn/simple' } })),
  ]);
  const data = apiData(res) || {};
  const githubProxyData = apiData(githubProxyRes) || { proxy: '', options: [] };
  const pnpmRegistryData = apiData(pnpmRegistryRes) || { registry: 'https://registry.npmmirror.com' };
  const pipxRegistryData = apiData(pipxRegistryRes) || { registry: 'https://pypi.tuna.tsinghua.edu.cn/simple' };
  settings.githubProxyOptions = githubProxyData.options || [];
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
    github_proxy: githubProxyData.proxy || '',
    pnpm_registry: pnpmRegistryData.registry || 'https://registry.npmmirror.com',
    pipx_registry: pipxRegistryData.registry || 'https://pypi.tuna.tsinghua.edu.cn/simple',
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
  const [githubProxyRes, pnpmRegistryRes, pipxRegistryRes] = await Promise.all([
    put<ApiEnvelope<{ proxy?: string }>>('/api/admin/plugins/github-proxy', { proxy: String(v.github_proxy || '').trim() }),
    put<ApiEnvelope<{ registry?: string }>>('/api/admin/node/dependency/registry', { registry: String(v.pnpm_registry || '').trim() }),
    put<ApiEnvelope<{ registry?: string }>>('/api/admin/plugin/dependency/registry', { runtime: 'python', registry: String(v.pipx_registry || '').trim() }),
  ]);
  settings.form.github_proxy = apiData(githubProxyRes)?.proxy || '';
  settings.form.pnpm_registry = apiData(pnpmRegistryRes)?.registry || settings.form.pnpm_registry;
  settings.form.pipx_registry = apiData(pipxRegistryRes)?.registry || settings.form.pipx_registry;
  nodeDeps.pnpm.registry = settings.form.pnpm_registry;
  nodeDeps.pipx.registry = settings.form.pipx_registry;
  nodeDeps.registry = currentDependencyTool.value.registry || nodeDeps.registry;
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
  const res = await get<ApiEnvelope<{ list: any[] }>>(`/api/admin/storage/list?keys=${bucket}`);
  msgState.rows = (apiData(res)?.list || []).map((row) => {
    try {
      return { ...row, ...JSON.parse(row.value || '{}') };
    } catch {
      return row;
    }
  });
  const master = await get<ApiEnvelope<{ platforms?: any[] }>>('/api/admin/master/list').catch(() => ({ data: { platforms: [] } }));
  msgState.platforms = apiData(master)?.platforms || [];
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

const messageToolOptions = [
  { label: '转发', value: 'carry' },
  { label: '回复', value: 'reply' },
  { label: '监听', value: 'messages' },
] as { label: string; value: MessageToolKind }[];
const messageToolHelpText = computed(() => {
  if (messageToolKind.value === 'carry') return '选择平台、群号、工作机器人和处理脚本。';
  if (messageToolKind.value === 'reply') return '按关键词或正则维护自动回复规则。';
  return '维护监听群组、禁言群组和屏蔽用户。';
});
const messageToolAddLabel = computed(() => {
  if (messageToolKind.value === 'carry') return '新增转发群组';
  if (messageToolKind.value === 'reply') return '新增回复';
  return '新增监听规则';
});

function loadActiveMessageTool() {
  if (messageToolKind.value === 'carry') return loadCarry();
  if (messageToolKind.value === 'reply') return loadReplies();
  return loadMessages();
}

function openActiveMessageTool() {
  if (messageToolKind.value === 'carry') {
    openCarry();
    return;
  }
  if (messageToolKind.value === 'reply') {
    openReply();
    return;
  }
  openMessage();
}

watch([page, user], ([p]) => {
  if (!user.value) return;
  if (p === 'bots') loadBots();
  if (p === 'users') loadNormalUsers();
  if (p === 'masters') loadMasters();
  if (p === 'tasks') loadTasks();
  if (p === 'message-tools') loadActiveMessageTool();
  if (p === 'containers') loadActiveContainerPanels();
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
}, { immediate: true });
watch(containerKind, (kind) => {
  if (page.value !== 'containers') return;
  window.history.replaceState({}, '', `/admin/containers/${kind}`);
  loadActiveContainerPanels();
});
watch(messageToolKind, (kind) => {
  if (page.value !== 'message-tools') return;
  window.history.replaceState({}, '', `/admin/message-tools/${kind}`);
  loadActiveMessageTool();
});
watch(() => plugins.tab, () => loadPlugins());
watch(() => plugins.klass, () => loadPlugins());
watch(() => nodeDeps.plugin, (plugin) => {
  if (page.value === 'dependencies') loadNodeDependencies(plugin);
});
watch(() => nodeDeps.runtime, () => {
  nodeDeps.plugin = '';
  nodeDeps.packageName = '';
  if (page.value === 'dependencies') loadNodeDependencies('');
});
watch(() => msgState.active, () => {
  if (page.value === 'message-tools' && messageToolKind.value === 'messages') loadMessages();
});

function optionMap(values?: string[]) {
  return (values || []).map((value) => ({ value, label: value }));
}
function recordOptions(record?: Record<string, string>) {
  return Object.entries(record || {}).map(([value, label]) => ({ value, label }));
}
function smallcatOpenids(record?: AdminUserRow) {
  const rows = [] as string[];
  if (record?.bindings?.smallcat_openid) rows.push(record.bindings.smallcat_openid);
  for (const item of record?.bindings?.smallcat_openids || []) {
    if (item) rows.push(item);
  }
  return Array.from(new Set(rows.map((item) => item.trim()).filter(Boolean)));
}
</script>

<template>
  <ConfigProvider :locale="zhCN">
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
        <Layout.Sider class="desktop-sider" :width="220" theme="light">
          <div class="brand"><span class="brand-mark">S</span><span>SillyGirl</span></div>
          <Menu mode="inline" :selected-keys="[page]" :items="menuItems" style="border-inline-end: 0; padding-top: 8px" @click="(e:any) => navigate(e.key)" />
        </Layout.Sider>
        <Layout>
          <div class="topbar">
            <div class="topbar-title">
              <Button class="mobile-menu-button" type="text" @click="mobileMenuOpen = true">
                <template #icon><MenuIcon :size="18" /></template>
              </Button>
              <div class="topbar-heading">
              <Typography.Text strong>{{ menuItems.find((item) => item.key === page)?.label || '后台' }}</Typography.Text>
              <Typography.Text class="muted" style="margin-left: 10px">{{ user?.name || '傻妞' }}</Typography.Text>
              </div>
            </div>
            <Button class="logout-button" @click="logout"><template #icon><LogOut :size="16" /></template>退出</Button>
          </div>
          <main class="content">
            <section v-if="page === 'welcome'" class="panel">
              <Typography.Title :level="3" style="margin-top: 0">{{ user?.name || '傻妞' }}</Typography.Title>
              <Space wrap style="margin-bottom: 14px">
                <Tag color="blue">当前版本 {{ overviewVersion.local }}</Tag>
                <Tag color="green">最新版本 {{ overviewVersion.remote }}</Tag>
                <Typography.Link :href="overviewVersion.repository" target="_blank">GitHub</Typography.Link>
                <Button type="primary" size="small" :loading="systemUpdate.running" @click="startOnlineUpdate">
                  <template #icon><CloudDownload :size="15" /></template>
                  在线更新
                </Button>
              </Space>
              <Row :gutter="[12, 12]">
                <Col :xs="24" :sm="12" :md="8"><Card><Statistic title="脚本数量" :value="realScripts.length" /></Card></Col>
                <Col :xs="24" :sm="12" :md="8"><Card><Statistic title="今日新增用户" :value="overviewUserStats.today" /></Card></Col>
                <Col :xs="24" :sm="12" :md="8"><Card><Statistic title="总用户数量" :value="overviewUserStats.total" /></Card></Col>
                <Col :xs="24" :sm="12" :md="8"><Card><Statistic title="青龙容器" :value="overviewIntegrations.find((item) => item.key === 'qinglong')?.count || 0" /></Card></Col>
                <Col :xs="24" :sm="12" :md="8"><Card><Statistic title="smallcat" :value="overviewIntegrations.find((item) => item.key === 'smallcat')?.count || 0" /></Card></Col>
                <Col :xs="24" :sm="12" :md="8"><Card><Statistic title="呆呆容器" :value="overviewIntegrations.find((item) => item.key === 'daidai')?.count || 0" /></Card></Col>
              </Row>
            </section>

            <section v-if="page === 'bots'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Bot :size="16" />
                  <Typography.Text strong>BOT 对接管理</Typography.Text>
                </div>
                <div class="toolbar-right">
                  <Button @click="refreshBots"><template #icon><RefreshCw :size="16" /></template>刷新状态</Button>
                  <Button type="primary" :loading="botSettings.saving" @click="saveBots">
                    <template #icon><Save :size="16" /></template>保存 BOT 配置
                  </Button>
                </div>
              </div>
              <Spin :spinning="botSettings.loading">
                <Table :row-key="(row:any) => row.platform" :data-source="botStatusRows" :pagination="false" size="small">
                  <Table.Column title="平台" data-index="label" />
                  <Table.Column title="启用状态" :width="120">
                    <template #default="{ record }">
                      <Tag :color="botEnabled(record) ? 'green' : 'red'">{{ botEnabled(record) ? '已开启' : '已关闭' }}</Tag>
                    </template>
                  </Table.Column>
                  <Table.Column title="连接状态" :width="120">
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
                  <Table.Column title="操作" :width="120">
                    <template #default="{ record }">
                      <Button v-if="botEnabled(record)" size="small" danger @click="setBotEnabled(record, false)">关闭</Button>
                      <Button v-else size="small" type="primary" @click="setBotEnabled(record, true)">开启</Button>
                    </template>
                  </Table.Column>
                </Table>

                <div class="bot-config-grid">
                  <section class="bot-config-panel">
                    <div class="bot-config-header">
                      <Radio :size="16" />
                      <Typography.Text strong>微信 ClawBot</Typography.Text>
                      <Tag :color="botStatusRows.find((item) => item.platform === 'clawbot')?.online ? 'green' : 'default'">
                        {{ botStatusRows.find((item) => item.platform === 'clawbot')?.online ? '已连接' : '未连接' }}
                      </Tag>
                    </div>
                    <Form layout="vertical">
                      <Form.Item label="启用 ClawBot">
                        <Switch v-model:checked="botSettings.form.clawbot_enable" />
                      </Form.Item>
                      <Form.Item label="Token" extra="ClawBot / OpenClaw 微信通道的 iLink bot token。保存后适配器会自动重启。">
                        <Space.Compact style="width: 100%">
                          <Input.Password v-model:value="botSettings.form.clawbot_token" placeholder="请输入 ClawBot Token" />
                          <Button :loading="clawbotLogin.starting" @click="startClawbotLogin">
                            <template #icon><QrCode :size="16" /></template>
                            扫码获取
                          </Button>
                        </Space.Compact>
                      </Form.Item>
                      <Form.Item label="API 地址" extra="默认使用腾讯 iLink API；如果你有兼容反代地址可以填写在这里。">
                        <Input v-model:value="botSettings.form.clawbot_api_base" placeholder="https://ilinkai.weixin.qq.com" />
                      </Form.Item>
                      <Form.Item label="ClawBot 调试日志">
                        <Switch v-model:checked="botSettings.form.clawbot_debug" />
                      </Form.Item>
                    </Form>
                  </section>

                  <section class="bot-config-panel">
                    <div class="bot-config-header">
                      <Radio :size="16" />
                      <Typography.Text strong>QQ / OneBot</Typography.Text>
                      <Tag :color="botStatusRows.find((item) => item.platform === 'qq')?.online ? 'green' : 'default'">
                        {{ botStatusRows.find((item) => item.platform === 'qq')?.online ? '已连接' : '未连接' }}
                      </Tag>
                    </div>
                    <Form layout="vertical">
                      <Form.Item label="启用 QQ">
                        <Switch v-model:checked="botSettings.form.qq_enable" />
                      </Form.Item>
                      <Form.Item label="反向 WebSocket 地址">
                        <Input :value="oneBotReceiveURL" readonly>
                          <template #suffix><Typography.Text class="muted">NapCat 填这个 URL</Typography.Text></template>
                        </Input>
                      </Form.Item>
                      <Form.Item label="连接密钥" extra="需要和 NapCat / OneBot 客户端配置里的 accessToken 保持一致；公网部署建议必须填写。">
                        <Input.Password v-model:value="botSettings.form.qq_token" placeholder="请输入 QQ 连接密钥" />
                      </Form.Item>
                      <Form.Item label="QQ 调试日志">
                        <Switch v-model:checked="botSettings.form.qq_debug" />
                      </Form.Item>
                    </Form>
                  </section>

                  <section class="bot-config-panel">
                    <div class="bot-config-header">
                      <Radio :size="16" />
                      <Typography.Text strong>Telegram Bot</Typography.Text>
                      <Tag :color="botStatusRows.find((item) => item.platform === 'telegram')?.online ? 'green' : 'default'">
                        {{ botStatusRows.find((item) => item.platform === 'telegram')?.online ? '已连接' : '未连接' }}
                      </Tag>
                    </div>
                    <Form layout="vertical">
                      <Form.Item label="启用 Telegram">
                        <Switch v-model:checked="botSettings.form.telegram_enable" />
                      </Form.Item>
                      <Form.Item label="Token" extra="BotFather 提供的 Bot Token，保存后 Telegram 适配器会自动重启。">
                        <Input.Password v-model:value="botSettings.form.telegram_token" placeholder="123456:ABC-DEF..." />
                      </Form.Item>
                      <Form.Item label="代理 API" extra="Telegram Bot API 地址，默认 https://api.telegram.org；网络不通时填写兼容反代地址。">
                        <Input v-model:value="botSettings.form.telegram_api_base" placeholder="https://api.telegram.org" />
                      </Form.Item>
                      <Form.Item label="Telegram 调试日志">
                        <Switch v-model:checked="botSettings.form.telegram_debug" />
                      </Form.Item>
                    </Form>
                  </section>

                  <section class="bot-config-panel">
                    <div class="bot-config-header">
                      <Radio :size="16" />
                      <Typography.Text strong>钉钉机器人</Typography.Text>
                      <Tag :color="botStatusRows.find((item) => item.platform === 'dingtalk')?.online ? 'green' : 'default'">
                        {{ botStatusRows.find((item) => item.platform === 'dingtalk')?.online ? '已连接' : '未连接' }}
                      </Tag>
                    </div>
                    <Form layout="vertical">
                      <Form.Item label="启用钉钉">
                        <Switch v-model:checked="botSettings.form.dingtalk_enable" />
                      </Form.Item>
                      <Form.Item label="Client ID" extra="钉钉开放平台应用的 Client ID（原 AppKey）。适配器使用 Stream 模式，不需要公网回调地址。">
                        <Input v-model:value="botSettings.form.dingtalk_client_id" placeholder="dingxxxxxxxx" />
                      </Form.Item>
                      <Form.Item label="Client Secret">
                        <Input.Password v-model:value="botSettings.form.dingtalk_client_secret" placeholder="请输入 Client Secret" />
                      </Form.Item>
                      <Form.Item label="钉钉调试日志">
                        <Switch v-model:checked="botSettings.form.dingtalk_debug" />
                      </Form.Item>
                    </Form>
                  </section>

                  <section class="bot-config-panel">
                    <div class="bot-config-header">
                      <Radio :size="16" />
                      <Typography.Text strong>QQ 官方频道机器人</Typography.Text>
                      <Tag :color="botStatusRows.find((item) => item.platform === 'qqguild')?.online ? 'green' : 'default'">
                        {{ botStatusRows.find((item) => item.platform === 'qqguild')?.online ? '已连接' : '未连接' }}
                      </Tag>
                    </div>
                    <Form layout="vertical">
                      <Form.Item label="启用 QQ 官方频道">
                        <Switch v-model:checked="botSettings.form.qqguild_enable" />
                      </Form.Item>
                      <Form.Item label="Webhook 回调地址" extra="把该地址填入 QQ 开放平台机器人事件回调；HTTPS 由反向代理提供。">
                        <Input :value="qqGuildWebhookURL" readonly />
                      </Form.Item>
                      <Form.Item label="AppID">
                        <Input v-model:value="botSettings.form.qqguild_app_id" placeholder="请输入机器人 AppID" />
                      </Form.Item>
                      <Form.Item label="AppSecret">
                        <Input.Password v-model:value="botSettings.form.qqguild_app_secret" placeholder="请输入机器人 AppSecret" />
                      </Form.Item>
                      <Form.Item label="沙箱环境">
                        <Switch v-model:checked="botSettings.form.qqguild_sandbox" />
                      </Form.Item>
                      <Form.Item label="QQ 频道调试日志">
                        <Switch v-model:checked="botSettings.form.qqguild_debug" />
                      </Form.Item>
                    </Form>
                  </section>

                  <section class="bot-config-panel">
                    <div class="bot-config-header">
                      <Radio :size="16" />
                      <Typography.Text strong>Pagermaid</Typography.Text>
                      <Tag :color="botStatusRows.find((item) => item.platform === 'pagermaid')?.online ? 'green' : 'default'">
                        {{ botStatusRows.find((item) => item.platform === 'pagermaid')?.online ? '已连接' : '未连接' }}
                      </Tag>
                    </div>
                    <div class="bot-info-list">
                      <div>
                        <Typography.Text class="muted">启用 Pagermaid</Typography.Text>
                        <div><Switch v-model:checked="botSettings.form.pagermaid_enable" /></div>
                      </div>
                      <div>
                        <Typography.Text class="muted">连接密钥</Typography.Text>
                        <Input.Password v-model:value="botSettings.form.pagermaid_token" placeholder="可选，建议填写" />
                      </div>
                      <div>
                        <Typography.Text class="muted">Pagermaid 调试日志</Typography.Text>
                        <div><Switch v-model:checked="botSettings.form.pagermaid_debug" /></div>
                      </div>
                      <div>
                        <Typography.Text class="muted">桥接脚本</Typography.Text>
                        <Typography.Text class="mono block">adapters/pagermaid/sillyplus.py</Typography.Text>
                      </div>
                      <div>
                        <Typography.Text class="muted">WebSocket 地址</Typography.Text>
                        <Typography.Text class="mono block">{{ pagermaidBridgeURL }}</Typography.Text>
                      </div>
                    </div>
                  </section>
                </div>

              </Spin>
            </section>

            <Modal
              v-model:open="clawbotLogin.open"
              title="扫码获取 ClawBot Token"
              :footer="null"
              @cancel="closeClawbotLogin"
            >
              <div class="clawbot-login-modal">
                <div class="clawbot-qr-frame">
                  <Spin :spinning="clawbotLogin.starting">
                    <img v-if="clawbotLogin.qrcodeImg" :src="clawbotLogin.qrcodeImg" alt="ClawBot 登录二维码" />
                    <Empty v-else :description="clawbotLogin.message || '等待生成二维码'" />
                  </Spin>
                </div>
                <Typography.Text>{{ clawbotLogin.message || '请使用微信扫码' }}</Typography.Text>
                <div v-if="clawbotLogin.needVerify" class="clawbot-verify-row">
                  <Input v-model:value="clawbotLogin.verifyCode" placeholder="输入手机微信显示的数字" @pressEnter="submitClawbotVerifyCode" />
                  <Button type="primary" :loading="clawbotLogin.polling" @click="submitClawbotVerifyCode">确认</Button>
                </div>
                <Space>
                  <Button :loading="clawbotLogin.starting" @click="startClawbotLogin">
                    <template #icon><RefreshCw :size="16" /></template>
                    重新生成
                  </Button>
                  <Button @click="closeClawbotLogin">关闭</Button>
                </Space>
              </div>
            </Modal>

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
                  <Input id="script-file-search" name="script-file-search" v-model:value="scriptKeyword" allow-clear placeholder="搜索脚本文件">
                    <template #prefix><Search :size="15" /></template>
                  </Input>
                  <div class="script-file-actions">
                    <Button type="primary" block @click="openCreateScriptModal"><template #icon><Plus :size="16" /></template>新增脚本</Button>
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
                    <Empty v-if="scriptFileRows.length === 0" description="暂无脚本文件" />
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
                    <span>{{ scriptRuntimeLabel() }}</span>
                    <span>{{ scriptState.content.split('\n').length }} 行</span>
                    <span>{{ scriptState.content.length }} 字符</span>
                  </div>
                </div>
              </div>
            </section>

            <section v-if="page === 'dependencies'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Typography.Text class="muted">共 {{ nodeDeps.plugins.length }} 个 {{ dependencyRuntimeLabel }} 脚本插件，依赖共享到 {{ dependencySharedPath }}</Typography.Text>
                  <Typography.Text class="muted">{{ dependencyRegistryLabel }}：{{ nodeDeps.registry }}</Typography.Text>
                  <Typography.Text v-if="currentDependencyTool.message" type="danger">{{ currentDependencyTool.message }}</Typography.Text>
                </div>
                <div class="toolbar-right">
                  <Button @click="loadNodeDependencies()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
              </div>
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Segmented v-model:value="nodeDeps.runtime" :options="dependencyRuntimeOptions" />
                <Select
                  v-model:value="nodeDeps.plugin"
                  style="width: 260px"
                  show-search
                  :options="dependencyPluginOptions"
                  option-filter-prop="label"
                />
                <Input
                  v-model:value="nodeDeps.packageName"
                  style="width: 320px"
                  :placeholder="dependencyPackagePlaceholder"
                  @press-enter="installNodeDependency"
                />
                <Switch v-if="nodeDeps.runtime === 'node'" v-model:checked="nodeDeps.dev" checked-children="Dev" un-checked-children="Prod" />
                <Button type="primary" :disabled="!currentDependencyTool.available" :loading="nodeDeps.saving" @click="installNodeDependency">
                  <template #icon><Download :size="16" /></template>安装依赖
                </Button>
              </div>
              <Table :row-key="(row:any) => `${row.type || nodeDeps.runtime}.${row.plugin}.${row.name}`" :loading="nodeDeps.loading" :data-source="nodeDeps.rows" :pagination="{ pageSize: 20 }">
                <Table.Column title="#" :width="64">
                  <template #default="{ index }">{{ index + 1 }}</template>
                </Table.Column>
                <Table.Column title="插件" :width="180">
                  <template #default="{ record }"><Typography.Text>{{ record.plugin_title || record.plugin }}</Typography.Text></template>
                </Table.Column>
                <Table.Column title="文件名" :width="140">
                  <template #default="{ record }"><Typography.Text class="mono">{{ record.plugin_file || (nodeDeps.runtime === 'python' ? 'main.py' : 'main.js') }}</Typography.Text></template>
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
                  <template #default="{ record }"><Tag :color="nodeDeps.runtime === 'python' ? 'purple' : (record.dev ? 'blue' : 'green')">{{ nodeDeps.runtime === 'python' ? 'pipx' : (record.dev ? 'dev' : 'prod') }}</Tag></template>
                </Table.Column>
                <Table.Column title="操作" :width="130">
                  <template #default="{ record }">
                    <Button v-if="!record.installed" type="link" :disabled="!currentDependencyTool.available" :loading="nodeDeps.saving" @click="installNodeDependencyRow(record)">安装</Button>
                    <Popconfirm v-else title="确认卸载这个依赖？" @confirm="removeNodeDependency(record)">
                      <Button type="text" danger :loading="nodeDeps.removing[`${nodeDeps.runtime}.${record.plugin}.${record.name}`]"><Trash2 :size="16" /></Button>
                    </Popconfirm>
                  </template>
                </Table.Column>
              </Table>
              <Empty v-if="!nodeDeps.loading && nodeDeps.rows.length === 0" description="暂未识别到插件需要依赖。" />
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
                <Button :loading="storageState.creatingBucket" @click="openCreateStorageBucket">
                  <template #icon><Plus :size="16" /></template>新建桶
                </Button>
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
              <Space.Compact style="width: 100%; margin-bottom: 12px">
                <Input
                  v-model:value="storageState.entryKey"
                  style="width: 260px"
                  placeholder="Key"
                  :disabled="!selectedStorageBucket"
                  @press-enter="createStorageEntry"
                />
                <Input
                  v-model:value="storageState.entryValue"
                  placeholder="Value"
                  :disabled="!selectedStorageBucket"
                  @press-enter="createStorageEntry"
                />
                <Button type="primary" :loading="storageState.savingEntry" :disabled="!selectedStorageBucket" @click="createStorageEntry">
                  <template #icon><Plus :size="16" /></template>
                </Button>
              </Space.Compact>
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

            <section v-if="page === 'users'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Typography.Text strong>普通用户</Typography.Text>
                  <Tag>{{ normalUsers.total }}</Tag>
                </div>
                <Button @click="loadNormalUsers"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Table
                row-key="id"
                :loading="normalUsers.loading"
                :data-source="normalUsers.rows"
                :pagination="{ pageSize: 20, total: normalUsers.total }"
              >
                <Table.Column title="#" :width="72">
                  <template #default="{ index }">{{ index + 1 }}</template>
                </Table.Column>
                <Table.Column title="账号" data-index="username" :width="180">
                  <template #default="{ record }">
                    <Space direction="vertical" size="small">
                      <Typography.Text strong>{{ record.username }}</Typography.Text>
                      <Typography.Text class="muted">{{ record.nickname || '-' }}</Typography.Text>
                    </Space>
                  </template>
                </Table.Column>
                <Table.Column title="smallcat openid" :width="300">
                  <template #default="{ record }">
                    <Space v-if="smallcatOpenids(record).length" direction="vertical" size="small">
                      <Typography.Text
                        v-for="openid in smallcatOpenids(record)"
                        :key="openid"
                        class="mono"
                        :copyable="true"
                      >
                        {{ openid }}
                      </Typography.Text>
                    </Space>
                    <Typography.Text v-else class="muted">-</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="QQ" :width="150">
                  <template #default="{ record }">
                    <Typography.Text class="mono">{{ record.bindings?.qq || '-' }}</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="TGID" :width="170">
                  <template #default="{ record }">
                    <Typography.Text class="mono">{{ record.bindings?.telegram || '-' }}</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="绑定更新时间" :width="180">
                  <template #default="{ record }">{{ timestamp(record.bindings?.updated_at) }}</template>
                </Table.Column>
                <Table.Column title="注册时间" data-index="created_at" :width="180">
                  <template #default="{ text }">{{ timestamp(text) }}</template>
                </Table.Column>
                <Table.Column title="状态" :width="100">
                  <template #default="{ record }">
                    <Tag :color="record.disabled ? 'default' : 'green'">{{ record.disabled ? '禁用' : '正常' }}</Tag>
                  </template>
                </Table.Column>
              </Table>
            </section>

            <section v-if="page === 'message-tools'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Segmented v-model:value="messageToolKind" :options="messageToolOptions" />
                  <Button type="primary" @click="openActiveMessageTool()"><template #icon><Plus :size="16" /></template>{{ messageToolAddLabel }}</Button>
                  <Button @click="loadActiveMessageTool"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
                <Typography.Text class="muted">{{ messageToolHelpText }}</Typography.Text>
              </div>

              <Table v-if="messageToolKind === 'carry'" row-key="chat_id" :data-source="carry.rows" :pagination="{ total: carry.total, pageSize: 20, onChange: loadCarry }">
                <Table.Column title="#" data-index="id" :width="64" />
                <Table.Column title="平台" data-index="platform" :width="100" />
                <Table.Column title="群号" data-index="chat_id" :width="160" />
                <Table.Column title="备注" data-index="remark" />
                <Table.Column title="操作" :width="150"><template #default="{ record }"><Button type="text" @click="openCarry(record)">编辑</Button><Popconfirm title="确认删除？" @confirm="removeCarry(record)"><Button type="text" danger><Trash2 :size="16" /></Button></Popconfirm></template></Table.Column>
              </Table>

              <Table v-else-if="messageToolKind === 'reply'" :row-key="(row:any) => String(row.id)" :data-source="replies.rows" :pagination="{ total: replies.total, pageSize: 20, onChange: loadReplies }">
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

              <template v-else>
                <Tabs v-model:active-key="msgState.active" :items="Object.entries(messageBuckets).map(([key, item]) => ({ key, label: item.label }))" />
                <Table row-key="key" :data-source="msgState.rows">
                  <Table.Column title="号码" data-index="key" :width="220" />
                  <Table.Column title="平台" data-index="platform" :width="140" />
                  <Table.Column title="说明" data-index="desc" />
                  <Table.Column title="启用" data-index="enable" :width="90"><template #default="{ text }">{{ text ? '是' : '否' }}</template></Table.Column>
                  <Table.Column title="操作" :width="150"><template #default="{ record }"><Button type="text" @click="openMessage(record)">编辑</Button><Popconfirm title="确认删除？" @confirm="removeMessageRow(record)"><Button type="text" danger><Trash2 :size="16" /></Button></Popconfirm></template></Table.Column>
                </Table>
              </template>
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

            <section v-if="page === 'containers'" class="panel">
              <div class="toolbar">
                <div class="toolbar-left">
                  <Segmented v-model:value="containerKind" :options="containerOptions" />
                  <Button type="primary" @click="openActiveContainerPanel()"><template #icon><Plus :size="16" /></template>{{ containerAddLabel }}</Button>
                  <Button @click="loadActiveContainerPanels"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </div>
                <Typography.Text class="muted">{{ containerHelpText }}</Typography.Text>
              </div>

              <Table v-if="containerKind === 'qinglong'" row-key="id" :loading="qinglong.loading" :data-source="qinglong.rows" :pagination="{ total: qinglong.total, pageSize: 20 }">
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

              <Table v-else-if="containerKind === 'daidai'" row-key="id" :loading="daidai.loading" :data-source="daidai.rows" :pagination="{ total: daidai.total, pageSize: 20 }">
                <Table.Column title="#" :width="72">
                  <template #default="{ index }">{{ index + 1 }}</template>
                </Table.Column>
                <Table.Column title="名称" data-index="name" :width="180">
                  <template #default="{ record }">
                    <Typography.Text strong>{{ record.name || record.address }}</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="地址" data-index="address" ellipsis />
                <Table.Column title="App Key" data-index="app_key" :width="220" ellipsis />
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
                    <Button type="text" @click="testDaidaiPanel(record)">检测</Button>
                    <Button type="text" @click="openDaidaiPanel(record)">编辑</Button>
                    <Popconfirm title="确认删除这个呆呆面板？" @confirm="removeDaidaiPanel(record)">
                      <Button type="text" danger><Trash2 :size="16" /></Button>
                    </Popconfirm>
                  </template>
                </Table.Column>
              </Table>

              <Table v-else row-key="id" :loading="smallcat.loading" :data-source="smallcat.rows" :pagination="{ total: smallcat.total, pageSize: 20 }">
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
                <Table.Column title="用户组" data-index="group" :width="130">
                  <template #default="{ record }">
                    <Tag :color="record.group === 'VIP' ? 'gold' : record.group === 'PRO' ? 'blue' : record.group ? 'green' : 'default'">
                      {{ record.group || '-' }}
                    </Tag>
                  </template>
                </Table.Column>
                <Table.Column title="积分" data-index="credit_balance" :width="110">
                  <template #default="{ text }">
                    <Typography.Text>{{ text || '-' }}</Typography.Text>
                  </template>
                </Table.Column>
                <Table.Column title="账号额度" :width="120">
                  <template #default="{ record }">{{ smallcatQuotaText(record) }}</template>
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
                <Select
                  v-model:value="plugins.klass"
                  show-search
                  style="width: 180px"
                  placeholder="插件分类"
                  :options="pluginClassOptions"
                  :filter-option="filterPluginClassOption"
                />
                <Button type="primary" @click="loadPlugins()"><template #icon><Search :size="16" /></template>搜索</Button>
                <Button type="primary" @click="openPluginSourceManager">
                  <template #icon><Settings :size="16" /></template>管理插件源
                </Button>
                <Button @click="loadPlugins()"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Spin :spinning="plugins.loading">
                <div v-if="plugins.rows.length" class="plugin-market-grid">
                  <article v-for="record in plugins.rows" :key="record.id" class="plugin-market-card">
                    <div class="plugin-card-icon" aria-hidden="true">
                      <img v-if="pluginIconIsImage(record)" :src="record.icon" :alt="record.title || record.id" />
                      <span v-else>{{ pluginInitial(record) }}</span>
                    </div>
                    <div class="plugin-card-main">
                      <div class="plugin-card-title-row">
                        <Typography.Text strong class="plugin-card-title">{{ record.title || record.id }}</Typography.Text>
                        <Tag v-if="record.status === 1" color="green">可更新</Tag>
                      </div>
                      <Tag v-if="pluginTriggerText(record)" class="plugin-card-command">
                        <span class="plugin-card-command-text">口令 {{ pluginTriggerText(record) }}</span>
                      </Tag>
                      <Typography.Text class="plugin-card-desc">{{ record.desc || '无描述' }}</Typography.Text>
                      <Typography.Text v-if="record.status === 1 && record.update_content" type="success" class="plugin-card-update">
                        更新内容：{{ record.update_content }}
                      </Typography.Text>
                      <div class="plugin-card-meta">
                        <Tag :color="pluginStatusColor(record)">{{ pluginStatusLabel(record) }}</Tag>
                        <Tag v-if="record.status === 1" color="green">新版本 {{ record.latest_version || record.version || '-' }} / 当前 {{ record.current_version || '-' }}</Tag>
                        <Tag v-else-if="record.version">{{ record.version }}</Tag>
                        <Tag v-for="klass in pluginClassTags(record)" :key="klass" color="cyan">{{ klass }}</Tag>
                        <Tag v-if="record.author" color="blue">{{ record.author }}</Tag>
                        <Tag v-if="record.running" color="green">运行中</Tag>
                        <Tag v-if="record.disable" color="red">禁用</Tag>
                      </div>
                    </div>
                    <Button
                      class="plugin-card-download"
                      shape="circle"
                      :disabled="pluginActionDisabled(record)"
                      :loading="plugins.installing[record.id]"
                      :title="pluginActionLabel(record)"
                      :aria-label="pluginActionLabel(record)"
                      @click="installPlugin(record)"
                    >
                      <template #icon><CloudDownload :size="20" /></template>
                    </Button>
                  </article>
                </div>
                <Empty v-else description="暂无插件" />
                <Pagination
                  v-if="plugins.total > plugins.pageSize"
                  class="plugin-market-pagination"
                  :current="plugins.current"
                  :page-size="plugins.pageSize"
                  :total="plugins.total"
                  show-less-items
                  @change="loadPlugins"
                />
              </Spin>
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
                  <Button type="primary" :disabled="!pluginConfigs.selected || pluginConfigs.selected.registered === false" @click="savePluginConfig"><template #icon><Save :size="16" /></template>保存配置</Button>
                </div>
              </div>
              <Spin :spinning="pluginConfigs.loading">
                <div v-if="pluginConfigs.selected" class="config-form">
                  <Typography.Title :level="4" style="margin-top: 0">{{ pluginConfigs.selected.plugin || pluginConfigs.selected.title }}</Typography.Title>
                  <Typography.Text class="muted mono">{{ pluginConfigs.selected.file || 'main.js' }}</Typography.Text>
                  <Alert
                    v-if="pluginConfigs.selected.registered === false"
                    type="warning"
                    show-icon
                    style="margin-top: 16px"
                    message="该插件检测到配置代码，但安装时没有成功导出配置表单。请确认 new SillyGirlPluginConfig(schema) 或 form(schema) 在脚本顶层执行，且脚本初始化没有报错。"
                  />
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
                <Empty v-else :description="pluginConfigs.rows.length ? '请选择一个插件查看配置。' : '暂无插件配置。插件安装后会自动注册顶层声明的 SillyGirlPluginConfig/form 配置。'" />
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
                <Typography.Title :level="5">网络与镜像</Typography.Title>
                <Form.Item label="GitHub 加速" extra="用于读取 GitHub 插件源和下载 GitHub 插件；选择关闭表示直连。">
                  <Select
                    v-model:value="settings.form.github_proxy"
                    :options="[
                      { value: '', label: '关闭加速' },
                      ...settings.githubProxyOptions.map((value) => ({ value, label: value })),
                    ]"
                  />
                </Form.Item>
                <Form.Item label="pnpm 镜像" extra="用于安装和更新脚本插件的 NodeJS 依赖。">
                  <Input v-model:value="settings.form.pnpm_registry" placeholder="https://registry.npmmirror.com" />
                </Form.Item>
                <Form.Item label="pipx 源" extra="用于安装 Python 脚本插件依赖。">
                  <Input v-model:value="settings.form.pipx_registry" placeholder="https://pypi.tuna.tsinghua.edu.cn/simple" />
                </Form.Item>
                <Form.Item label="系统调试模式"><Switch v-model:checked="settings.form.debug" /></Form.Item>
                <Form.Item label="未监听群允许管理员触发"><Switch v-model:checked="settings.form.listen_admin" /></Form.Item>
                <Button type="primary" @click="saveSettings"><template #icon><Save :size="16" /></template>保存设置</Button>
              </Form>
            </section>

          </main>
        </Layout>

        <Drawer
          v-model:open="mobileMenuOpen"
          class="mobile-menu-drawer"
          placement="left"
          :width="280"
          :body-style="{ padding: 0 }"
          :closable="false"
        >
          <div class="brand"><span class="brand-mark">S</span><span>SillyGirl</span></div>
          <Menu mode="inline" :selected-keys="[page]" :items="menuItems" style="border-inline-end: 0; padding-top: 8px" @click="(e:any) => navigate(e.key)" />
        </Drawer>
      </Layout>

      <Modal
        v-model:open="systemUpdate.open"
        title="在线更新"
        :footer="null"
        :closable="!systemUpdate.running && !systemUpdate.restartChecking"
        :mask-closable="!systemUpdate.running && !systemUpdate.restartChecking"
      >
        <Space direction="vertical" style="width: 100%" size="middle">
          <Progress
            :percent="systemUpdate.percent"
            :status="systemUpdate.status === 'error' ? 'exception' : systemUpdate.status === 'done' ? 'success' : 'active'"
          />
          <Alert
            :type="systemUpdate.status === 'error' ? 'error' : systemUpdate.status === 'done' ? 'success' : 'info'"
            :message="systemUpdate.message || '准备更新'"
            show-icon
          />
          <div v-if="systemUpdate.result" class="update-result">
            <Typography.Text class="block">版本：{{ systemUpdate.result.before || '-' }} -> {{ systemUpdate.result.after || '-' }}</Typography.Text>
            <Typography.Text class="block">文件：{{ systemUpdate.result.asset || '-' }}</Typography.Text>
            <Typography.Text class="block muted">{{ systemUpdate.result.output || '' }}</Typography.Text>
          </div>
          <Space v-if="systemUpdate.status === 'done' && !systemUpdate.restartChecking" style="justify-content: flex-end; width: 100%">
            <Button @click="systemUpdate.open = false">关闭</Button>
            <Button v-if="systemUpdate.result" type="primary" :loading="systemUpdate.restarting" @click="restartAfterUpdate">立即重启</Button>
          </Space>
          <Space v-if="systemUpdate.status === 'error'" style="justify-content: flex-end; width: 100%">
            <Button @click="systemUpdate.open = false">关闭</Button>
          </Space>
        </Space>
      </Modal>

      <Modal
        v-model:open="scriptCreateState.open"
        title="新增脚本插件"
        :confirm-loading="scriptCreateState.saving"
        ok-text="确认创建"
        cancel-text="取消"
        @ok="createScript"
      >
        <Form layout="vertical">
          <Form.Item label="脚本名称" html-for="script-create-name" required extra="只填写名称，系统会按下面选择的类型自动添加后缀。">
            <Input id="script-create-name" v-model:value="scriptCreateState.fileName" placeholder="例如：daily-sign" @press-enter="createScript" />
          </Form.Item>
          <Form.Item label="脚本类型" html-for="script-create-suffix" required>
            <Select
              id="script-create-suffix"
              v-model:value="scriptCreateState.suffix"
              :options="[
                { label: 'NodeJS（.js）', value: '.js' },
                { label: 'Python（.py）', value: '.py' },
              ]"
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal :open="!!replies.editing" title="回复规则" @cancel="replies.editing = null" @ok="saveReply">
        <Form layout="vertical"><Form.Item label="关键词/正则"><Input v-model:value="replies.form.keyword" /></Form.Item><Form.Item label="回复内容"><Input.TextArea v-model:value="replies.form.value" :rows="6" /></Form.Item><Form.Item label="限定用户/群号"><Input v-model:value="replies.form.number" /></Form.Item><Form.Item label="平台"><Select v-model:value="replies.form.platforms" mode="tags" /></Form.Item><Form.Item label="优先级"><InputNumber v-model:value="replies.form.priority" style="width: 100%" /></Form.Item></Form>
      </Modal>

      <Modal v-model:open="masters.editing" title="管理员" @cancel="masters.editing = false" @ok="saveMaster">
        <Form layout="vertical"><Form.Item label="平台"><Select v-model:value="masters.form.platform" :options="masters.platforms" /></Form.Item><Form.Item label="账号"><Input v-model:value="masters.form.number" /></Form.Item></Form>
      </Modal>

      <Modal :open="!!tasks.editing" title="定时任务" width="640px" @cancel="tasks.editing = null" @ok="saveTask">
        <Form layout="vertical"><Form.Item label="标题" required help="定时任务标题不能为空"><Input v-model:value="tasks.form.title" placeholder="例如：每小时检查 IP" /></Form.Item><Form.Item label="Cron 表达式" required help="例如：0 * * * *，也支持带秒字段的 6 段表达式"><Input v-model:value="tasks.form.schedule" placeholder="0 * * * *" /></Form.Item><Form.Item label="触发命令"><Select v-model:value="tasks.form.command" show-search :options="tasks.scripts" placeholder="node xxx.js 或 python xxx.py" /></Form.Item><Form.Item label="启用"><Switch v-model:checked="tasks.form.enable" /></Form.Item></Form>
      </Modal>

      <Modal :open="!!carry.editing" title="搬运群组" width="820px" @cancel="carry.editing = null" @ok="saveCarry">
        <Form layout="vertical">
          <Form.Item label="平台" required>
            <Select v-model:value="carry.form.platform" :options="optionMap(carry.selects.platforms)" @change="changeCarryPlatform" />
          </Form.Item>
          <Form.Item label="群号" required>
            <Input v-model:value="carry.form.chat_id" />
          </Form.Item>
          <Form.Item label="备注">
            <Input.TextArea v-model:value="carry.form.remark" :rows="2" />
          </Form.Item>
          <Form.Item label="工作机器人">
            <Select v-model:value="carry.form.bots_id" mode="multiple" :options="optionMap(carry.selects.bots_id)" />
          </Form.Item>
          <Form.Item label="处理脚本">
            <Select v-model:value="carry.form.scripts" mode="multiple" :options="recordOptions(carry.selects.scripts)" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal :open="plugins.sourceModal" title="管理插件源" width="820px" :footer="null" @cancel="plugins.sourceModal = false">
        <Space direction="vertical" style="width: 100%" size="middle">
          <Form layout="vertical">
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

      <Modal :open="!!daidai.editing" title="呆呆面板" width="720px" :confirm-loading="daidai.saving" @cancel="daidai.editing = null" @ok="saveDaidaiPanel">
        <Form layout="vertical">
          <Form.Item label="名称">
            <Input v-model:value="daidai.form.name" placeholder="例如：主呆呆" />
          </Form.Item>
          <Form.Item label="呆呆面板地址" required>
            <Input v-model:value="daidai.form.address" placeholder="http://127.0.0.1:5701" />
          </Form.Item>
          <Form.Item label="App Key" required>
            <Input v-model:value="daidai.form.app_key" />
          </Form.Item>
          <Form.Item label="App Secret" required>
            <Input.Password v-model:value="daidai.form.app_secret" />
          </Form.Item>
          <Button @click="testDaidaiPanel()" :loading="daidai.testing">
            <template #icon><RefreshCw :size="16" /></template>检测连接
          </Button>
        </Form>
      </Modal>

      <Modal
        v-model:open="storageState.createBucketOpen"
        title="新建存储桶"
        ok-text="确认创建"
        cancel-text="取消"
        :confirm-loading="storageState.creatingBucket"
        @ok="createStorageBucket"
      >
        <Form layout="vertical">
          <Form.Item label="存储桶名称" required extra="不能包含点号、逗号或空白字符。">
            <Input v-model:value="storageState.newBucketName" placeholder="例如：myPlugin" @press-enter="createStorageBucket" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal :open="!!msgState.editing" :title="messageBuckets[msgState.active].label" @cancel="msgState.editing = null" @ok="saveMessageRow">
        <Form layout="vertical"><Form.Item :label="msgState.active === 'private' ? '用户 ID' : '群号'"><Input v-model:value="msgState.form.key" :disabled="!!msgState.editing?.value" /></Form.Item><Form.Item label="平台"><Select v-model:value="msgState.form.platform" :options="msgState.platforms" /></Form.Item><Form.Item label="说明"><Input v-model:value="msgState.form.desc" /></Form.Item><Form.Item label="启用"><Switch v-model:checked="msgState.form.enable" /></Form.Item></Form>
      </Modal>
    </AntApp>
  </ConfigProvider>
</template>
