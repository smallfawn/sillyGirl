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
import Tabs, { TabPane } from 'ant-design-vue/es/tabs';
import Tag from 'ant-design-vue/es/tag';
import Typography from 'ant-design-vue/es/typography';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import QRCode from 'qrcode';
import {
  Antenna,
  Bot,
  CircleCheck,
  CircleX,
  ClipboardList,
  CloudDownload,
  Database,
  Download,
  DoorOpen,
  Edit3,
  FileCode2,
  FolderOpen,
  Home,
  LogOut,
  MessageSquare,
  Package,
  Pause,
  Play,
  Plug,
  Plus,
  QrCode,
  RefreshCw,
  Save,
  Search,
  Send,
  Server,
  Settings,
  ShieldCheck,
  Trash2,
  User,
  X,
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

type WebChatMessage = {
  t?: string;
  c?: string;
  m?: string[];
};

type WebChatEntry = WebChatMessage & {
  id: number;
  own?: boolean;
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

const webChatMessagesEl = ref<HTMLElement | null>(null);
const webChat = reactive({
  open: false,
  input: '',
  sending: false,
  polling: false,
  error: '',
  unread: 0,
  messages: [] as WebChatEntry[],
});
const webChatRid = loadWebChatRid();
let webChatMessageID = 0;
let webChatPollGeneration = 0;
let webChatPollController: AbortController | null = null;

function loadWebChatRid() {
  const key = 'sillygirl_web_chat_rid';
  const current = sessionStorage.getItem(key)?.trim();
  if (current) return current;
  const suffix = typeof crypto?.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const rid = `admin-${suffix}`;
  sessionStorage.setItem(key, rid);
  return rid;
}

function webChatRows(res: ApiEnvelope<WebChatMessage[]> | WebChatMessage[]) {
  const rows = apiData(res);
  return Array.isArray(rows) ? rows : [];
}

async function scrollWebChatToBottom() {
  await nextTick();
  const el = webChatMessagesEl.value;
  if (el) el.scrollTop = el.scrollHeight;
}

function appendWebChatMessages(rows: WebChatMessage[], own = false) {
  for (const row of rows) {
    const content = `${row?.c || ''}`;
    const images = Array.isArray(row?.m) ? row.m.filter(Boolean) : [];
    if (!content && images.length === 0) continue;
    webChat.messages.push({
      id: ++webChatMessageID,
      t: row?.t || 'chat',
      c: content,
      m: images,
      own,
    });
  }
  if (!webChat.open) webChat.unread += rows.length;
  if (webChat.messages.length > 200) {
    webChat.messages.splice(0, webChat.messages.length - 200);
  }
  void scrollWebChatToBottom();
}

async function pollWebChat(generation: number) {
  if (!webChat.open || !user.value || generation !== webChatPollGeneration) return;
  const controller = new AbortController();
  webChatPollController = controller;
  webChat.polling = true;
  try {
    const res = await get<ApiEnvelope<WebChatMessage[]>>(
      `/api/web_chat?rid=${encodeURIComponent(webChatRid)}`,
      { signal: controller.signal },
    );
    if (generation !== webChatPollGeneration) return;
    webChat.error = '';
    appendWebChatMessages(webChatRows(res));
  } catch (error) {
    if (generation !== webChatPollGeneration) return;
    if (error instanceof DOMException && error.name === 'AbortError') return;
    webChat.error = error instanceof Error ? error.message : 'Web Bot 连接失败';
    await new Promise((resolve) => window.setTimeout(resolve, 1200));
  } finally {
    if (webChatPollController === controller) webChatPollController = null;
    if (generation === webChatPollGeneration) webChat.polling = false;
  }
  if (webChat.open && user.value && generation === webChatPollGeneration) {
    void pollWebChat(generation);
  }
}

function toggleWebChat() {
  webChatPollController?.abort();
  webChatPollController = null;
  webChat.open = !webChat.open;
  webChat.unread = 0;
  webChat.error = '';
  webChatPollGeneration += 1;
  if (webChat.open) {
    if (webChat.messages.length === 0) {
      appendWebChatMessages([{ t: 'notice', c: 'Web Bot 已连接，可以直接发送命令。' }]);
    }
    void pollWebChat(webChatPollGeneration);
  }
}

async function sendWebChat() {
  const content = webChat.input.trim();
  if (!content || webChat.sending) return;
  webChat.input = '';
  webChat.sending = true;
  webChat.error = '';
  appendWebChatMessages([{ t: 'chat', c: content }], true);
  try {
    await post<ApiEnvelope<WebChatMessage[]>>('/api/web_chat', { rid: webChatRid, ctt: content });
  } catch (error) {
    webChat.error = error instanceof Error ? error.message : '消息发送失败';
  } finally {
    webChat.sending = false;
    void scrollWebChatToBottom();
  }
}

function stopWebChat() {
  webChatPollController?.abort();
  webChatPollController = null;
  webChat.open = false;
  webChat.polling = false;
  webChatPollGeneration += 1;
}

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
    { platform: 'dingtalk', label: '钉钉机器人' },
    { platform: 'pagermaid', label: 'Pagermaid' },
    { platform: 'qq', label: 'QQ' },
    { platform: 'qqguild', label: 'QQ 官方频道机器人' },
    { platform: 'web', label: 'Web Bot' },
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
      manageable: row ? row.manageable !== false : item.platform !== 'web',
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
      local: info.local || '1.0.3',
      remote: info.remote || info.local || '1.0.3',
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
  if (key === 'plugin-configs') {
    window.history.replaceState({}, '', '/admin/plugins');
    return 'plugins';
  }
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
  stopWebChat();
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
  stopWebChat();
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
  stopWebChat();
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
  bucket: 'sillyGirl',
  search: '',
  newBucketName: '',
  createBucketOpen: false,
  entryBucket: 'sillyGirl',
  entryKey: '',
  entryValue: '',
  rows: [] as any[],
  current: 1,
  pageSize: 20,
  total: 0,
  buckets: [] as Array<{ value: string; label: string }>,
  loading: false,
  loadingBuckets: false,
  creatingBucket: false,
  savingEntry: false,
  deletingBucket: false,
});
const protectedStorageBuckets = new Set(['plugins', 'sillyGirl', 'auths']);
const selectedStorageBucket = computed(() => {
  return storageState.bucket.trim();
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
async function loadStorage(current = 1, pageSize = storageState.pageSize) {
  storageState.loading = true;
  try {
    const params = new URLSearchParams({
      keys: selectedStorageBucket.value,
      current: String(current),
      pageSize: String(pageSize),
    });
    const search = storageState.search.trim();
    if (search) params.set('search', search);
    const res = await get<ApiEnvelope<{ list: any[]; total: number; page?: number; pageSize?: number }>>(`/api/admin/storage/list?${params.toString()}`);
    const data = apiData(res);
    storageState.rows = data?.list || [];
    storageState.current = data?.page || current;
    storageState.pageSize = data?.pageSize || pageSize;
    storageState.total = data?.total || 0;
  } finally {
    storageState.loading = false;
  }
}
function changeStoragePage(pagination: { current?: number; pageSize?: number }) {
  return loadStorage(pagination.current || 1, pagination.pageSize || storageState.pageSize);
}
async function saveStorageRow(row: any) {
  await saveStorage({ [`${row.bucket}.${row.key}`]: row.value });
  message.success('已保存');
}
async function selectStorageBucket(bucket?: string) {
  storageState.bucket = bucket || '';
  storageState.search = '';
  if (!bucket) {
    storageState.rows = [];
    return;
  }
  await loadStorage(1);
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
    storageState.bucket = bucket;
    storageState.search = '';
    await loadStorageBuckets();
    await loadStorage(1);
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
    storageState.bucket = bucket;
    storageState.search = '';
    await loadStorageBuckets();
    await loadStorage(1);
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
    storageState.bucket = 'sillyGirl';
    storageState.search = '';
    await loadStorageBuckets();
    await loadStorage(1);
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

type NormalUserForm = {
  username: string;
  password: string;
  nickname: string;
  qq: string;
  telegram: string;
  smallcat_openids: string[];
  disabled: boolean;
};

const emptyNormalUserForm = (): NormalUserForm => ({
  username: '',
  password: '',
  nickname: '',
  qq: '',
  telegram: '',
  smallcat_openids: [],
  disabled: false,
});

const normalUsers = reactive({
  rows: [] as AdminUserRow[],
  total: 0,
  loading: false,
  modalOpen: false,
  editing: null as AdminUserRow | null,
  saving: false,
  deleting: {} as Record<string, boolean>,
  form: emptyNormalUserForm(),
});
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
function openNormalUser(row?: AdminUserRow) {
  normalUsers.editing = row || null;
  normalUsers.form = row
    ? {
        username: row.username,
        password: '',
        nickname: row.nickname || '',
        qq: row.bindings?.qq || '',
        telegram: row.bindings?.telegram || '',
        smallcat_openids: smallcatOpenids(row),
        disabled: !!row.disabled,
      }
    : emptyNormalUserForm();
  normalUsers.modalOpen = true;
}
async function saveNormalUser() {
  const form = normalUsers.form;
  const username = form.username.trim();
  if (!username) {
    message.warning('请输入账号');
    return;
  }
  if (!normalUsers.editing && form.password.length < 6) {
    message.warning('密码至少 6 位');
    return;
  }
  normalUsers.saving = true;
  try {
    const payload = {
      username,
      password: form.password,
      nickname: form.nickname.trim(),
      qq: form.qq.trim(),
      telegram: form.telegram.trim(),
      smallcat_openids: [...new Set(form.smallcat_openids.map((item) => String(item).trim()).filter(Boolean))],
      disabled: !!form.disabled,
    };
    if (normalUsers.editing) {
      await put('/api/admin/users', payload);
      message.success('账号已更新');
    } else {
      await post('/api/admin/users', payload);
      message.success('账号已新增');
    }
    normalUsers.modalOpen = false;
    await loadNormalUsers();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存账号失败');
  } finally {
    normalUsers.saving = false;
  }
}
async function removeNormalUser(row: AdminUserRow) {
  normalUsers.deleting[row.id] = true;
  try {
    await del('/api/admin/users', { username: row.username });
    message.success('账号已删除');
    await loadNormalUsers();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '删除账号失败');
  } finally {
    normalUsers.deleting[row.id] = false;
  }
}

const taskPlatformLabels: Record<string, string> = {
  clawbot: '微信 ClawBot',
  qq: 'QQ',
  telegram: 'Telegram Bot',
  dingtalk: '钉钉机器人',
  qqguild: 'QQ 官方频道机器人',
  web: 'Web Bot',
  pagermaid: 'Pagermaid',
};
const tasks = reactive({
  rows: [] as Task[],
  total: 0,
  editing: null as Task | null,
  form: {} as any,
  scripts: [] as any[],
  platforms: [] as Array<{ value: string; label: string }>,
  platformBots: {} as Record<string, string[]>,
});
async function loadTasks(current = 1, pageSize = 20) {
  const res = await get<ApiEnvelope<{ list: Task[]; total: number }>>(`/api/admin/tasks?current=${current}&pageSize=${pageSize}`);
  const data = apiData(res);
  tasks.rows = data?.list || [];
  tasks.total = data?.total || 0;
}
async function loadTaskSelects(taskId = '') {
  const res = await get<ApiEnvelope<{ scripts?: Record<string, string>; platforms?: Record<string, string[]> }>>(`/api/admin/task/selects?task_id=${encodeURIComponent(taskId)}`);
  const data = apiData(res);
  tasks.scripts = Object.entries(data?.scripts || {})
    .filter(([, label]) => /\.(js|py)$/i.test(String(label)))
    .map(([, label]) => {
      const text = String(label);
      const runtime = /\.py$/i.test(text) ? 'python' : 'node';
      return { value: `${runtime} ${text}`, label: `${runtime} ${text}` };
    });
  tasks.platformBots = data?.platforms || {};
  const platformNames = new Set([...Object.keys(taskPlatformLabels), ...Object.keys(tasks.platformBots)]);
  tasks.platforms = [...platformNames].map((platform) => ({
    value: platform,
    label: taskPlatformLabels[platform] || platform,
  }));
}
async function openTask(row?: Task) {
  const data = row || { enable: true, command: '' };
  tasks.editing = data;
  await loadTaskSelects(data.task_id || '');
  const target = data.senders?.[0];
  tasks.form = {
    ...data,
    platform: target?.platform || '',
    recipient: target?.user_id || target?.chat_id || '',
  };
  if (tasks.form.platform && !tasks.platforms.some((item) => item.value === tasks.form.platform)) {
    tasks.platforms.push({ value: tasks.form.platform, label: taskPlatformLabels[tasks.form.platform] || tasks.form.platform });
  }
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
  const platform = `${tasks.form.platform || ''}`.trim();
  const recipient = `${tasks.form.recipient || ''}`.trim();
  if ((platform && !recipient) || (!platform && recipient)) {
    message.error('平台和接收人必须同时填写');
    return;
  }
  const payload = {
    task_id: tasks.form.task_id,
    title: `${tasks.form.title || ''}`.trim(),
    schedule: `${tasks.form.schedule || ''}`.trim(),
    command: tasks.form.command,
    enable: tasks.form.enable,
    senders: platform && recipient ? [{ platform, user_id: recipient }] : [],
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
	accountLoadingID: '',
	accountsOpen: false,
	accountPanelName: '',
	accountOpenids: [] as string[],
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

async function showSmallcatOpenids(row: SmallcatPanel) {
  const id = `${row.id || ''}`.trim();
  if (!id) {
    message.error('smallcat ID 缺失');
    return;
  }
  smallcat.accountLoadingID = id;
  try {
    const res = await post<ApiEnvelope<{ openids: string[]; total: number }>>('/api/admin/smallcat/panel/accounts', { id });
    const data = apiData(res);
    smallcat.accountOpenids = Array.isArray(data?.openids) ? data.openids : [];
    smallcat.accountPanelName = `${row.name || row.address || 'smallcat'}`;
    smallcat.accountsOpen = true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : 'OpenID 列表读取失败');
  } finally {
    smallcat.accountLoadingID = '';
  }
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
  uninstalling: {} as Record<string, boolean>,
  opening: {} as Record<string, boolean>,
  requestId: 0,
  detailOpen: false,
  detail: null as PluginInfo | null,
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
function pluginDependencies(row: PluginInfo) {
  return [...new Set((row.dependencies || []).map((item) => String(item).trim()).filter(Boolean))];
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
function openPluginDetail(row: PluginInfo) {
  plugins.detail = row;
  plugins.detailOpen = true;
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
async function loadPlugins(current = 1, pageSize = 12, refresh = false) {
  const requestId = ++plugins.requestId;
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
      init: refresh ? 'true' : 'false',
    });
    const res = await get<ApiEnvelope<any>>(`/api/plugins/list.json?${params.toString()}`);
    if (requestId !== plugins.requestId) return;
    const data = apiData(res) || {};
    plugins.rows = data.data || data.list || [];
    plugins.total = data.total || 0;
    plugins.meta = data;
  } finally {
    if (requestId === plugins.requestId) plugins.loading = false;
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
    try {
      await offerPluginDependencyInstall(row);
    } catch (error) {
      message.warning(`插件已安装，但依赖检测失败：${error instanceof Error ? error.message : '未知错误'}`);
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '插件安装失败');
  } finally {
    plugins.installing[row.id] = false;
  }
}
async function uninstallPlugin(row: PluginInfo) {
  plugins.uninstalling[row.id] = true;
  try {
    const res = await put<ApiEnvelope<{ errors?: Record<string, string>; messages?: Record<string, string> }>>('/api/admin/storage', {
      [`plugins.${row.id}`]: 'uninstall',
    });
    const data = apiData(res) || {};
    const firstError = Object.values(data.errors || {}).find(Boolean);
    if (firstError) {
      throw new ApiError(200, firstError);
    }
    const firstMessage = Object.values(data.messages || {}).find(Boolean);
    if (pluginConfigs.selected?.uuid === row.id) {
      pluginConfigs.modalOpen = false;
      pluginConfigs.selected = null;
    }
    message.success(firstMessage || '插件已卸载');
    await Promise.all([loadPlugins(), loadUser(), loadPluginConfigs()]);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '插件卸载失败');
  } finally {
    plugins.uninstalling[row.id] = false;
  }
}
async function togglePluginOpen(row: PluginInfo) {
  if (!pluginInstalled(row)) {
    message.warning('请先安装插件');
    return;
  }
  plugins.opening[row.id] = true;
  try {
    const res = await put<ApiEnvelope<{ uuid: string; open: boolean }>>('/api/admin/plugin/open', {
      uuid: row.id,
      open: !row.open,
    });
    const data = apiData(res);
    row.open = data.open;
    if (plugins.detail?.id === row.id) {
      plugins.detail.open = data.open;
    }
    message.success(data.open ? '插件已开放到 Home 页面' : '已取消 Home 页面开放');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '修改开放状态失败');
  } finally {
    plugins.opening[row.id] = false;
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
function pluginInstalled(row: PluginInfo) {
  return row.status === 1 || row.status === 2 || row.status === 6;
}

function pluginCanOpen(row: PluginInfo) {
  return pluginInstalled(row) && row.uses_smallcat === true;
}

function pluginCanConfigure(row: PluginInfo) {
  return pluginInstalled(row) && row.config_registered === true;
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
type PluginDependencyPlan = {
  runtime: DependencyRuntime;
  plugin: string;
  pluginTitle: string;
  dependencies: NodeDependencyRow[];
  tool: DependencyToolStatus;
};

function marketPluginFileName(row: PluginInfo) {
  const address = `${row.address || ''}`.trim();
  if (!address) return '';
  const query = address.includes('?') ? address.slice(address.indexOf('?') + 1) : '';
  const params = new URLSearchParams(query);
  const candidate = params.get('path') || params.get('raw') || address;
  try {
    const pathname = candidate.startsWith('http://') || candidate.startsWith('https://')
      ? new URL(candidate).pathname
      : decodeURIComponent(candidate).split('?')[0];
    return pathname.split(/[\\/]/).filter(Boolean).pop() || '';
  } catch {
    return candidate.split(/[\\/]/).filter(Boolean).pop()?.split('?')[0] || '';
  }
}

function marketPluginDependencyRuntime(row: PluginInfo): DependencyRuntime {
  return row.type === 'python' || row.suffix?.toLowerCase() === '.py' ? 'python' : 'node';
}

async function resolvePluginDependencyPlan(row: PluginInfo): Promise<PluginDependencyPlan | null> {
  if (!Array.isArray(row.dependencies) || row.dependencies.length === 0) return null;
  const runtime = marketPluginDependencyRuntime(row);
  const listRes = await get<ApiEnvelope<{
    plugins: NodeDependencyPlugin[];
    dependencies: NodeDependencyRow[];
    tool: DependencyToolStatus;
  }>>(`/api/admin/plugin/dependencies?runtime=${encodeURIComponent(runtime)}`);
  const listData = apiData(listRes);
  const fileName = marketPluginFileName(row).toLowerCase();
  const fallbackName = fileName.replace(/\.(js|py)$/i, '');
  const plugin = (listData.plugins || []).find((item) => `${item.file || ''}`.toLowerCase() === fileName)
    || (listData.plugins || []).find((item) => item.name === fallbackName)
    || (listData.plugins || []).find((item) => item.title === row.title);
  if (!plugin) {
    throw new Error(`未识别到已安装插件文件 ${fileName || row.title}`);
  }
  const detailRes = await get<ApiEnvelope<{
    dependencies: NodeDependencyRow[];
    tool: DependencyToolStatus;
  }>>(`/api/admin/plugin/dependencies?runtime=${encodeURIComponent(runtime)}&plugin=${encodeURIComponent(plugin.name)}`);
  const detail = apiData(detailRes);
  const missing = (detail.dependencies || []).filter((item) => !item.installed);
  if (missing.length === 0) return null;
  return {
    runtime,
    plugin: plugin.name,
    pluginTitle: row.title || plugin.title || plugin.name,
    dependencies: missing,
    tool: detail.tool || listData.tool,
  };
}

async function installMarketPluginDependencies(plan: PluginDependencyPlan) {
  const messageKey = `plugin-dependencies-${plan.runtime}-${plan.plugin}`;
  message.loading({ key: messageKey, content: `正在安装 ${plan.dependencies.length} 个依赖…`, duration: 0 });
  const failed: string[] = [];
  for (const dependency of plan.dependencies) {
    try {
      await post('/api/admin/plugin/dependency', {
        runtime: plan.runtime,
        plugin: plan.plugin,
        package: dependency.name,
        dev: dependency.dev,
      });
    } catch (error) {
      failed.push(`${dependency.name}：${error instanceof Error ? error.message : '安装失败'}`);
    }
  }
  if (failed.length > 0) {
    message.error({ key: messageKey, content: `依赖安装失败：${failed.join('；')}`, duration: 6 });
    throw new Error(failed.join('；'));
  }
  message.success({ key: messageKey, content: `${plan.pluginTitle} 的依赖已全部安装`, duration: 3 });
  await Promise.all([loadPluginConfigs(), loadUser(false), loadPlugins()]);
}

async function offerPluginDependencyInstall(row: PluginInfo) {
  const plan = await resolvePluginDependencyPlan(row);
  if (!plan) return;
  const names = plan.dependencies.map((item) => item.name).join('、');
  const toolWarning = plan.tool?.available === false
    ? `安装工具当前不可用：${plan.tool.message || (plan.runtime === 'python' ? 'pipx/Python 未就绪' : 'pnpm 未就绪')}`
    : '';
  Modal.confirm({
    title: `${plan.pluginTitle} 需要安装依赖`,
    content: h('div', { class: 'plugin-dependency-confirm' }, [
      h('p', `检测到 ${plan.dependencies.length} 个未安装依赖：`),
      h('p', { class: 'mono' }, names),
      toolWarning ? h('p', { style: 'color: #d46b08' }, toolWarning) : null,
      h('p', '是否现在自动安装？'),
    ]),
    okText: '自动安装',
    cancelText: '暂不安装',
    centered: true,
    onOk: () => installMarketPluginDependencies(plan),
  });
}

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
  saving: false,
  modalOpen: false,
  opening: '',
});
const schemaFields = computed(() => {
  const props = pluginConfigs.selected?.schema?.properties || {};
  return Object.entries(props).map(([key, prop]) => ({ key, prop: prop as any }));
});
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
  pluginConfigs.saving = true;
  try {
    await putPluginConfig(pluginConfigs.selected.uuid, value);
    message.success('插件配置已保存');
    pluginConfigs.modalOpen = false;
    await loadPluginConfigs();
  } catch (error) {
    message.error(error instanceof Error ? error.message : '插件配置保存失败');
  } finally {
    pluginConfigs.saving = false;
  }
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
  qqguild_mode: 'webhook' | 'websocket';
  qqguild_app_id: string;
  qqguild_app_secret: string;
  qqguild_sandbox: boolean;
  qqguild_debug: boolean;
  pagermaid_enable: boolean;
  pagermaid_token: string;
  pagermaid_debug: boolean;
  web_chat_public: boolean;
};

type BotSettingsPlatform = 'clawbot' | 'qq' | 'telegram' | 'dingtalk' | 'qqguild' | 'web' | 'pagermaid';

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
    qqguild_mode: 'webhook',
    qqguild_app_id: '',
    qqguild_app_secret: '',
    qqguild_sandbox: false,
    qqguild_debug: false,
    pagermaid_enable: true,
    pagermaid_token: '',
    pagermaid_debug: false,
    web_chat_public: false,
  } as BotSettingsForm,
});
const botSettingsModal = reactive({
  open: false,
  platform: 'clawbot' as BotSettingsPlatform,
  label: '微信 ClawBot',
  snapshot: null as BotSettingsForm | null,
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
async function openMarketPluginConfig(row: PluginInfo) {
  if (!pluginInstalled(row)) {
    message.info('请先安装插件后再配置');
    return;
  }
  pluginConfigs.opening = row.id;
  try {
    await loadPluginConfigs();
    const config = pluginConfigs.rows.find((item) => item.uuid === row.id);
    if (!config) {
      message.info('该插件没有可配置项');
      return;
    }
    openPluginConfig(config);
    pluginConfigs.modalOpen = true;
  } catch (error) {
    message.error(error instanceof Error ? error.message : '插件配置加载失败');
  } finally {
    pluginConfigs.opening = '';
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
  'qqguild.mode',
  'qqguild.app_id',
  'qqguild.app_secret',
  'qqguild.sandbox',
  'qqguild.debug',
  'pagermaid.enable',
  'pagermaid.token',
  'pagermaid.debug',
  'sillyGirl.web_chat_public',
];
const botStatusRows = computed(() => overviewAdapters.value);
const oneBotReceiveURL = computed(() => {
  const wsProtocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const host = window.location.port === '5173' ? `${window.location.hostname}:8080` : window.location.host;
  return `${wsProtocol}://${host}/qq/receive`;
});
const qqGuildWebhookURL = computed(() => {
  const host = window.location.port === '5173' ? `${window.location.hostname}:8080` : window.location.host;
  return `${window.location.protocol}//${host}/qqguild/webhook`;
});
const webChatEndpointURL = computed(() => {
  const host = window.location.port === '5173' ? `${window.location.hostname}:8080` : window.location.host;
  return `${window.location.protocol}//${host}/api/web_chat`;
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
      qqguild_mode: data['qqguild.mode'] === 'websocket' ? 'websocket' : 'webhook',
      qqguild_app_id: data['qqguild.app_id'] || '',
      qqguild_app_secret: data['qqguild.app_secret'] || '',
      qqguild_sandbox: boolSetting(data['qqguild.sandbox']),
      qqguild_debug: boolSetting(data['qqguild.debug']),
      pagermaid_enable: boolSetting(data['pagermaid.enable'], true),
      pagermaid_token: data['pagermaid.token'] || '',
      pagermaid_debug: boolSetting(data['pagermaid.debug']),
      web_chat_public: boolSetting(data['sillyGirl.web_chat_public']),
    });
  } finally {
    botSettings.loading = false;
  }
}

async function refreshBots() {
  await Promise.all([loadUser(false), loadBots()]);
}

function openBotSettings(row: { platform: string; label: string }) {
  botSettingsModal.platform = row.platform as BotSettingsPlatform;
  botSettingsModal.label = row.label;
  botSettingsModal.snapshot = { ...botSettings.form };
  botSettingsModal.open = true;
}

function cancelCurrentBotSettings() {
  if (botSettingsModal.snapshot) {
    Object.assign(botSettings.form, botSettingsModal.snapshot);
  }
  botSettingsModal.snapshot = null;
  botSettingsModal.open = false;
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
      'qqguild.mode': v.qqguild_mode === 'websocket' ? 'websocket' : 'webhook',
      'qqguild.app_id': v.qqguild_app_id || '',
      'qqguild.app_secret': v.qqguild_app_secret || '',
      'qqguild.sandbox': !!v.qqguild_sandbox,
      'qqguild.debug': !!v.qqguild_debug,
      'pagermaid.enable': !!v.pagermaid_enable,
      'pagermaid.token': v.pagermaid_token || '',
      'pagermaid.debug': !!v.pagermaid_debug,
      'sillyGirl.web_chat_public': !!v.web_chat_public,
    });
    message.success('BOT 配置已保存');
    await refreshBots();
  } finally {
    botSettings.saving = false;
  }
}

async function saveCurrentBotSettings() {
  await saveBots();
  botSettingsModal.snapshot = null;
  botSettingsModal.open = false;
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
    loadPlugins(1, 12, true);
    loadPluginConfigs();
  }
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
          <div class="brand"><span class="brand-mark" role="img" aria-label="傻妞 Logo"></span><span>SillyGirl</span></div>
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
                </div>
              </div>
              <Spin :spinning="botSettings.loading">
                <div class="bot-card-grid">
                  <article
                    v-for="record in botStatusRows"
                    :key="record.platform"
                    class="bot-card"
                    :class="{ 'is-online': record.online, 'is-disabled': !botEnabled(record) }"
                  >
                    <header class="bot-card-header">
                      <span class="bot-card-avatar"><Bot :size="22" /></span>
                      <div class="bot-card-heading">
                        <strong>{{ record.label }}</strong>
                        <span>{{ record.platform }}</span>
                      </div>
                      <div class="bot-card-actions">
                        <template v-if="record.manageable !== false">
                          <Button
                            v-if="botEnabled(record)"
                            class="bot-card-toggle bot-card-pause"
                            type="text"
                            shape="circle"
                            :title="`暂停 ${record.label}`"
                            :aria-label="`暂停 ${record.label}`"
                            @click="setBotEnabled(record, false)"
                          >
                            <template #icon><Pause :size="18" /></template>
                          </Button>
                          <Button
                            v-else
                            class="bot-card-toggle bot-card-play"
                            type="text"
                            shape="circle"
                            :title="`开启 ${record.label}`"
                            :aria-label="`开启 ${record.label}`"
                            @click="setBotEnabled(record, true)"
                          >
                            <template #icon><Play :size="18" /></template>
                          </Button>
                        </template>
                        <Button
                          class="bot-card-settings"
                          type="text"
                          shape="circle"
                          :title="`${record.label}设置`"
                          :aria-label="`${record.label}设置`"
                          @click="openBotSettings(record)"
                        >
                          <template #icon><Settings :size="18" /></template>
                        </Button>
                      </div>
                    </header>

                    <div class="bot-card-status-list">
                      <div class="bot-card-status">
                        <CircleCheck v-if="botEnabled(record)" class="bot-status-enabled" :size="20" aria-hidden="true" />
                        <CircleX v-else class="bot-status-disabled" :size="20" aria-hidden="true" />
                        <span>启用状态</span>
                        <strong :class="botEnabled(record) ? 'bot-status-enabled' : 'bot-status-disabled'">
                          {{ botEnabled(record) ? '已启用' : '未启用' }}
                        </strong>
                      </div>
                      <div class="bot-card-status">
                        <Antenna :class="record.online ? 'bot-status-online' : 'bot-status-offline'" :size="20" aria-hidden="true" />
                        <span>连接状态</span>
                        <strong :class="record.online ? 'bot-status-online' : 'bot-status-offline'">
                          {{ record.online ? '已连接' : '未连接' }}
                        </strong>
                      </div>
                    </div>

                    <div class="bot-card-meta">
                      <div><span>实例数</span><strong>{{ record.count || 0 }}</strong></div>
                      <div>
                        <span>Bot ID</span>
                        <Typography.Text class="mono bot-card-bot-ids">{{ record.bots_id?.length ? record.bots_id.join(', ') : '-' }}</Typography.Text>
                      </div>
                      <div v-if="record.manageable === false"><span>类型</span><Tag color="blue">内置 BOT</Tag></div>
                    </div>
                  </article>
                </div>
              </Spin>
            </section>

            <Modal
              v-model:open="botSettingsModal.open"
              :title="`${botSettingsModal.label} 设置`"
              width="680px"
              ok-text="保存"
              cancel-text="取消"
              :confirm-loading="botSettings.saving"
              @cancel="cancelCurrentBotSettings"
              @ok="saveCurrentBotSettings"
            >
              <Spin :spinning="botSettings.loading">
                <Form v-if="botSettingsModal.platform === 'clawbot'" layout="vertical" class="bot-settings-modal-form">
                  <Form.Item label="启用 ClawBot" html-for="bot-clawbot-enable">
                    <Switch id="bot-clawbot-enable" v-model:checked="botSettings.form.clawbot_enable" />
                  </Form.Item>
                  <Form.Item label="Token" html-for="bot-clawbot-token" extra="ClawBot / OpenClaw 微信通道的 iLink bot token。保存后适配器会自动重启。">
                    <Space.Compact style="width: 100%">
                      <Input.Password id="bot-clawbot-token" v-model:value="botSettings.form.clawbot_token" name="clawbot-token" placeholder="请输入 ClawBot Token" />
                      <Button :loading="clawbotLogin.starting" @click="startClawbotLogin">
                        <template #icon><QrCode :size="16" /></template>扫码获取
                      </Button>
                    </Space.Compact>
                  </Form.Item>
                  <Form.Item label="API 地址" html-for="bot-clawbot-api" extra="默认使用腾讯 iLink API；如果你有兼容反代地址可以填写在这里。">
                    <Input id="bot-clawbot-api" v-model:value="botSettings.form.clawbot_api_base" name="clawbot-api" placeholder="https://ilinkai.weixin.qq.com" />
                  </Form.Item>
                  <Form.Item label="ClawBot 调试日志" html-for="bot-clawbot-debug">
                    <Switch id="bot-clawbot-debug" v-model:checked="botSettings.form.clawbot_debug" />
                  </Form.Item>
                </Form>

                <Form v-else-if="botSettingsModal.platform === 'qq'" layout="vertical" class="bot-settings-modal-form">
                  <Form.Item label="启用 QQ" html-for="bot-qq-enable">
                    <Switch id="bot-qq-enable" v-model:checked="botSettings.form.qq_enable" />
                  </Form.Item>
                  <Form.Item label="反向 WebSocket 地址" html-for="bot-qq-receive">
                    <Input id="bot-qq-receive" :value="oneBotReceiveURL" readonly>
                      <template #suffix><Typography.Text class="muted">NapCat 填这个 URL</Typography.Text></template>
                    </Input>
                  </Form.Item>
                  <Form.Item label="连接密钥" html-for="bot-qq-token" extra="需要和 NapCat / OneBot 客户端配置里的 accessToken 保持一致；公网部署建议必须填写。">
                    <Input.Password id="bot-qq-token" v-model:value="botSettings.form.qq_token" name="qq-token" placeholder="请输入 QQ 连接密钥" />
                  </Form.Item>
                  <Form.Item label="QQ 调试日志" html-for="bot-qq-debug">
                    <Switch id="bot-qq-debug" v-model:checked="botSettings.form.qq_debug" />
                  </Form.Item>
                </Form>

                <Form v-else-if="botSettingsModal.platform === 'telegram'" layout="vertical" class="bot-settings-modal-form">
                  <Form.Item label="启用 Telegram" html-for="bot-telegram-enable">
                    <Switch id="bot-telegram-enable" v-model:checked="botSettings.form.telegram_enable" />
                  </Form.Item>
                  <Form.Item label="Token" html-for="bot-telegram-token" extra="BotFather 提供的 Bot Token，保存后 Telegram 适配器会自动重启。">
                    <Input.Password id="bot-telegram-token" v-model:value="botSettings.form.telegram_token" name="telegram-token" placeholder="123456:ABC-DEF..." />
                  </Form.Item>
                  <Form.Item label="代理 API" html-for="bot-telegram-api" extra="默认使用 https://api.telegram.org；网络不通时填写兼容反代地址。">
                    <Input id="bot-telegram-api" v-model:value="botSettings.form.telegram_api_base" name="telegram-api" placeholder="https://api.telegram.org" />
                  </Form.Item>
                  <Form.Item label="Telegram 调试日志" html-for="bot-telegram-debug">
                    <Switch id="bot-telegram-debug" v-model:checked="botSettings.form.telegram_debug" />
                  </Form.Item>
                </Form>

                <Form v-else-if="botSettingsModal.platform === 'dingtalk'" layout="vertical" class="bot-settings-modal-form">
                  <Form.Item label="启用钉钉" html-for="bot-dingtalk-enable">
                    <Switch id="bot-dingtalk-enable" v-model:checked="botSettings.form.dingtalk_enable" />
                  </Form.Item>
                  <Form.Item label="Client ID" html-for="bot-dingtalk-client-id" extra="钉钉开放平台应用的 Client ID（原 AppKey）。适配器使用 Stream 模式，不需要公网回调地址。">
                    <Input id="bot-dingtalk-client-id" v-model:value="botSettings.form.dingtalk_client_id" name="dingtalk-client-id" placeholder="dingxxxxxxxx" />
                  </Form.Item>
                  <Form.Item label="Client Secret" html-for="bot-dingtalk-secret">
                    <Input.Password id="bot-dingtalk-secret" v-model:value="botSettings.form.dingtalk_client_secret" name="dingtalk-client-secret" placeholder="请输入 Client Secret" />
                  </Form.Item>
                  <Form.Item label="钉钉调试日志" html-for="bot-dingtalk-debug">
                    <Switch id="bot-dingtalk-debug" v-model:checked="botSettings.form.dingtalk_debug" />
                  </Form.Item>
                </Form>

                <Form v-else-if="botSettingsModal.platform === 'qqguild'" layout="vertical" class="bot-settings-modal-form">
                  <Form.Item label="启用 QQ 官方频道" html-for="bot-qqguild-enable">
                    <Switch id="bot-qqguild-enable" v-model:checked="botSettings.form.qqguild_enable" />
                  </Form.Item>
                  <fieldset class="bot-mode-fieldset">
                    <legend id="bot-qqguild-mode-label">接入模式</legend>
                    <Segmented
                      v-model:value="botSettings.form.qqguild_mode"
                      block
                      role="radiogroup"
                      aria-labelledby="bot-qqguild-mode-label"
                      :options="[
                        { label: 'Webhook', value: 'webhook' },
                        { label: 'WebSocket', value: 'websocket' },
                      ]"
                    />
                  </fieldset>
                  <Form.Item
                    v-if="botSettings.form.qqguild_mode === 'webhook'"
                    label="Webhook 回调地址"
                    html-for="bot-qqguild-webhook"
                    extra="把该地址填入 QQ 开放平台机器人事件回调；HTTPS 由反向代理提供。"
                  >
                    <Input id="bot-qqguild-webhook" :value="qqGuildWebhookURL" readonly />
                  </Form.Item>
                  <Alert
                    v-else
                    type="info"
                    show-icon
                    message="WebSocket 主动连接 QQ Gateway，不需要公网回调地址。"
                    style="margin-bottom: 18px"
                  />
                  <Form.Item label="AppID" html-for="bot-qqguild-app-id">
                    <Input id="bot-qqguild-app-id" v-model:value="botSettings.form.qqguild_app_id" name="qqguild-app-id" placeholder="请输入机器人 AppID" />
                  </Form.Item>
                  <Form.Item label="AppSecret" html-for="bot-qqguild-secret">
                    <Input.Password id="bot-qqguild-secret" v-model:value="botSettings.form.qqguild_app_secret" name="qqguild-app-secret" placeholder="请输入机器人 AppSecret" />
                  </Form.Item>
                  <Form.Item label="沙箱环境" html-for="bot-qqguild-sandbox">
                    <Switch id="bot-qqguild-sandbox" v-model:checked="botSettings.form.qqguild_sandbox" />
                  </Form.Item>
                  <Form.Item label="QQ 频道调试日志" html-for="bot-qqguild-debug">
                    <Switch id="bot-qqguild-debug" v-model:checked="botSettings.form.qqguild_debug" />
                  </Form.Item>
                </Form>

                <Form v-else-if="botSettingsModal.platform === 'web'" layout="vertical" class="bot-settings-modal-form">
                  <Form.Item label="运行状态" html-for="bot-web-status" extra="Web Bot 是内置适配器，随 SillyGirl 自动启动。">
                    <Switch id="bot-web-status" :checked="true" disabled />
                  </Form.Item>
                  <Form.Item label="允许匿名聊天" html-for="bot-web-public" extra="关闭时仅已登录的后台管理员可以发送 Web Bot 消息。">
                    <Switch id="bot-web-public" v-model:checked="botSettings.form.web_chat_public" />
                  </Form.Item>
                  <Form.Item label="聊天接口" html-for="bot-web-endpoint">
                    <Input id="bot-web-endpoint" :value="webChatEndpointURL" readonly />
                  </Form.Item>
                  <Button type="primary" @click="toggleWebChat">
                    <template #icon><MessageSquare :size="16" /></template>
                    {{ webChat.open ? '关闭聊天窗口' : '打开聊天窗口' }}
                  </Button>
                </Form>

                <Form v-else layout="vertical" class="bot-settings-modal-form">
                  <Form.Item label="启用 Pagermaid" html-for="bot-pagermaid-enable">
                    <Switch id="bot-pagermaid-enable" v-model:checked="botSettings.form.pagermaid_enable" />
                  </Form.Item>
                  <Form.Item label="连接密钥" html-for="bot-pagermaid-token">
                    <Input.Password id="bot-pagermaid-token" v-model:value="botSettings.form.pagermaid_token" name="pagermaid-token" placeholder="可选，建议填写" />
                  </Form.Item>
                  <Form.Item label="Pagermaid 调试日志" html-for="bot-pagermaid-debug">
                    <Switch id="bot-pagermaid-debug" v-model:checked="botSettings.form.pagermaid_debug" />
                  </Form.Item>
                  <div class="bot-settings-readonly">
                    <Typography.Text class="muted block">桥接脚本</Typography.Text>
                    <Typography.Text class="mono block">adapters/pagermaid/sillyplus.py</Typography.Text>
                  </div>
                  <div class="bot-settings-readonly">
                    <Typography.Text class="muted block">WebSocket 地址</Typography.Text>
                    <Typography.Text class="mono block">{{ pagermaidBridgeURL }}</Typography.Text>
                  </div>
                </Form>
              </Spin>
            </Modal>

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
                  :value="storageState.bucket"
                  style="width: 220px"
                  show-search
                  allow-clear
                  placeholder="选择存储桶"
                  :loading="storageState.loadingBuckets"
                  :options="storageState.buckets"
                  @change="selectStorageBucket"
                />
                <Input
                  v-model:value="storageState.search"
                  allow-clear
                  style="width: 360px"
                  placeholder="按 Key 或 Value 查询"
                  :disabled="!selectedStorageBucket"
                  autocomplete="off"
                  @press-enter="loadStorage(1)"
                />
                <Button type="primary" @click="loadStorage(1)"><template #icon><Search :size="16" /></template>查询</Button>
                <Button @click="loadStorage(storageState.current)"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                <Button :loading="storageState.creatingBucket" @click="openCreateStorageBucket">
                  <template #icon><Plus :size="16" /></template>新建桶
                </Button>
                <Popconfirm
                  :title="`确认删除存储桶 ${selectedStorageBucket}？`"
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
              <Table
                :row-key="(row:any) => `${row.bucket}.${row.key}`"
                :loading="storageState.loading"
                :data-source="storageState.rows"
                :pagination="{ current: storageState.current, pageSize: storageState.pageSize, total: storageState.total, showSizeChanger: true }"
                @change="changeStoragePage"
              >
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
                <Space>
                  <Button type="primary" @click="openNormalUser()"><template #icon><Plus :size="16" /></template>新增账号</Button>
                  <Button @click="loadNormalUsers"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
                </Space>
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
                <Table.Column title="操作" fixed="right" :width="130">
                  <template #default="{ record }">
                    <Space size="small">
                      <Button type="text" title="编辑账号" :aria-label="`编辑账号 ${record.username}`" @click="openNormalUser(record)">
                        <Edit3 :size="16" />
                      </Button>
                      <Popconfirm
                        :title="`确认删除账号「${record.username}」？`"
                        description="账号、openid/QQ/TGID 绑定和插件授权将一并删除。"
                        ok-text="确认删除"
                        cancel-text="取消"
                        @confirm="removeNormalUser(record)"
                      >
                        <Button type="text" danger :loading="normalUsers.deleting[record.id]" title="删除账号" :aria-label="`删除账号 ${record.username}`">
                          <Trash2 :size="16" />
                        </Button>
                      </Popconfirm>
                    </Space>
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
                <Tabs v-model:active-key="msgState.active">
                  <TabPane v-for="([key, item]) in Object.entries(messageBuckets)" :key="key" :tab="item.label" />
                </Tabs>
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
                <Table.Column title="操作" :width="300">
                  <template #default="{ record }">
                    <Button type="text" @click="testSmallcatPanel(record)">检测</Button>
                    <Button type="text" :loading="smallcat.accountLoadingID === record.id" @click="showSmallcatOpenids(record)">获取 OpenID</Button>
                    <Button type="text" @click="openSmallcatPanel(record)">编辑</Button>
                    <Popconfirm title="确认删除这个 smallcat？" @confirm="removeSmallcatPanel(record)">
                      <Button type="text" danger><Trash2 :size="16" /></Button>
                    </Popconfirm>
                  </template>
                </Table.Column>
              </Table>
            </section>

            <section v-if="page === 'plugins'" class="panel">
              <Tabs v-model:active-key="plugins.tab">
                <TabPane key="all" :tab="`全部 ${plugins.meta.all ?? ''}`" />
                <TabPane key="private" :tab="`非公开 ${plugins.meta.private ?? ''}`" />
                <TabPane key="tab1" :tab="`已安装 ${plugins.meta.tab1 ?? ''}`" />
                <TabPane key="tab2" :tab="`未安装 ${plugins.meta.tab2 ?? ''}`" />
                <TabPane key="tab3" :tab="`可更新 ${plugins.meta.tab3 ?? ''}`" />
              </Tabs>
              <div class="toolbar-left" style="margin-bottom: 12px">
                <Input id="plugin-market-search" v-model:value="plugins.keyword" name="plugin-market-search" allow-clear style="width: 260px" placeholder="搜索插件或来源" @press-enter="loadPlugins()" />
                <Select
                  id="plugin-market-class"
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
                <Button @click="loadPlugins(1, 12, true)"><template #icon><RefreshCw :size="16" /></template>刷新</Button>
              </div>
              <Spin :spinning="plugins.loading">
                <div v-if="plugins.rows.length" class="plugin-market-grid">
                  <article v-for="record in plugins.rows" :key="record.id" class="plugin-market-card">
                    <button
                      type="button"
                      class="plugin-card-icon"
                      :aria-label="`查看 ${record.title || record.id} 介绍`"
                      @click="openPluginDetail(record)"
                    >
                      <img v-if="pluginIconIsImage(record)" :src="record.icon" :alt="record.title || record.id" />
                      <span v-else>{{ pluginInitial(record) }}</span>
                    </button>
                    <div class="plugin-card-main">
                      <div class="plugin-card-title-row">
                        <button type="button" class="plugin-card-title" @click="openPluginDetail(record)">
                          {{ record.title || record.id }}
                        </button>
                      </div>
                      <div class="plugin-card-meta">
                        <Tag>{{ record.latest_version || record.version || record.current_version || '-' }}</Tag>
                        <Tag color="cyan">{{ pluginClassTags(record).join(' / ') || '-' }}</Tag>
                        <Tag color="gold">{{ pluginTriggerText(record) || '-' }}</Tag>
                        <Tag color="blue">{{ record.author || '-' }}</Tag>
                      </div>
                    </div>
                    <div class="plugin-card-actions">
                      <Button
                        v-if="pluginCanOpen(record)"
                        class="plugin-card-open"
                        :class="{ 'is-open': record.open }"
                        shape="circle"
                        :loading="plugins.opening[record.id]"
                        :title="record.open ? '取消 Home 页面开放' : '开放到 Home 页面'"
                        :aria-label="`${record.open ? '取消开放' : '开放'}${record.title || record.id}`"
                        @click="togglePluginOpen(record)"
                      >
                        <template #icon><DoorOpen :size="18" /></template>
                      </Button>
                      <Button
                        v-if="pluginCanConfigure(record)"
                        class="plugin-card-settings"
                        shape="circle"
                        :loading="pluginConfigs.opening === record.id"
                        title="插件配置"
                        :aria-label="`${record.title || record.id}插件配置`"
                        @click="openMarketPluginConfig(record)"
                      >
                        <template #icon><Settings :size="18" /></template>
                      </Button>
                      <Popconfirm
                        v-if="pluginInstalled(record)"
                        :title="`确认卸载「${record.title || record.id}」？`"
                        ok-text="确认卸载"
                        cancel-text="取消"
                        @confirm="uninstallPlugin(record)"
                      >
                        <Button
                          class="plugin-card-remove"
                          shape="circle"
                          danger
                          :loading="plugins.uninstalling[record.id]"
                          :title="`卸载 ${record.title || record.id}`"
                          :aria-label="`卸载 ${record.title || record.id}`"
                        >
                          <template #icon><Trash2 :size="18" /></template>
                        </Button>
                      </Popconfirm>
                      <Button
                        v-else
                        class="plugin-card-download"
                        shape="circle"
                        :loading="plugins.installing[record.id]"
                        title="安装"
                        :aria-label="`安装 ${record.title || record.id}`"
                        @click="installPlugin(record)"
                      >
                        <template #icon><CloudDownload :size="20" /></template>
                      </Button>
                    </div>
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
          <div class="brand"><span class="brand-mark" role="img" aria-label="傻妞 Logo"></span><span>SillyGirl</span></div>
          <Menu mode="inline" :selected-keys="[page]" :items="menuItems" style="border-inline-end: 0; padding-top: 8px" @click="(e:any) => navigate(e.key)" />
        </Drawer>
      </Layout>

      <div v-if="user" class="web-chat-widget">
        <section v-if="webChat.open" class="web-chat-panel" role="dialog" aria-label="Web Bot 对话框">
          <header class="web-chat-header">
            <div class="web-chat-title">
              <span class="web-chat-avatar"><Bot :size="19" /></span>
              <div>
                <strong>Web Bot</strong>
                <span class="web-chat-status">
                  <i :class="{ online: webChat.polling }"></i>
                  {{ webChat.polling ? '在线' : '连接中' }}
                </span>
              </div>
            </div>
            <button type="button" class="web-chat-close" aria-label="关闭 Web Bot" @click="toggleWebChat">
              <X :size="18" />
            </button>
          </header>

          <div ref="webChatMessagesEl" class="web-chat-messages" aria-live="polite">
            <div
              v-for="item in webChat.messages"
              :key="item.id"
              class="web-chat-message-row"
              :class="{ own: item.own, notice: item.t === 'notice' }"
            >
              <div class="web-chat-bubble">
                <div v-if="item.c" class="web-chat-content">{{ item.c }}</div>
                <div v-if="item.m?.length" class="web-chat-images">
                  <a v-for="imageURL in item.m" :key="imageURL" :href="imageURL" target="_blank" rel="noreferrer">
                    <img :src="imageURL" alt="Web Bot 返回图片" />
                  </a>
                </div>
              </div>
            </div>
          </div>

          <div v-if="webChat.error" class="web-chat-error">{{ webChat.error }}</div>
          <footer class="web-chat-composer">
            <Input.TextArea
              id="web-chat-input"
              name="web-chat-input"
              v-model:value="webChat.input"
              :auto-size="{ minRows: 1, maxRows: 4 }"
              :maxlength="2000"
              aria-label="Web Bot 消息"
              placeholder="输入命令，Enter 发送"
              @keydown.enter.exact.prevent="sendWebChat"
            />
            <Button
              type="primary"
              shape="circle"
              aria-label="发送消息"
              :loading="webChat.sending"
              :disabled="!webChat.input.trim()"
              @click="sendWebChat"
            >
              <template #icon><Send :size="17" /></template>
            </Button>
          </footer>
        </section>

        <button
          type="button"
          class="web-chat-fab"
          :class="{ active: webChat.open }"
          :aria-expanded="webChat.open"
          :aria-label="webChat.open ? '关闭 Web Bot' : '打开 Web Bot'"
          @click="toggleWebChat"
        >
          <X v-if="webChat.open" :size="24" />
          <MessageSquare v-else :size="25" />
          <span v-if="webChat.unread" class="web-chat-unread">{{ webChat.unread > 99 ? '99+' : webChat.unread }}</span>
        </button>
      </div>

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

      <Modal
        v-model:open="normalUsers.modalOpen"
        :title="normalUsers.editing ? `编辑账号：${normalUsers.editing.username}` : '新增账号'"
        width="620px"
        ok-text="保存"
        cancel-text="取消"
        :confirm-loading="normalUsers.saving"
        @ok="saveNormalUser"
      >
        <Form layout="vertical">
          <Form.Item label="账号" html-for="normal-user-username" required extra="3-32 位字母、数字、下划线、横线或点；创建后不可修改。">
            <Input
              id="normal-user-username"
              v-model:value="normalUsers.form.username"
              name="normal-user-username"
              :disabled="!!normalUsers.editing"
              autocomplete="username"
              placeholder="请输入登录账号"
            />
          </Form.Item>
          <Form.Item
            :label="normalUsers.editing ? '新密码' : '密码'"
            html-for="normal-user-password"
            :required="!normalUsers.editing"
            :extra="normalUsers.editing ? '留空则保留原密码。' : '至少 6 位。'"
          >
            <Input.Password
              id="normal-user-password"
              v-model:value="normalUsers.form.password"
              name="normal-user-password"
              autocomplete="new-password"
              placeholder="请输入密码"
            />
          </Form.Item>
          <Form.Item label="昵称" html-for="normal-user-nickname">
            <Input id="normal-user-nickname" v-model:value="normalUsers.form.nickname" name="normal-user-nickname" maxlength="64" placeholder="留空则使用账号名" />
          </Form.Item>
          <Form.Item label="smallcat openid 列表" extra="可输入多个 openid，按回车确认；保存时自动去重。">
            <Select
              v-model:value="normalUsers.form.smallcat_openids"
              mode="tags"
              :token-separators="[',', ';', ' ']"
              :options="[]"
              placeholder="输入 openid 后按回车"
            />
          </Form.Item>
          <Form.Item label="绑定 QQ" html-for="normal-user-qq">
            <Input id="normal-user-qq" v-model:value="normalUsers.form.qq" name="normal-user-qq" inputmode="numeric" placeholder="5-12 位 QQ 号；留空解除绑定" />
          </Form.Item>
          <Form.Item label="绑定 TGID" html-for="normal-user-tgid">
            <Input id="normal-user-tgid" v-model:value="normalUsers.form.telegram" name="normal-user-tgid" placeholder="Telegram 用户 ID；留空解除绑定" />
          </Form.Item>
          <Form.Item label="禁用账号" html-for="normal-user-disabled">
            <Switch id="normal-user-disabled" v-model:checked="normalUsers.form.disabled" />
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
        <Form layout="vertical">
          <Form.Item label="标题" html-for="task-title" required help="定时任务标题不能为空"><Input id="task-title" v-model:value="tasks.form.title" name="task-title" placeholder="例如：每小时检查 IP" /></Form.Item>
          <Form.Item label="Cron 表达式" html-for="task-schedule" required help="例如：0 * * * *，也支持带秒字段的 6 段表达式"><Input id="task-schedule" v-model:value="tasks.form.schedule" name="task-schedule" placeholder="0 * * * *" /></Form.Item>
          <Form.Item label="触发命令" html-for="task-command"><Select id="task-command" v-model:value="tasks.form.command" show-search :options="tasks.scripts" placeholder="node xxx.js 或 python xxx.py" /></Form.Item>
          <Form.Item label="接收平台" html-for="task-platform"><Select id="task-platform" v-model:value="tasks.form.platform" allow-clear :options="tasks.platforms" placeholder="选择 BOT 平台" /></Form.Item>
          <Form.Item label="接收人 ID" html-for="task-recipient" help="填写该平台的用户 ID；插件调用 s.reply() 时会私聊此账号"><Input id="task-recipient" v-model:value="tasks.form.recipient" name="task-recipient" allow-clear placeholder="请输入用户 ID / OpenID" /></Form.Item>
          <Form.Item label="启用" html-for="task-enable"><Switch id="task-enable" v-model:checked="tasks.form.enable" /></Form.Item>
        </Form>
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

      <Modal
        v-model:open="plugins.detailOpen"
        :title="plugins.detail?.title || plugins.detail?.id || '插件介绍'"
        width="640px"
        :footer="null"
      >
        <div v-if="plugins.detail" class="plugin-detail">
          <div class="plugin-detail-header">
            <div class="plugin-detail-icon" aria-hidden="true">
              <img v-if="pluginIconIsImage(plugins.detail)" :src="plugins.detail.icon" alt="" />
              <span v-else>{{ pluginInitial(plugins.detail) }}</span>
            </div>
            <div class="plugin-detail-heading">
              <Typography.Title :level="4">{{ plugins.detail.title || plugins.detail.id }}</Typography.Title>
              <Space wrap size="small">
                <Tag :color="pluginStatusColor(plugins.detail)">{{ pluginStatusLabel(plugins.detail) }}</Tag>
                <Tag v-if="plugins.detail.version">{{ plugins.detail.version }}</Tag>
                <Tag v-for="klass in pluginClassTags(plugins.detail)" :key="klass" color="cyan">{{ klass }}</Tag>
              </Space>
            </div>
          </div>

          <Typography.Paragraph class="plugin-detail-description">
            {{ plugins.detail.desc || '该插件暂未填写介绍。' }}
          </Typography.Paragraph>

          <Alert
            v-if="plugins.detail.status === 1 && plugins.detail.update_content"
            type="success"
            show-icon
            :message="`更新内容：${plugins.detail.update_content}`"
          />

          <dl class="plugin-detail-meta">
            <template v-if="pluginTriggerText(plugins.detail)">
              <dt>触发口令</dt>
              <dd class="mono">{{ pluginTriggerText(plugins.detail) }}</dd>
            </template>
            <template v-if="plugins.detail.author">
              <dt>作者</dt>
              <dd>{{ plugins.detail.author }}</dd>
            </template>
            <template v-if="pluginDependencies(plugins.detail).length">
              <dt>所需依赖</dt>
              <dd>
                <Space wrap size="small">
                  <Tag
                    v-for="dependency in pluginDependencies(plugins.detail)"
                    :key="dependency"
                    color="blue"
                    class="mono"
                  >
                    {{ dependency }}
                  </Tag>
                </Space>
              </dd>
            </template>
            <template v-if="plugins.detail.status === 1">
              <dt>版本</dt>
              <dd>当前 {{ plugins.detail.current_version || '-' }} / 最新 {{ plugins.detail.latest_version || plugins.detail.version || '-' }}</dd>
            </template>
            <template v-if="plugins.detail.organization">
              <dt>来源</dt>
              <dd>{{ plugins.detail.organization }}</dd>
            </template>
          </dl>
        </div>
      </Modal>

      <Modal
        v-model:open="pluginConfigs.modalOpen"
        :title="`${pluginConfigs.selected?.plugin || pluginConfigs.selected?.title || '插件'} 配置`"
        width="720px"
        ok-text="保存配置"
        cancel-text="取消"
        :confirm-loading="pluginConfigs.saving"
        :ok-button-props="{ disabled: !pluginConfigs.selected || pluginConfigs.selected.registered === false }"
        @ok="savePluginConfig"
      >
        <Spin :spinning="pluginConfigs.loading">
          <div v-if="pluginConfigs.selected" class="config-form plugin-config-modal-form">
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
                <Form.Item :label="field.prop.title || field.key" :html-for="`plugin-config-${field.key}`" :extra="field.prop.description">
                  <Select
                    v-if="fieldType(field.prop) === 'enum'"
                    :id="`plugin-config-${field.key}`"
                    v-model:value="pluginConfigs.form[field.key]"
                    :options="fieldOptions(field.prop)"
                  />
                  <Switch
                    v-else-if="fieldType(field.prop) === 'boolean'"
                    :id="`plugin-config-${field.key}`"
                    v-model:checked="pluginConfigs.form[field.key]"
                  />
                  <InputNumber
                    v-else-if="fieldType(field.prop) === 'number' || fieldType(field.prop) === 'integer'"
                    :id="`plugin-config-${field.key}`"
                    v-model:value="pluginConfigs.form[field.key]"
                    style="width: 100%"
                    :min="field.prop.minimum"
                    :max="field.prop.maximum"
                  />
                  <Input.TextArea
                    v-else-if="fieldType(field.prop) === 'object' || fieldType(field.prop) === 'array'"
                    :id="`plugin-config-${field.key}`"
                    :name="field.key"
                    v-model:value="pluginConfigs.text[field.key]"
                    :rows="6"
                    class="mono"
                  />
                  <Input.Password
                    v-else-if="field.prop.format === 'password' || field.prop['ui:widget'] === 'password'"
                    :id="`plugin-config-${field.key}`"
                    :name="field.key"
                    v-model:value="pluginConfigs.form[field.key]"
                  />
                  <Input.TextArea
                    v-else-if="field.prop.format === 'textarea' || field.prop['ui:widget'] === 'textarea'"
                    :id="`plugin-config-${field.key}`"
                    :name="field.key"
                    v-model:value="pluginConfigs.form[field.key]"
                    :rows="4"
                  />
                  <Input v-else :id="`plugin-config-${field.key}`" :name="field.key" v-model:value="pluginConfigs.form[field.key]" />
                </Form.Item>
              </template>
            </Form>
          </div>
        </Spin>
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
			<Input.Password v-model:value="smallcat.form.api_auth" :placeholder="smallcat.form.id ? '留空保持原 AUTH 不变' : '请输入 API AUTH'" />
          </Form.Item>
          <Button @click="testSmallcatPanel()" :loading="smallcat.testing">
            <template #icon><RefreshCw :size="16" /></template>检测连接
          </Button>
        </Form>
      </Modal>

	  <Modal
		:open="smallcat.accountsOpen"
		:title="`${smallcat.accountPanelName} · OpenID 列表（${smallcat.accountOpenids.length}）`"
		width="680px"
		:footer="null"
		@cancel="smallcat.accountsOpen = false"
	  >
		<Empty v-if="!smallcat.accountOpenids.length" description="暂无账号" />
		<Space v-else direction="vertical" size="small" style="width: 100%">
		  <Typography.Text
			v-for="(openid, index) in smallcat.accountOpenids"
			:key="openid"
			code
			:copyable="true"
		  >{{ index + 1 }}. {{ openid }}</Typography.Text>
		</Space>
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
