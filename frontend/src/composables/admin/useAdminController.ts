import {
  computed,
  defineComponent,
  h,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import type { Compartment, Extension } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import message from "ant-design-vue/es/message";
import Modal from "ant-design-vue/es/modal";
import QRCode from "qrcode";
import {
  Bot,
  ClipboardList,
  Database,
  Home,
  MessageSquare,
  Package,
  Plug,
  Server,
  Settings,
  ShieldCheck,
  User,
} from "lucide-vue-next";
import {
  ApiError,
  clearAuthToken,
  get,
  getAuthToken,
  post,
  saveStorage,
  setAuthToken,
} from "../../api";
import type { AdminUserRow, CurrentUser, PluginInfo } from "../../types";
import {
  confirmDownloadedPluginDependencyInstall,
  declaredPluginDependenciesFromContent,
  pluginDependencies,
  resolveDownloadedPluginDependencyPlan,
  type DownloadedPluginDependencyPlan,
} from "./pluginInstallPrompt";
import { apiData, type ApiEnvelope } from "./adminApi";
import { useNormalUsersAdmin } from "./useNormalUsersAdmin";
import { useCarryAdmin } from "./useCarryAdmin";
import { useMastersAdmin } from "./useMastersAdmin";
import { useMessageRulesAdmin } from "./useMessageRulesAdmin";
import { useRepliesAdmin } from "./useRepliesAdmin";
import { usePanelsAdmin, type AdminPanelsResponse } from "./usePanelsAdmin";
import { useStorageAdmin } from "./useStorageAdmin";
import { useTasksAdmin } from "./useTasksAdmin";

export function useAdminController() {
  type WebChatMessage = {
    t?: string;
    c?: string;
    m?: string[];
  };

  type WebChatEntry = WebChatMessage & {
    id: number;
    own?: boolean;
  };

  type PageKey =
    | "welcome"
    | "bots"
    | "dependencies"
    | "plugins"
    | "storage"
    | "users"
    | "tasks"
    | "message-tools"
    | "containers"
    | "masters"
    | "settings";

  type ContainerKind = "qinglong" | "daidai" | "smallcat";
  type MessageToolKind = "carry" | "reply" | "messages";

  const validPages: PageKey[] = [
    "welcome",
    "bots",
    "dependencies",
    "plugins",
    "storage",
    "users",
    "tasks",
    "message-tools",
    "containers",
    "masters",
    "settings",
  ];
  const legacyContainerPages: ContainerKind[] = [
    "qinglong",
    "daidai",
    "smallcat",
  ];
  const legacyMessageToolPages: MessageToolKind[] = [
    "carry",
    "reply",
    "messages",
  ];

  const user = ref<CurrentUser | null>(null);
  const booting = ref(true);
  const page = ref<PageKey>(pageFromPath());
  const containerKind = ref<ContainerKind>(containerKindFromPath());
  const messageToolKind = ref<MessageToolKind>(messageToolKindFromPath());
  const mobileMenuOpen = ref(false);
  const loginModel = reactive({ username: "", password: "" });
  const setupRequired = ref(false);
  const setupModel = reactive({ username: "", password: "", confirm: "" });

  const webChatMessagesEl = ref<HTMLElement | null>(null);
  const webChat = reactive({
    open: false,
    input: "",
    sending: false,
    polling: false,
    error: "",
    unread: 0,
    messages: [] as WebChatEntry[],
  });
  const webChatRid = loadWebChatRid();
  let webChatMessageID = 0;
  let webChatPollGeneration = 0;
  let webChatPollController: AbortController | null = null;

  function loadWebChatRid() {
    const key = "sillygirl_web_chat_rid";
    const current = sessionStorage.getItem(key)?.trim();
    if (current) return current;
    const suffix =
      typeof crypto?.randomUUID === "function"
        ? crypto.randomUUID()
        : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    const rid = `admin-${suffix}`;
    sessionStorage.setItem(key, rid);
    return rid;
  }

  function webChatRows(res: ApiEnvelope<WebChatMessage[]>) {
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
      const content = `${row?.c || ""}`;
      const images = Array.isArray(row?.m) ? row.m.filter(Boolean) : [];
      if (!content && images.length === 0) continue;
      webChat.messages.push({
        id: ++webChatMessageID,
        t: row?.t || "chat",
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
    if (!webChat.open || !user.value || generation !== webChatPollGeneration)
      return;
    const controller = new AbortController();
    webChatPollController = controller;
    webChat.polling = true;
    try {
      const res = await get<ApiEnvelope<WebChatMessage[]>>(
        `/api/web-chat/messages?rid=${encodeURIComponent(webChatRid)}`,
        { signal: controller.signal },
      );
      if (generation !== webChatPollGeneration) return;
      webChat.error = "";
      appendWebChatMessages(webChatRows(res));
    } catch (error) {
      if (generation !== webChatPollGeneration) return;
      if (error instanceof DOMException && error.name === "AbortError") return;
      webChat.error =
        error instanceof Error ? error.message : "Web Bot 连接失败";
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
    webChat.error = "";
    webChatPollGeneration += 1;
    if (webChat.open) {
      if (webChat.messages.length === 0) {
        appendWebChatMessages([
          { t: "notice", c: "Web Bot 已连接，可以直接发送命令。" },
        ]);
      }
      void pollWebChat(webChatPollGeneration);
    }
  }

  async function sendWebChat() {
    const content = webChat.input.trim();
    if (!content || webChat.sending) return;
    webChat.input = "";
    webChat.sending = true;
    webChat.error = "";
    appendWebChatMessages([{ t: "chat", c: content }], true);
    try {
      await post<ApiEnvelope<WebChatMessage[]>>("/api/web-chat/messages", {
        rid: webChatRid,
        ctt: content,
      });
    } catch (error) {
      webChat.error = error instanceof Error ? error.message : "消息发送失败";
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
  const realScripts = computed(() =>
    scripts.value.filter(
      (item) =>
        item.path?.startsWith("/script/") && !item.name?.startsWith("+"),
    ),
  );
  function dependencyPluginFileName(item?: {
    name?: string;
    type?: string;
    file?: string;
  }) {
    if (!item) return "-";
    if (item.file) return item.file.split(/[\/]/).pop() || item.file;
    const suffix = item.type === "python" ? ".py" : ".js";
    return `${item.name || "plugin"}${suffix}`;
  }
  const overviewAdapters = computed(() => {
    const defaults = [
      { platform: "clawbot", label: "微信 ClawBot" },
      { platform: "dingtalk", label: "钉钉机器人" },
      { platform: "pagermaid", label: "Pagermaid" },
      { platform: "qq", label: "QQ" },
      { platform: "qqguild", label: "QQ 官方频道机器人" },
      { platform: "web", label: "Web Bot" },
      { platform: "telegram", label: "Telegram Bot" },
    ];
    const rows = new Map(
      (user.value?.adapters || []).map((item) => [item.platform, item]),
    );
    return defaults.map((item) => {
      const row = rows.get(item.platform);
      return {
        platform: item.platform,
        label: row?.label || item.label,
        online: !!row?.online,
        enabled: row?.enabled !== false,
        manageable: row ? row.manageable !== false : item.platform !== "web",
        bots_id: row?.bots_id || [],
        count: row?.count || 0,
      };
    });
  });
  const overviewIntegrations = computed(() => {
    const defaults = [
      { key: "qinglong", label: "青龙容器" },
      { key: "smallcat", label: "smallcat" },
      { key: "daidai", label: "呆呆容器" },
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
      local: info.local || "1.1.6",
      remote: info.remote || info.local || "1.1.6",
      source: info.source || "reserved",
      repository: info.repository || "https://github.com/smallfawn/sillyGirl",
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
    id?: string;
    running?: boolean;
    status?: "idle" | "running" | "done" | "error";
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
    status: "idle" as "idle" | "running" | "done" | "error",
    message: "",
    result: null as SystemUpdateResult | null,
    timer: 0,
    restartTimer: 0,
    jobId: "",
  });

  function applySystemUpdateSnapshot(snapshot: SystemUpdateSnapshot) {
    systemUpdate.jobId = snapshot.id || systemUpdate.jobId;
    systemUpdate.running = !!snapshot.running;
    systemUpdate.status =
      snapshot.status || (snapshot.running ? "running" : "idle");
    systemUpdate.percent = Math.max(
      0,
      Math.min(100, Number(snapshot.percent || 0)),
    );
    systemUpdate.message = snapshot.error || snapshot.message || "";
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
    systemUpdate.status = "running";
    systemUpdate.percent = 6;
    systemUpdate.result = null;
    systemUpdate.message = "正在连接 GitHub Release";
    window.clearInterval(systemUpdate.timer);
    try {
      const res = await post<ApiEnvelope<SystemUpdateSnapshot>>(
        "/api/admin/system-update-jobs",
        {},
      );
      applySystemUpdateSnapshot(apiData(res));
      systemUpdate.timer = window.setInterval(() => {
        pollSystemUpdateStatus().catch((error) => {
          systemUpdate.status = "error";
          systemUpdate.message =
            error instanceof Error ? error.message : "读取更新状态失败";
          systemUpdate.running = false;
          window.clearInterval(systemUpdate.timer);
        });
      }, 1000);
      await pollSystemUpdateStatus();
    } catch (error) {
      systemUpdate.status = "error";
      systemUpdate.running = false;
      systemUpdate.message =
        error instanceof Error ? error.message : "更新失败";
      message.error(systemUpdate.message);
      window.clearInterval(systemUpdate.timer);
    }
  }

  async function pollSystemUpdateStatus() {
    if (!systemUpdate.jobId) return;
    const res = await get<ApiEnvelope<SystemUpdateSnapshot>>(
      `/api/admin/system-update-jobs/${encodeURIComponent(systemUpdate.jobId)}`,
    );
    const snapshot = apiData(res);
    applySystemUpdateSnapshot(snapshot);
    if (snapshot.status === "done") {
      await loadUser(false);
    }
  }

  async function restartAfterUpdate() {
    systemUpdate.restarting = true;
    systemUpdate.restartChecking = false;
    try {
      await post("/api/admin/system-restart-jobs", {});
      systemUpdate.restarting = false;
      systemUpdate.restartChecking = true;
      systemUpdate.status = "running";
      systemUpdate.percent = 0;
      systemUpdate.message = "重启已触发，正在等待服务恢复";
      await waitForRestartReady();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "重启失败");
      systemUpdate.restartChecking = false;
      systemUpdate.status = "error";
      systemUpdate.message =
        error instanceof Error ? error.message : "重启失败";
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
          const res = await fetch(`/api/health?t=${Date.now()}`, {
            cache: "no-store",
          });
          if (!res.ok) return;
          const body = await res.json().catch(() => null);
          if (!body || body.status === false) return;
          window.clearInterval(systemUpdate.restartTimer);
          systemUpdate.restartChecking = false;
          systemUpdate.status = "done";
          systemUpdate.percent = 100;
          systemUpdate.message = "重启成功，服务已恢复";
          message.success("重启成功");
          await loadUser(false).catch(() => undefined);
          resolve();
        } catch (_) {
          if (elapsed >= 180) {
            window.clearInterval(systemUpdate.restartTimer);
            systemUpdate.restartChecking = false;
            systemUpdate.status = "error";
            systemUpdate.message = "重启等待超时，请手动刷新页面确认服务状态";
            resolve();
          }
        }
      }, 1000);
    });
  }

  const menuItems = [
    { key: "welcome", label: "概览", icon: () => h(Home, { size: 16 }) },
    { key: "bots", label: "BOT", icon: () => h(Bot, { size: 16 }) },
    {
      key: "dependencies",
      label: "依赖管理",
      icon: () => h(Package, { size: 16 }),
    },
    { key: "plugins", label: "插件市场", icon: () => h(Plug, { size: 16 }) },
    { key: "storage", label: "存储", icon: () => h(Database, { size: 16 }) },
    { key: "users", label: "用户管理", icon: () => h(User, { size: 16 }) },
    {
      key: "message-tools",
      label: "转发/回复/监听",
      icon: () => h(MessageSquare, { size: 16 }),
    },
    {
      key: "tasks",
      label: "定时任务",
      icon: () => h(ClipboardList, { size: 16 }),
    },
    {
      key: "containers",
      label: "容器管理",
      icon: () => h(Server, { size: 16 }),
    },
    {
      key: "masters",
      label: "管理员",
      icon: () => h(ShieldCheck, { size: 16 }),
    },
    {
      key: "settings",
      label: "基础设置",
      icon: () => h(Settings, { size: 16 }),
    },
  ];

  function pageFromPath(): PageKey {
    const path = window.location.pathname.replace(/^\/admin\/?/, "/");
    if (path.startsWith("/script/") || path === "/scripts") {
      window.history.replaceState({}, "", "/admin/plugins");
      return "plugins";
    }
    const key = path.split("/").filter(Boolean)[0] || "welcome";
    if (legacyContainerPages.includes(key as ContainerKind))
      return "containers";
    if (legacyMessageToolPages.includes(key as MessageToolKind))
      return "message-tools";
    if (key === "plugin-configs") {
      window.history.replaceState({}, "", "/admin/plugins");
      return "plugins";
    }
    return validPages.includes(key as PageKey) ? (key as PageKey) : "welcome";
  }

  function containerKindFromPath(): ContainerKind {
    const path = window.location.pathname.replace(/^\/admin\/?/, "/");
    const parts = path.split("/").filter(Boolean);
    const key = parts[0] === "containers" ? parts[1] : parts[0];
    return legacyContainerPages.includes(key as ContainerKind)
      ? (key as ContainerKind)
      : "qinglong";
  }

  function messageToolKindFromPath(): MessageToolKind {
    const path = window.location.pathname.replace(/^\/admin\/?/, "/");
    const parts = path.split("/").filter(Boolean);
    const key = parts[0] === "message-tools" ? parts[1] : parts[0];
    return legacyMessageToolPages.includes(key as MessageToolKind)
      ? (key as MessageToolKind)
      : "carry";
  }

  function navigate(next: PageKey, path?: string) {
    const url =
      path ||
      (next === "welcome"
        ? "/admin/"
        : next === "containers"
          ? `/admin/containers/${containerKind.value}`
          : next === "message-tools"
            ? `/admin/message-tools/${messageToolKind.value}`
            : `/admin/${next}`);
    window.history.pushState({}, "", url);
    page.value = next;
    mobileMenuOpen.value = false;
  }

  async function loadSetupStatus() {
    const res =
      await get<ApiEnvelope<{ initialized: boolean }>>("/api/admin/setup");
    const data = apiData(res);
    setupRequired.value = !data?.initialized;
    return !!data?.initialized;
  }

  async function loadUser(setBooting = true, reloadSetupOnUnauthorized = true) {
    if (setBooting) booting.value = true;
    try {
      const res = await get<ApiEnvelope<CurrentUser>>(
        "/api/admin/sessions/current",
      );
      user.value = apiData(res) || {};
      setupRequired.value = false;
    } catch (error) {
      if (error instanceof ApiError && error.status !== 401)
        message.error(error.message);
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
      const res = await post<ApiEnvelope<AuthResponse>>(
        "/api/admin/sessions",
        loginModel,
      );
      const auth = apiData(res);
      if (auth.status === "setup_required") {
        setupRequired.value = true;
        message.error("请先设置管理员账号和密码");
        return;
      }
      if (auth.status !== "ok" || !auth.token) {
        message.error("账号或密码不正确");
        return;
      }
      setAuthToken(auth.token, auth.expiresIn);
      message.success("已登录");
      await loadUser();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "登录失败");
    }
  }

  async function setupAdmin() {
    if (!setupModel.username.trim()) {
      message.error("账号不能为空");
      return;
    }
    if (!setupModel.password.trim()) {
      message.error("密码不能为空");
      return;
    }
    if (setupModel.password !== setupModel.confirm) {
      message.error("两次输入的密码不一致");
      return;
    }
    try {
      const res = await post<ApiEnvelope<AuthResponse>>("/api/admin/setup", {
        username: setupModel.username.trim(),
        password: setupModel.password,
      });
      const auth = apiData(res);
      if (auth.status !== "ok" || !auth.token) {
        message.error("账号创建失败");
        return;
      }
      setAuthToken(auth.token, auth.expiresIn);
      message.success("账号已创建");
      setupRequired.value = false;
      loginModel.username = setupModel.username.trim();
      loginModel.password = "";
      setupModel.password = "";
      setupModel.confirm = "";
      await loadUser();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "创建账号失败");
    }
  }

  async function logout() {
    stopWebChat();
    await post("/api/admin/sessions/current/deletions").catch(() => undefined);
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
    window.addEventListener("popstate", () => {
      page.value = pageFromPath();
      containerKind.value = containerKindFromPath();
      messageToolKind.value = messageToolKindFromPath();
    });
    window.addEventListener(
      "sillygirl:admin-auth-expired",
      handleAdminAuthExpired,
    );
  });

  onBeforeUnmount(() => {
    stopWebChat();
    clearClawbotLoginPoll();
    window.clearInterval(systemUpdate.timer);
    window.clearInterval(systemUpdate.restartTimer);
    window.removeEventListener(
      "sillygirl:admin-auth-expired",
      handleAdminAuthExpired,
    );
  });

  const {
    storageState,
    selectedStorageBucket,
    canRemoveStorageBucket,
    loadStorage,
    changeStoragePage,
    saveStorageRow,
    selectStorageBucket,
    openCreateStorageBucket,
    createStorageBucket,
    createStorageEntry,
    removeStorageBucket,
  } = useStorageAdmin();

  const { replies, loadReplies, openReply, saveReply, removeReply } =
    useRepliesAdmin();

  const { masters, loadMasters, saveMaster, removeMaster } = useMastersAdmin();

  const {
    normalUsers,
    normalUserPluginAuthorizations,
    loadNormalUsers,
    openNormalUserPluginAuthorizations,
    openNormalUser,
    pluginAuthorizations,
    saveNormalUser,
    saveNormalUserPluginAuthorization,
    removeNormalUser,
  } = useNormalUsersAdmin(smallcatOpenids);

  const {
    isPluginCronTask,
    tasks,
    loadTasks,
    openTask,
    saveTask,
    removeTask,
    runTask,
    toggleTaskEnabled,
  } = useTasksAdmin();

  const {
    carry,
    loadCarry,
    changeCarryPlatform,
    openCarry,
    saveCarry,
    removeCarry,
  } = useCarryAdmin();

  const {
    qinglong,
    openQinglongPanel,
    testQinglongPanel,
    saveQinglongPanel,
    removeQinglongPanel,
    smallcat,
    smallcatQuotaText,
    openSmallcatPanel,
    testSmallcatPanel,
    saveSmallcatPanel,
    removeSmallcatPanel,
    showSmallcatOpenids,
    daidai,
    applyAdminPanels,
    openDaidaiPanel,
    testDaidaiPanel,
    saveDaidaiPanel,
    removeDaidaiPanel,
    containerOptions,
    containerHelpText,
    containerAddLabel,
    loadActiveContainerPanels,
    refreshActiveContainerPanels,
    openActiveContainerPanel,
  } = usePanelsAdmin(containerKind);

  const plugins = reactive({
    rows: [] as PluginInfo[],
    total: 0,
    current: 1,
    pageSize: 16,
    tab: "all",
    keyword: "",
    klass: "全部",
    meta: {} as any,
    loading: false,
    sources: [] as string[],
    sourceAddress: "",
    sourceSaving: false,
    sourceModal: false,
    sourceRemoving: {} as Record<string, boolean>,
    installing: {} as Record<string, boolean>,
    uninstalling: {} as Record<string, boolean>,
    dependencyChecking: {} as Record<string, boolean>,
    toggling: {} as Record<string, boolean>,
    uninstallModal: {
      open: false,
      row: null as PluginInfo | null,
      deleteConfig: false,
      dependents: [] as Array<{ id: string; title: string; type: string }>,
    },
    requestId: 0,
    detailOpen: false,
    detail: null as PluginInfo | null,
  });

  const pluginSearchDebounceMs = 350;
  let pluginSearchTimer = 0;

  function cancelPluginSearch() {
    if (!pluginSearchTimer) return;
    window.clearTimeout(pluginSearchTimer);
    pluginSearchTimer = 0;
  }

  function searchPluginsNow() {
    cancelPluginSearch();
    void loadPlugins(1, plugins.pageSize);
  }

  function schedulePluginSearch() {
    cancelPluginSearch();
    if (page.value !== "plugins") return;
    // 输入变化后立即让进行中的旧请求失效，避免防抖等待期间回填旧结果。
    plugins.requestId += 1;
    pluginSearchTimer = window.setTimeout(() => {
      pluginSearchTimer = 0;
      void loadPlugins(1, plugins.pageSize);
    }, pluginSearchDebounceMs);
  }

  const pluginEditor = reactive({
    open: false,
    loading: false,
    saving: false,
    deleting: false,
    isNew: false,
    id: "",
    name: "",
    title: "",
    type: "node" as DependencyRuntime,
    theme: "dark" as "dark" | "light",
    installed: false,
    content: "",
    row: null as PluginInfo | null,
  });
  const pluginEditorHost = ref<HTMLElement | null>(null);
  type PluginEditorRuntime = {
    Compartment: typeof import("@codemirror/state").Compartment;
    EditorState: typeof import("@codemirror/state").EditorState;
    EditorView: typeof import("@codemirror/view").EditorView;
    javascript: typeof import("@codemirror/lang-javascript").javascript;
    python: typeof import("@codemirror/lang-python").python;
    oneDark: typeof import("@codemirror/theme-one-dark").oneDark;
    basicSetup: typeof import("codemirror").basicSetup;
  };
  let pluginEditorRuntime: PluginEditorRuntime | null = null;
  let pluginEditorRuntimePromise: Promise<PluginEditorRuntime> | null = null;
  let pluginEditorEditable: Compartment | null = null;
  let pluginEditorLanguage: Compartment | null = null;
  let pluginEditorTheme: Compartment | null = null;
  let pluginEditorView: EditorView | null = null;

  async function loadPluginEditorRuntime() {
    if (pluginEditorRuntime) return pluginEditorRuntime;
    if (!pluginEditorRuntimePromise) {
      pluginEditorRuntimePromise = Promise.all([
        import("@codemirror/state"),
        import("@codemirror/view"),
        import("@codemirror/lang-javascript"),
        import("@codemirror/lang-python"),
        import("@codemirror/theme-one-dark"),
        import("codemirror"),
      ]).then(
        ([
          state,
          view,
          javascriptLanguage,
          pythonLanguage,
          theme,
          codemirror,
        ]) => ({
          Compartment: state.Compartment,
          EditorState: state.EditorState,
          EditorView: view.EditorView,
          javascript: javascriptLanguage.javascript,
          python: pythonLanguage.python,
          oneDark: theme.oneDark,
          basicSetup: codemirror.basicSetup,
        }),
      );
    }
    pluginEditorRuntime = await pluginEditorRuntimePromise;
    if (!pluginEditorEditable) {
      pluginEditorEditable = new pluginEditorRuntime.Compartment();
      pluginEditorLanguage = new pluginEditorRuntime.Compartment();
      pluginEditorTheme = new pluginEditorRuntime.Compartment();
    }
    return pluginEditorRuntime;
  }

  const pluginEditorStarter = `// [title: 本地插件]
  // [name: localPlugin]
  // [desc: 本地手动新增插件]
  // [author: admin]
  // [version: v1.0.0]
  // [status: true]
  // [rule: ^测试插件$]
  // [public: false]
  // [class: 工具]
  // [depe: []]

  const { sender: s } = require('sillygirl');

  async function main() {
    await s.reply('pong');
  }

  main().catch((error) => s.reply(error.message || String(error)));
  `;
  const pluginClassOptions = computed(() => {
    const classes = (plugins.meta.class || {}) as Record<string, number>;
    const names = Object.keys(classes).filter(Boolean);
    if (!names.includes("全部")) {
      names.unshift("全部");
    }
    return names
      .sort((a, b) => {
        if (a === "全部") return -1;
        if (b === "全部") return 1;
        return a.localeCompare(b, "zh-Hans-CN");
      })
      .map((value) => ({
        value,
        label:
          classes[value] === undefined ? value : `${value} (${classes[value]})`,
      }));
  });
  function filterPluginClassOption(
    input: string,
    option?: { label?: string; value?: string },
  ) {
    const keyword = String(input || "").toLowerCase();
    return String(option?.label || option?.value || "")
      .toLowerCase()
      .includes(keyword);
  }
  function pluginClassTags(row: PluginInfo) {
    return String(row.class || "")
      .split(/[,，\s]+/)
      .map((item) => item.trim())
      .filter(Boolean);
  }
  function pluginTriggerText(row: PluginInfo) {
    if (row.module) return "共享模块";
    const rule = String(row.rule || "").trim();
    if (!rule) return "";
    return (
      rule
        .replace(/^\^\\s\*\(?/, "")
        .replace(/\)?\\s\*\$$/, "")
        .replace(/^\^/, "")
        .replace(/\$$/, "")
        .replace(/\(\?:/g, "(")
        .replace(/^\((.*)\)$/, "$1")
        .replace(/\[Jj\]/g, "J")
        .replace(/\[Dd\]/g, "D")
        .replace(/\|/g, " / ")
        .replace(/\\s\*/g, " ")
        .replace(/\\s\+/g, " ")
        .replace(/\s+/g, " ")
        .trim() || rule
    );
  }
  function pluginIconIsImage(row: PluginInfo) {
    const icon = String(row.icon || "").trim();
    return (
      /^https?:\/\//i.test(icon) ||
      icon.startsWith("/") ||
      icon.startsWith("data:image/")
    );
  }
  function pluginInitial(row: PluginInfo) {
    const text = String(row.title || row.id || "P").trim();
    return (text ? text.slice(0, 1) : "P").toUpperCase();
  }
  function pluginHasSchedule(row: PluginInfo) {
    return Boolean(
      row.cron &&
      Object.values(row.cron).some((value) => String(value || "").trim()),
    );
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
      const res = await get<ApiEnvelope<string[]>>(
        "/api/admin/plugin-market/sources",
      );
      plugins.sources = apiData(res) || [];
    } catch {
      plugins.sources = [];
    }
  }
  async function loadPlugins(
    current = 1,
    pageSize = plugins.pageSize,
    refresh = false,
    includeBootstrap = false,
  ) {
    const requestId = ++plugins.requestId;
    plugins.loading = true;
    try {
      plugins.current = current;
      plugins.pageSize = pageSize;
      const params = new URLSearchParams({
        page: String(current),
        page_size: String(pageSize),
        status: plugins.tab,
        keyword: plugins.keyword,
        class: plugins.klass,
      });
      if (includeBootstrap) params.set("include", "sources,settings");
      const endpoint = refresh
        ? "/api/admin/plugin-market-snapshots"
        : "/api/admin/plugin-market/plugins";
      const res = refresh
        ? await post<ApiEnvelope<any>>(`${endpoint}?${params.toString()}`, {})
        : await get<ApiEnvelope<any>>(`${endpoint}?${params.toString()}`);
      if (requestId !== plugins.requestId) return;
      const data = apiData(res) || {};
      const responsePage = Number(data.page);
      if (Number.isInteger(responsePage) && responsePage > 0)
        plugins.current = responsePage;
      plugins.rows = data.data || data.list || [];
      plugins.total = data.total || 0;
      plugins.meta = data;
      if (Array.isArray(data.sources)) plugins.sources = data.sources;
      if (Array.isArray(data.settings)) pluginConfigs.rows = data.settings;
    } finally {
      if (requestId === plugins.requestId) plugins.loading = false;
    }
  }
  async function addPluginSource() {
    const address = plugins.sourceAddress.trim();
    if (!address) {
      message.error("请输入 GitHub 仓库地址或 link:// 地址");
      return;
    }
    plugins.sourceSaving = true;
    try {
      const res = await post<ApiEnvelope<{ count?: number }>>(
        "/api/admin/plugin-market/sources",
        { address },
      );
      const data = apiData(res);
      plugins.sourceAddress = "";
      plugins.tab = "all";
      message.success(
        `插件源已新增${data?.count ? `，发现 ${data.count} 个插件` : ""}`,
      );
      await Promise.all([loadPluginSources(), loadPlugins(1)]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "插件源新增失败");
    } finally {
      plugins.sourceSaving = false;
    }
  }
  async function removePluginSource(address: string) {
    plugins.sourceRemoving[address] = true;
    try {
      await post(
        `/api/admin/plugin-market/source-deletions/${encodeURIComponent(address)}`,
      );
      message.success("插件源已删除");
      await Promise.all([loadPluginSources(), loadPlugins(1)]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "插件源删除失败");
    } finally {
      plugins.sourceRemoving[address] = false;
    }
  }
  function installPlugin(row: PluginInfo) {
    void downloadPluginAndDetectDependencies(row);
  }

  async function downloadPluginAndDetectDependencies(row: PluginInfo) {
    const marketPage = plugins.current;
    const marketPageSize = plugins.pageSize;
    let downloaded = false;
    plugins.installing[row.id] = true;
    try {
      const res = await post<
        ApiEnvelope<{
          errors?: Record<string, string>;
          messages?: Record<string, string>;
        }>
      >("/api/admin/storage/values", {
        [`plugins.${row.id}`]: "download",
      });
      const data = apiData(res) || {};
      const firstError = Object.values(data.errors || {}).find(Boolean);
      if (firstError) {
        throw new ApiError(200, firstError);
      }
      downloaded = true;
      await Promise.all([
        loadPlugins(marketPage, marketPageSize, false, true),
        loadUser(),
      ]);
      const prompted = await offerDownloadedPluginDependencyInstall(
        row,
        marketPage,
        marketPageSize,
      );
      if (!prompted) {
        message.success(row.install_status === 1 ? "已更新" : "已安装");
      }
    } catch (error) {
      const reason = error instanceof Error ? error.message : "未知错误";
      message.error(
        downloaded ? `插件已下载，但依赖检测失败：${reason}` : reason,
      );
    } finally {
      plugins.installing[row.id] = false;
    }
  }
  async function openUninstallPluginModal(row: PluginInfo) {
    plugins.uninstallModal.dependents = [];
    if (row.module) {
      plugins.dependencyChecking[row.id] = true;
      try {
        const res = await get<
          ApiEnvelope<Array<{ id: string; title: string; type: string }>>
        >(`/api/admin/local-plugins/${encodeURIComponent(row.id)}/dependents`);
        plugins.uninstallModal.dependents = apiData(res) || [];
      } catch (error) {
        message.error(
          error instanceof Error ? error.message : "依赖关系检查失败",
        );
        return;
      } finally {
        plugins.dependencyChecking[row.id] = false;
      }
    }
    plugins.uninstallModal.row = row;
    plugins.uninstallModal.deleteConfig = false;
    plugins.uninstallModal.open = true;
  }
  function cancelUninstallPluginModal() {
    if (
      plugins.uninstallModal.row &&
      plugins.uninstalling[plugins.uninstallModal.row.id]
    )
      return;
    plugins.uninstallModal.open = false;
    plugins.uninstallModal.row = null;
    plugins.uninstallModal.deleteConfig = false;
    plugins.uninstallModal.dependents = [];
  }
  async function confirmUninstallPlugin() {
    const row = plugins.uninstallModal.row;
    if (!row) return;
    await uninstallPlugin(row, plugins.uninstallModal.deleteConfig);
  }
  async function uninstallPlugin(row: PluginInfo, deleteConfig = false) {
    plugins.uninstalling[row.id] = true;
    try {
      const res = await post<
        ApiEnvelope<{
          errors?: Record<string, string>;
          messages?: Record<string, string>;
        }>
      >("/api/admin/storage/values", {
        [`plugins.${row.id}`]: "uninstall",
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
        pluginConfigs.marketRow = null;
        pluginConfigs.configurable = false;
      }
      if (deleteConfig) {
        await post(
          `/api/admin/plugin-settings/${encodeURIComponent(row.id)}/deletions?delete_schema=true`,
        );
      }
      plugins.uninstallModal.open = false;
      plugins.uninstallModal.row = null;
      plugins.uninstallModal.deleteConfig = false;
      plugins.uninstallModal.dependents = [];
      message.success(
        deleteConfig
          ? "插件已卸载，配置已同步删除"
          : firstMessage || "插件已卸载",
      );
      await Promise.all([
        loadPlugins(1, plugins.pageSize, false, true),
        loadUser(),
      ]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "插件卸载失败");
    } finally {
      plugins.uninstalling[row.id] = false;
    }
  }
  function pluginStatusLabel(row: PluginInfo) {
    if (row.install_status === 1) return "可更新";
    if (row.install_status === 2 || row.install_status === 6) return "已安装";
    return "未安装";
  }
  function pluginStatusColor(row: PluginInfo) {
    if (row.install_status === 1) return "green";
    if (row.install_status === 2 || row.install_status === 6) return "green";
    return "default";
  }
  function pluginInstalled(row: PluginInfo) {
    return (
      row.install_status === 1 ||
      row.install_status === 2 ||
      row.install_status === 6
    );
  }

  function pluginRuntimeEnabled(row: PluginInfo) {
    return pluginInstalled(row) && row.status !== false;
  }

  async function togglePluginStatus(
    row: PluginInfo,
    status = !pluginRuntimeEnabled(row),
  ) {
    plugins.toggling[row.id] = true;
    try {
      const res = await post<ApiEnvelope<{ id?: string; status?: boolean }>>(
        `/api/admin/local-plugins/${encodeURIComponent(row.id)}/status`,
        { status },
      );
      const data = apiData(res);
      const currentStatus = data?.status ?? status;
      row.status = currentStatus;
      if (plugins.detail?.id === row.id) plugins.detail.status = currentStatus;
      message.success(
        `${row.title || row.id} 已${currentStatus ? "开启" : "关闭"}`,
      );
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "插件状态更新失败",
      );
    } finally {
      plugins.toggling[row.id] = false;
    }
  }

  function pluginUpgradable(row: PluginInfo) {
    return row.install_status === 1;
  }

  function pluginCanOpen(row: PluginInfo) {
    return (
      pluginInstalled(row) &&
      (row.uses_smallcat === true || row.has_user_form === true)
    );
  }

  function pluginCanConfigure(row: PluginInfo) {
    return pluginInstalled(row) && row.config_registered === true;
  }

  function pluginCanManage(row: PluginInfo) {
    return !row.module && (pluginCanConfigure(row) || pluginCanOpen(row));
  }

  function pluginEditorLanguageExtension(): Extension {
    if (!pluginEditorRuntime) return [];
    return pluginEditor.type === "python" ||
      /from sillygirl import|import sillygirl/.test(pluginEditor.content)
      ? pluginEditorRuntime.python()
      : pluginEditorRuntime.javascript();
  }
  function syncPluginEditorContent(value = pluginEditor.content) {
    if (!pluginEditorView) return;
    const current = pluginEditorView.state.doc.toString();
    if (current === value) return;
    pluginEditorView.dispatch({
      changes: { from: 0, to: current.length, insert: value },
    });
  }
  function syncPluginEditorLanguage() {
    if (!pluginEditorLanguage) return;
    pluginEditorView?.dispatch({
      effects: pluginEditorLanguage.reconfigure(
        pluginEditorLanguageExtension(),
      ),
    });
  }
  function pluginEditorThemeExtension(): Extension {
    return pluginEditor.theme === "dark" && pluginEditorRuntime
      ? pluginEditorRuntime.oneDark
      : [];
  }
  function syncPluginEditorTheme() {
    if (!pluginEditorTheme) return;
    pluginEditorView?.dispatch({
      effects: pluginEditorTheme.reconfigure(pluginEditorThemeExtension()),
    });
  }
  function togglePluginEditorTheme() {
    pluginEditor.theme = pluginEditor.theme === "dark" ? "light" : "dark";
    syncPluginEditorTheme();
  }
  function destroyPluginEditor() {
    pluginEditorView?.destroy();
    pluginEditorView = null;
  }
  async function initPluginEditor() {
    if (pluginEditorView || !pluginEditorHost.value) return;
    const runtime = await loadPluginEditorRuntime();
    if (pluginEditorView || !pluginEditorHost.value) return;
    if (!pluginEditorLanguage || !pluginEditorTheme || !pluginEditorEditable)
      return;
    pluginEditorView = new runtime.EditorView({
      parent: pluginEditorHost.value,
      state: runtime.EditorState.create({
        doc: pluginEditor.content,
        extensions: [
          runtime.basicSetup,
          pluginEditorLanguage.of(pluginEditorLanguageExtension()),
          pluginEditorTheme.of(pluginEditorThemeExtension()),
          pluginEditorEditable.of(runtime.EditorView.editable.of(true)),
          runtime.EditorView.updateListener.of((update) => {
            if (update.docChanged)
              pluginEditor.content = update.state.doc.toString();
          }),
        ],
      }),
    });
  }
  function openNewMarketPluginEditor() {
    pluginEditor.isNew = true;
    pluginEditor.id = "";
    pluginEditor.name = "localPlugin";
    pluginEditor.title = "新增本地插件";
    pluginEditor.type = "node";
    pluginEditor.installed = false;
    pluginEditor.row = null;
    pluginEditor.content = pluginEditorStarter;
    pluginEditor.open = true;
    pluginEditor.loading = false;
    nextTick(() => {
      destroyPluginEditor();
      void initPluginEditor();
    });
  }
  async function openMarketPluginEditor(row: PluginInfo) {
    pluginEditor.isNew = false;
    pluginEditor.id = row.id;
    pluginEditor.name = row.title || row.id;
    pluginEditor.title = row.title || row.id;
    pluginEditor.type = marketPluginDependencyRuntime(row);
    pluginEditor.installed = pluginInstalled(row);
    pluginEditor.row = row;
    pluginEditor.content = "";
    pluginEditor.open = true;
    pluginEditor.loading = true;
    await nextTick();
    destroyPluginEditor();
    await initPluginEditor();
    try {
      const res = await get<
        ApiEnvelope<{
          id: string;
          title?: string;
          name?: string;
          type?: string;
          installed?: boolean;
          content: string;
        }>
      >(`/api/admin/local-plugins/${encodeURIComponent(row.id)}`);
      const data = apiData(res);
      pluginEditor.id = data.id || row.id;
      pluginEditor.name = data.name || row.title || row.id;
      pluginEditor.title = data.title || row.title || row.id;
      pluginEditor.type = data.type === "python" ? "python" : "node";
      pluginEditor.installed = data.installed !== false;
      pluginEditor.content = data.content || "";
      syncPluginEditorLanguage();
      syncPluginEditorContent(pluginEditor.content);
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "读取插件源码失败",
      );
    } finally {
      pluginEditor.loading = false;
    }
  }
  function closeMarketPluginEditor() {
    pluginEditor.open = false;
    destroyPluginEditor();
  }
  function handlePluginEditorOpenChange(open: boolean) {
    if (open) nextTick(() => void initPluginEditor());
    else destroyPluginEditor();
  }
  function pluginEditorMetaValue(content: string, key: string) {
    const metaKeyWanted = key.toLowerCase();
    let value = "";
    for (const line of String(content || "").split(/\r?\n/)) {
      const legacy =
        /^[ \t]*(?:\/\/|#+)[ \t]*\[[ \t]*([\d\w+-]+)[ \t]*:[ \t]*(.*)[ \t]*\][^\r\n]*$/.exec(
          line,
        );
      if (legacy) {
        const metaKey = String(legacy[1] || "").toLowerCase();
        const metaValue = String(legacy[2] || "").trim();
        if (metaKey === metaKeyWanted && metaValue) value = metaValue;
        continue;
      }
      const at = /^[ \t]*(?:\*[ \t]*)?@([\d\w+-]+)(?:[ \t]+(.+?))?[ \t]*$/.exec(
        line,
      );
      if (!at) continue;
      const metaKey = String(at[1] || "").toLowerCase();
      const metaValue = String(at[2] || "").trim();
      if (metaKey === metaKeyWanted && metaValue) value = metaValue;
    }
    return value;
  }

  function pluginEditorMetaEnabled(content: string, key: string) {
    const value = pluginEditorMetaValue(content, key).toLowerCase();
    return (
      value === "true" || value === "1" || value === "yes" || value === "on"
    );
  }
  function normalizePluginEditorFileBase(value: string) {
    return String(value || "")
      .trim()
      .replace(/\\/g, "/")
      .split("/")
      .pop()!
      .replace(/\.(js|py)$/i, "");
  }
  function validatePluginEditorRequired() {
    const content = pluginEditor.content || "";
    const missing: string[] = [];
    for (const item of ["title", "name", "desc", "version"]) {
      if (!pluginEditorMetaValue(content, item)) {
        missing.push(
          (
            {
              title: "[title: xxx]",
              name: "[name: 文件名]",
              desc: "[desc: xxx]",
              version: "[version: vx.y.z]",
            } as Record<string, string>
          )[item],
        );
      }
    }
    if (
      !pluginEditorMetaValue(content, "rule") &&
      !pluginEditorMetaValue(content, "cron") &&
      !pluginEditorMetaEnabled(content, "on_start") &&
      !pluginEditorMetaEnabled(content, "web") &&
      !pluginEditorMetaEnabled(content, "module")
    ) {
      missing.push(
        "[rule: xxx] 或 [cron: xxx]/[on_start: true]/[web: true]/[module: true]",
      );
    }
    if (missing.length) {
      message.warning(`插件注释缺少必须字段：${missing.join("、")}`);
      return false;
    }
    const inputName = normalizePluginEditorFileBase(pluginEditor.name);
    const metaName = normalizePluginEditorFileBase(
      pluginEditorMetaValue(content, "name"),
    );
    if (!inputName) {
      message.warning("插件名称不能为空，且必须和 [name: 文件名] 一致");
      return false;
    }
    if (inputName !== metaName) {
      message.warning(
        `插件名称必须和 [name: ${metaName || "文件名"}] 一致，当前填写：${inputName}`,
      );
      return false;
    }
    return true;
  }

  async function formatMarketPluginEditor() {
    if (!pluginEditor.content.trim()) return;
    if (pluginEditor.type === "python") {
      message.info("Python 插件暂不做前端格式化，请保存前自行确认缩进");
      return;
    }
    try {
      const [
        { default: prettier },
        { default: parserBabel },
        { default: parserEstree },
      ] = await Promise.all([
        import("prettier/standalone"),
        import("prettier/plugins/babel"),
        import("prettier/plugins/estree"),
      ]);
      const formatted = await prettier.format(pluginEditor.content, {
        parser: "babel",
        plugins: [parserBabel, parserEstree],
        singleQuote: true,
        trailingComma: "all",
      });
      pluginEditor.content = formatted.trimEnd() + "\n";
      syncPluginEditorContent(pluginEditor.content);
      message.success("格式化完成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "格式化失败");
    }
  }
  async function saveMarketPluginEditor() {
    const creatingPlugin = pluginEditor.isNew || !pluginEditor.installed;
    const currentMarketPage = plugins.current;
    const nameInput = document.getElementById(
      "plugin-editor-name",
    ) as HTMLInputElement | null;
    if (nameInput) pluginEditor.name = nameInput.value;
    if (!validatePluginEditorRequired()) return;
    pluginEditor.saving = true;
    try {
      const payload = {
        id: pluginEditor.id,
        name: pluginEditor.name,
        type: pluginEditor.type,
        content: pluginEditor.content,
      };
      const res = creatingPlugin
        ? await post<
            ApiEnvelope<{
              id: string;
              type?: string;
              title?: string;
              path?: string;
            }>
          >("/api/admin/local-plugins", payload)
        : await post<
            ApiEnvelope<{
              id: string;
              type?: string;
              title?: string;
              path?: string;
            }>
          >(
            `/api/admin/local-plugins/${encodeURIComponent(pluginEditor.id)}`,
            payload,
          );
      const data = apiData(res);
      pluginEditor.id = data?.id || pluginEditor.id;
      pluginEditor.installed = true;
      const savedRuntime: DependencyRuntime =
        data?.type === "python" || pluginEditor.type === "python"
          ? "python"
          : "node";
      const savedDependencies = declaredPluginDependenciesFromContent(
        pluginEditor.content,
      );
      const savedStatus = pluginEditorMetaValue(pluginEditor.content, "status");
      const savedRow: PluginInfo = {
        id: pluginEditor.id,
        title:
          data?.title ||
          pluginEditor.name ||
          pluginEditor.title ||
          pluginEditor.id,
        type: savedRuntime,
        suffix: savedRuntime === "python" ? ".py" : ".js",
        status: savedStatus
          ? pluginEditorMetaEnabled(pluginEditor.content, "status")
          : true,
        install_status: 2,
        address: data?.path
          ? `local://?path=${encodeURIComponent(data.path)}`
          : "",
        dependencies: savedDependencies,
      };
      message.success(creatingPlugin ? "本地插件已新增" : "插件已保存");
      pluginEditor.open = false;
      destroyPluginEditor();
      if (creatingPlugin) plugins.tab = "private";
      await Promise.all([
        loadUser(),
        loadPlugins(
          creatingPlugin ? 1 : currentMarketPage,
          plugins.pageSize,
          true,
        ),
      ]);
      try {
        await offerPluginDependencyInstall(savedRow);
      } catch (error) {
        message.warning(
          `插件已保存，但依赖检测失败：${error instanceof Error ? error.message : "未知错误"}`,
        );
      }
    } catch (error) {
      message.error(error instanceof Error ? error.message : "保存插件失败");
    } finally {
      pluginEditor.saving = false;
    }
  }
  async function deleteMarketPluginEditor() {
    if (!pluginEditor.id || !pluginEditor.installed) return;
    pluginEditor.deleting = true;
    try {
      await post(
        `/api/admin/local-plugins/${encodeURIComponent(pluginEditor.id)}/deletions`,
      );
      message.success("插件已删除");
      pluginEditor.open = false;
      destroyPluginEditor();
      await Promise.all([loadUser(), loadPlugins(1, plugins.pageSize, true)]);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "删除插件失败");
    } finally {
      pluginEditor.deleting = false;
    }
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

  type DependencyRuntime = "node" | "python";
  type DependencyToolStatus = {
    available: boolean;
    path?: string;
    message?: string;
    registry?: string;
    target?: string;
  };
  type PluginDependencyPlan = {
    runtime: DependencyRuntime;
    plugin: string;
    pluginTitle: string;
    dependencies: NodeDependencyRow[];
    tool: DependencyToolStatus;
  };

  function marketPluginSourcePath(row: PluginInfo) {
    const address = `${row.address || ""}`.trim();
    if (!address) return "";
    const query = address.includes("?")
      ? address.slice(address.indexOf("?") + 1)
      : "";
    const params = new URLSearchParams(query);
    const candidate = params.get("path") || params.get("raw") || address;
    try {
      const pathname =
        candidate.startsWith("http://") || candidate.startsWith("https://")
          ? new URL(candidate).pathname
          : decodeURIComponent(candidate).split("?")[0];
      return pathname.replace(/\\/g, "/");
    } catch {
      return candidate.split("?")[0].replace(/\\/g, "/");
    }
  }

  function marketPluginFileName(row: PluginInfo) {
    return marketPluginSourcePath(row).split("/").filter(Boolean).pop() || "";
  }

  function marketPluginDependencyRuntime(row: PluginInfo): DependencyRuntime {
    return row.type === "python" || row.suffix?.toLowerCase() === ".py"
      ? "python"
      : "node";
  }

  async function resolvePluginDependencyPlan(
    row: PluginInfo,
  ): Promise<PluginDependencyPlan | null> {
    if (!Array.isArray(row.dependencies) || row.dependencies.length === 0)
      return null;
    const runtime = marketPluginDependencyRuntime(row);
    const listRes = await get<
      ApiEnvelope<{
        plugins: NodeDependencyPlugin[];
        dependencies: NodeDependencyRow[];
        tool: DependencyToolStatus;
      }>
    >(`/api/admin/dependencies?runtime=${encodeURIComponent(runtime)}`);
    const listData = apiData(listRes);
    const sourcePath = marketPluginSourcePath(row).toLowerCase();
    const fileName = marketPluginFileName(row).toLowerCase();
    const fallbackName = fileName.replace(/\.(js|py)$/i, "");
    const plugin =
      (sourcePath
        ? (listData.plugins || []).find(
            (item) =>
              `${item.path || ""}`.replace(/\\/g, "/").toLowerCase() ===
              sourcePath,
          )
        : undefined) ||
      (listData.plugins || []).find(
        (item) => `${item.file || ""}`.toLowerCase() === fileName,
      ) ||
      (listData.plugins || []).find((item) => item.name === fallbackName) ||
      (listData.plugins || []).find((item) => item.title === row.title);
    if (!plugin) {
      throw new Error(`未识别到已安装插件文件 ${fileName || row.title}`);
    }
    const detailRes = await get<
      ApiEnvelope<{
        dependencies: NodeDependencyRow[];
        tool: DependencyToolStatus;
      }>
    >(
      `/api/admin/dependencies?runtime=${encodeURIComponent(runtime)}&plugin=${encodeURIComponent(plugin.name)}`,
    );
    const detail = apiData(detailRes);
    const missing = (detail.dependencies || []).filter(
      (item) => !item.installed,
    );
    if (missing.length === 0) return null;
    return {
      runtime,
      plugin: plugin.name,
      pluginTitle: row.title || plugin.title || plugin.name,
      dependencies: missing,
      tool: detail.tool || listData.tool,
    };
  }

  async function installMarketPluginDependencies(
    plan: PluginDependencyPlan,
    marketPage = plugins.current,
    marketPageSize = plugins.pageSize,
  ) {
    const messageKey = `plugin-dependencies-${plan.runtime}-${plan.plugin}`;
    message.loading({
      key: messageKey,
      content: `正在安装 ${plan.dependencies.length} 个依赖…`,
      duration: 0,
    });
    const failed: string[] = [];
    for (const dependency of plan.dependencies) {
      try {
        await post("/api/admin/dependencies", {
          runtime: plan.runtime,
          plugin: plan.plugin,
          package: dependency.name,
          dev: dependency.dev,
        });
      } catch (error) {
        failed.push(
          `${dependency.name}：${error instanceof Error ? error.message : "安装失败"}`,
        );
      }
    }
    if (failed.length > 0) {
      message.error({
        key: messageKey,
        content: `依赖安装失败：${failed.join("；")}`,
        duration: 6,
      });
      throw new Error(failed.join("；"));
    }
    message.success({
      key: messageKey,
      content: `${plan.pluginTitle} 的依赖已全部安装`,
      duration: 3,
    });
    await Promise.all([
      loadUser(false),
      loadPlugins(marketPage, marketPageSize, false, true),
    ]);
  }

  async function installDownloadedPluginDependencies(
    row: PluginInfo,
    plan: DownloadedPluginDependencyPlan,
    marketPage: number,
    marketPageSize: number,
  ) {
    if (plan.moduleDependencies.length > 0) {
      const res = await post<
        ApiEnvelope<{
          errors?: Record<string, string>;
        }>
      >("/api/admin/storage/values", {
        [`plugins.${row.id}`]: "install-dependencies",
      });
      const firstError = Object.values(apiData(res)?.errors || {}).find(Boolean);
      if (firstError) throw new ApiError(200, firstError);
      message.success(`${plan.pluginTitle} 的依赖模块和运行依赖已处理`);
      await Promise.all([
        loadUser(false),
        loadPlugins(marketPage, marketPageSize, false, true),
      ]);
      return;
    }
    await installMarketPluginDependencies(plan, marketPage, marketPageSize);
  }

  async function offerDownloadedPluginDependencyInstall(
    row: PluginInfo,
    marketPage: number,
    marketPageSize: number,
  ) {
    const plan = await resolveDownloadedPluginDependencyPlan(row);
    if (
      plan.dependencies.length === 0 &&
      plan.moduleDependencies.length === 0
    ) {
      return false;
    }
    confirmDownloadedPluginDependencyInstall(plan, () =>
      installDownloadedPluginDependencies(row, plan, marketPage, marketPageSize),
    );
    return true;
  }

  async function offerPluginDependencyInstall(
    row: PluginInfo,
    marketPage = plugins.current,
    marketPageSize = plugins.pageSize,
  ) {
    const plan = await resolvePluginDependencyPlan(row);
    if (!plan) return;
    const names = plan.dependencies.map((item) => item.name).join("、");
    const toolWarning =
      plan.tool?.available === false
        ? `安装工具当前不可用：${plan.tool.message || (plan.runtime === "python" ? "pipx/Python 未就绪" : "pnpm 未就绪")}`
        : "";
    Modal.confirm({
      title: `${plan.pluginTitle} 需要安装依赖`,
      content: h("div", { class: "plugin-dependency-confirm" }, [
        h("p", `检测到 ${plan.dependencies.length} 个未安装依赖：`),
        h("p", { class: "mono" }, names),
        toolWarning ? h("p", { style: "color: #d46b08" }, toolWarning) : null,
        h("p", "是否现在自动安装？"),
      ]),
      okText: "自动安装",
      cancelText: "暂不安装",
      centered: true,
      onOk: () =>
        installMarketPluginDependencies(plan, marketPage, marketPageSize),
    });
  }

  const dependencyRuntimeOptions = [
    { label: "NodeJS", value: "node" },
    { label: "Python", value: "python" },
  ];

  const nodeDeps = reactive({
    runtime: "node" as DependencyRuntime,
    plugins: [] as NodeDependencyPlugin[],
    plugin: "",
    rows: [] as NodeDependencyRow[],
    packageName: "",
    registry: "https://registry.npmmirror.com",
    dev: false,
    loading: false,
    saving: false,
    removing: {} as Record<string, boolean>,
    pnpm: {
      available: false,
      path: "",
      message: "",
      registry: "",
    } as DependencyToolStatus,
    pipx: {
      available: false,
      path: "",
      message: "",
      registry: "",
      target: "",
    } as DependencyToolStatus,
  });
  const currentDependencyTool = computed(() =>
    nodeDeps.runtime === "python" ? nodeDeps.pipx : nodeDeps.pnpm,
  );
  const dependencyRuntimeLabel = computed(() =>
    nodeDeps.runtime === "python" ? "Python" : "NodeJS",
  );
  const dependencySharedPath = computed(() =>
    nodeDeps.runtime === "python"
      ? "/data/plugins/python_packages/venvs/sillygirl-python-runtime"
      : "/data/plugins/node_modules",
  );
  const dependencyRegistryLabel = computed(() =>
    nodeDeps.runtime === "python" ? "pipx 源" : "pnpm 镜像",
  );
  const dependencyPackagePlaceholder = computed(() =>
    nodeDeps.runtime === "python"
      ? "依赖名，例如 requests 或 requests==2.32.0"
      : "依赖名，例如 axios 或 ipp@latest",
  );
  const dependencyPluginOptions = computed(() => [
    { label: `全部 ${dependencyRuntimeLabel.value} 插件`, value: "" },
    ...nodeDeps.plugins.map((item) => ({
      label: `${item.title || item.name} / ${item.file || dependencyPluginFileName(item)}`,
      value: item.name,
    })),
  ]);
  function showDependencyInstallResult(output: unknown) {
    const text = String(output || "").trim();
    if (text.includes("插件配置表单重载失败：")) {
      message.warning(text.slice(text.lastIndexOf("插件配置表单重载失败：")));
      return;
    }
    message.success("依赖已安装，插件配置表单已自动重载");
  }
  async function loadNodeDependencies(plugin = nodeDeps.plugin) {
    nodeDeps.loading = true;
    try {
      const query = new URLSearchParams({ runtime: nodeDeps.runtime });
      if (plugin) query.set("plugin", plugin);
      const res = await get<
        ApiEnvelope<{
          runtime: DependencyRuntime;
          plugins: NodeDependencyPlugin[];
          plugin: string;
          dependencies: NodeDependencyRow[];
          pnpm: DependencyToolStatus;
          pipx: DependencyToolStatus;
        }>
      >(`/api/admin/dependencies?${query.toString()}`);
      const data = apiData(res) || {};
      nodeDeps.plugins = data.plugins || [];
      nodeDeps.plugin = data.plugin || "";
      nodeDeps.rows = data.dependencies || [];
      nodeDeps.pnpm = data.pnpm || { available: false };
      nodeDeps.pipx = data.pipx || { available: false };
      nodeDeps.registry =
        currentDependencyTool.value.registry ||
        (nodeDeps.runtime === "python"
          ? "https://pypi.tuna.tsinghua.edu.cn/simple"
          : "https://registry.npmmirror.com");
    } finally {
      nodeDeps.loading = false;
    }
  }
  async function installNodeDependency() {
    await installNodeDependencyPackage(nodeDeps.packageName.trim(), () => {
      nodeDeps.packageName = "";
    });
  }
  async function installNodeDependencyPackage(pkg: string, after?: () => void) {
    if (!pkg) {
      message.error("请输入依赖名称");
      return;
    }
    nodeDeps.saving = true;
    try {
      const res = await post<ApiEnvelope<string>>("/api/admin/dependencies", {
        runtime: nodeDeps.runtime,
        plugin: nodeDeps.plugin || "__shared__",
        package: pkg,
        dev: nodeDeps.dev,
      });
      after?.();
      showDependencyInstallResult(apiData(res));
      await loadNodeDependencies(nodeDeps.plugin);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "依赖安装失败");
    } finally {
      nodeDeps.saving = false;
    }
  }
  async function installNodeDependencyRow(row: NodeDependencyRow) {
    nodeDeps.saving = true;
    try {
      const res = await post<ApiEnvelope<string>>("/api/admin/dependencies", {
        runtime: nodeDeps.runtime,
        plugin: row.plugin || "__shared__",
        package: row.name,
        dev: row.dev,
      });
      showDependencyInstallResult(apiData(res));
      await loadNodeDependencies(nodeDeps.plugin);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "依赖安装失败");
    } finally {
      nodeDeps.saving = false;
    }
  }
  async function removeNodeDependency(row: NodeDependencyRow) {
    const key = `${nodeDeps.runtime}.${row.plugin}.${row.name}`;
    nodeDeps.removing[key] = true;
    try {
      await post("/api/admin/dependency-deletions", {
        runtime: nodeDeps.runtime,
        plugin: row.plugin || "__shared__",
        package: row.name,
      });
      message.success("依赖已卸载");
      await loadNodeDependencies(nodeDeps.plugin);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "依赖卸载失败");
    } finally {
      nodeDeps.removing[key] = false;
    }
  }

  const pluginConfigs = reactive({
    rows: [] as any[],
    selected: null as any,
    marketRow: null as PluginInfo | null,
    form: {} as Record<string, any>,
    text: {} as Record<string, string>,
    configurable: false,
    openToUsers: false,
    loading: false,
    saving: false,
    modalOpen: false,
    opening: "",
  });
  const schemaFields = computed(() => {
    const props = pluginConfigs.selected?.schema?.properties || {};
    return Object.entries(props).map(([key, prop]) => ({
      key,
      prop: prop as any,
    }));
  });
  const pluginOpenAvailable = computed(() =>
    Boolean(pluginConfigs.marketRow && pluginCanOpen(pluginConfigs.marketRow)),
  );
  const pluginSettingsCanSave = computed(() =>
    Boolean(
      pluginConfigs.selected &&
      (pluginConfigs.configurable || pluginOpenAvailable.value),
    ),
  );
  function openPluginConfig(
    row: any,
    marketRow: PluginInfo | null = null,
    configurable = true,
  ) {
    pluginConfigs.selected = row;
    pluginConfigs.marketRow = marketRow;
    pluginConfigs.configurable = configurable;
    pluginConfigs.openToUsers = Boolean(marketRow?.open);
    const values = { ...(row.user_config || {}) };
    for (const [key, prop] of Object.entries(
      row.schema?.properties || {},
    ) as Array<[string, any]>) {
      if (values[key] === undefined && prop.default !== undefined)
        values[key] = prop.default;
    }
    pluginConfigs.form = values;
    pluginConfigs.text = {};
    for (const [key, value] of Object.entries(values)) {
      if (typeof value === "object" && value !== null) {
        pluginConfigs.text[key] = JSON.stringify(value, null, 2);
      }
    }
  }
  function fieldOptions(prop: any) {
    const values = prop?.enum || [];
    const names = prop?.enumNames || [];
    return values.map((value: any, index: number) => ({
      value,
      label: names[index] || String(value),
    }));
  }
  function fieldType(prop: any) {
    if (Array.isArray(prop?.enum)) return "enum";
    return prop?.type || "string";
  }
  type PluginPanelKind = "smallcat" | "qinglong" | "daidai";
  function pluginPanelKind(field: {
    key: string;
    prop?: any;
  }): PluginPanelKind | null {
    const widget = String(field.prop?.["ui:widget"] || "").toLowerCase();
    if (widget === "smallcat-panel") return "smallcat";
    if (widget === "qinglong-panel") return "qinglong";
    if (widget === "daidai-panel") return "daidai";
    const key = String(field.key || "")
      .toLowerCase()
      .replace(/[^a-z0-9]/g, "");
    const title = String(field.prop?.title || "");
    if (key === "smallcatid" || /smallcat.*编号/i.test(title))
      return "smallcat";
    if (key === "qinglongid" || key === "qlid" || /青龙.*编号/.test(title))
      return "qinglong";
    if (key === "daidaiid" || key === "ddid" || /呆呆.*编号/.test(title))
      return "daidai";
    return null;
  }
  function pluginPanelChoices(field: { key: string; prop?: any }) {
    const kind = pluginPanelKind(field);
    let available = 0;
    if (kind === "smallcat")
      available = Math.max(smallcat.total, smallcat.rows.length);
    if (kind === "qinglong")
      available = Math.max(qinglong.total, qinglong.rows.length);
    if (kind === "daidai")
      available = Math.max(daidai.total, daidai.rows.length);
    return Array.from(
      { length: Math.max(available, 0) },
      (_, index) => index + 1,
    );
  }
  function pluginPanelEmptyText(field: { key: string; prop?: any }) {
    const labels: Record<PluginPanelKind, string> = {
      smallcat: "SmallCat",
      qinglong: "青龙",
      daidai: "呆呆",
    };
    const kind = pluginPanelKind(field);
    return kind ? `暂无已配置的${labels[kind]}容器，请先到容器管理中添加` : "";
  }
  const VISIBLE_WHEN_OP_MAP: Record<string, string> = {
    "==": "==", "===": "==",
    "!=": "!=", "!==": "!=",
    ">": ">", ">=": ">=", "<": "<", "<=": "<=",
  };
  function normalizeVisibleOp(op: any): string {
    return VISIBLE_WHEN_OP_MAP[String(op == null ? "==" : op)] || "==";
  }
  function compareValues(a: any, b: any): number {
    const na = Number(a);
    const nb = Number(b);
    if (!Number.isNaN(na) && !Number.isNaN(nb)) return na - nb;
    return String(a ?? "").localeCompare(String(b ?? ""));
  }
  function evalVisibleWhen(
    rule: any,
    form: Record<string, any> | undefined,
  ): boolean {
    if (!rule) return true;
    const list = Array.isArray(rule) ? rule : [rule];
    if (!list.length) return true;
    return (list as Array<any>).every((c: any) => {
      const op = normalizeVisibleOp(c.op);
      const actual = form ? form[c.field] : undefined;
      const target = c.value;
      switch (op) {
        case "!=": return String(actual ?? "") !== String(target ?? "");
        case ">": return compareValues(actual, target) > 0;
        case ">=": return compareValues(actual, target) >= 0;
        case "<": return compareValues(actual, target) < 0;
        case "<=": return compareValues(actual, target) <= 0;
        case "==":
        default: return String(actual ?? "") === String(target ?? "");
      }
    });
  }
  function pluginConfigFieldVisible(field: { key: string; prop: any }) {
    const prop = (field && field.prop) || {};
    // 通用：schema 声明的条件可见规则（ui:visibleWhen），由插件自行声明，不再依赖前端硬编码特例。
    // 规则格式：{ field, op: "=="|"!="|">"|">="|"<"|"<=", value } 或这类对象的数组（多条件 AND）。
    if (prop["ui:visibleWhen"] !== undefined) {
      return evalVisibleWhen(prop["ui:visibleWhen"], pluginConfigs.form);
    }
    // 以下为旧版硬编码兼容分支，保留以兼容历史插件（未来可移除）。
    const properties = pluginConfigs.selected?.schema?.properties || {};
    if (Object.prototype.hasOwnProperty.call(properties, "sync_panel")) {
      const syncPanel = String(pluginConfigs.form.sync_panel || "");
      if (field.key === "qinglong_id") return syncPanel === "qinglong";
      if (field.key === "daidai_id") return syncPanel === "daidai";
    }
    if (!Object.prototype.hasOwnProperty.call(properties, "account_mode"))
      return true;
    const title = String(field.prop?.title || "");
    const description = String(field.prop?.description || "");
    const isManualOnly =
      field.key === "openid" ||
      field.key === "manual_openids" ||
      field.key === "accounts_json" ||
      /手动\s*openid/i.test(title) ||
      /仅手动填写模式生效/.test(description);
    return !isManualOnly || pluginConfigs.form.account_mode === "manual";
  }
  async function savePluginConfig() {
    if (!pluginConfigs.selected) return;
    const value = { ...pluginConfigs.form };
    if (pluginConfigs.configurable) {
      for (const field of schemaFields.value) {
        const type = fieldType(field.prop);
        if (
          (type === "object" || type === "array") &&
          pluginConfigs.text[field.key] !== undefined
        ) {
          try {
            value[field.key] = JSON.parse(
              pluginConfigs.text[field.key] || (type === "array" ? "[]" : "{}"),
            );
          } catch {
            message.error(`${field.prop.title || field.key} JSON 格式错误`);
            return;
          }
        }
      }
    }
    pluginConfigs.saving = true;
    try {
      if (pluginConfigs.configurable) {
        await putPluginConfig(pluginConfigs.selected.uuid, value);
      }
      const marketRow = pluginConfigs.marketRow;
      if (
        marketRow &&
        pluginCanOpen(marketRow) &&
        Boolean(marketRow.open) !== pluginConfigs.openToUsers
      ) {
        const res = await post<ApiEnvelope<{ uuid: string; open: boolean }>>(
          `/api/admin/plugins/${encodeURIComponent(marketRow.id)}/access`,
          {
            open: pluginConfigs.openToUsers,
          },
        );
        const data = apiData(res);
        marketRow.open = data.open;
        if (plugins.detail?.id === marketRow.id)
          plugins.detail.open = data.open;
      }
      message.success("插件设置已保存");
      pluginConfigs.modalOpen = false;
      pluginConfigs.selected = null;
      pluginConfigs.marketRow = null;
      pluginConfigs.configurable = false;
      await loadPlugins(plugins.current, plugins.pageSize, false, true);
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "插件设置保存失败",
      );
    } finally {
      pluginConfigs.saving = false;
    }
  }
  async function putPluginConfig(uuid: string, value: Record<string, any>) {
    await post(`/api/admin/plugin-settings/${encodeURIComponent(uuid)}`, {
      value,
    });
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
    qqguild_mode: "webhook" | "websocket";
    qqguild_app_id: string;
    qqguild_app_secret: string;
    qqguild_sandbox: boolean;
    qqguild_debug: boolean;
    pagermaid_enable: boolean;
    pagermaid_token: string;
    pagermaid_debug: boolean;
    web_chat_public: boolean;
  };

  type BotSettingsPlatform =
    | "clawbot"
    | "qq"
    | "telegram"
    | "dingtalk"
    | "qqguild"
    | "web"
    | "pagermaid";

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
      clawbot_token: "",
      clawbot_api_base: "https://ilinkai.weixin.qq.com",
      clawbot_debug: false,
      qq_enable: true,
      qq_token: "",
      qq_debug: false,
      telegram_token: "",
      telegram_enable: true,
      telegram_api_base: "https://api.telegram.org",
      telegram_debug: false,
      dingtalk_enable: true,
      dingtalk_client_id: "",
      dingtalk_client_secret: "",
      dingtalk_debug: false,
      qqguild_enable: true,
      qqguild_mode: "webhook",
      qqguild_app_id: "",
      qqguild_app_secret: "",
      qqguild_sandbox: false,
      qqguild_debug: false,
      pagermaid_enable: true,
      pagermaid_token: "",
      pagermaid_debug: false,
      web_chat_public: false,
    } as BotSettingsForm,
  });
  const botSettingsModal = reactive({
    open: false,
    platform: "clawbot" as BotSettingsPlatform,
    label: "微信 ClawBot",
    snapshot: null as BotSettingsForm | null,
  });
  const clawbotLogin = reactive({
    open: false,
    starting: false,
    polling: false,
    session: "",
    qrcodeUrl: "",
    qrcodeImg: "",
    message: "",
    status: "",
    needVerify: false,
    verifyCode: "",
    expiresAt: 0,
  });
  let clawbotLoginPollTimer: number | null = null;

  function clearClawbotLoginPoll() {
    if (clawbotLoginPollTimer) {
      window.clearTimeout(clawbotLoginPollTimer);
      clawbotLoginPollTimer = null;
    }
  }
  async function openMarketPluginConfig(row: PluginInfo) {
    if (!pluginInstalled(row)) {
      message.info("请先安装插件后再设置");
      return;
    }
    pluginConfigs.opening = row.id;
    try {
      const res = await get<
        ApiEnvelope<{ config?: any; panels?: AdminPanelsResponse }>
      >(
        `/api/admin/plugin-settings/${encodeURIComponent(row.id)}?include=panels`,
      );
      const resource = apiData(res) || {};
      const config = resource.config;
      if (config) {
        const index = pluginConfigs.rows.findIndex(
          (item) => item.uuid === config.uuid,
        );
        if (index >= 0) pluginConfigs.rows[index] = config;
        else pluginConfigs.rows.push(config);
      }
      const properties = config?.schema?.properties || {};
      const panelKinds = new Set<PluginPanelKind>();
      for (const [key, prop] of Object.entries(properties)) {
        const kind = pluginPanelKind({ key, prop });
        if (kind) panelKinds.add(kind);
      }
      if (pluginCanOpen(row)) panelKinds.add("smallcat");
      if (panelKinds.size > 0) applyAdminPanels(resource.panels);
      const selected = config || {
        uuid: row.id,
        plugin: row.title || row.id,
        title: row.title || row.id,
        file: row.address || row.id,
        registered: row.has_form === true ? false : true,
        schema: { type: "object", properties: {} },
        user_config: {},
      };
      openPluginConfig(selected, row, Boolean(config));
      pluginConfigs.modalOpen = true;
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "插件设置加载失败",
      );
    } finally {
      pluginConfigs.opening = "";
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
      const res = await post<ApiEnvelope<ClawbotLoginStart>>(
        "/api/admin/clawbot-login-sessions",
        {},
      );
      const data = apiData(res);
      const qrcodeImg = data.qrcode_url
        ? await QRCode.toDataURL(data.qrcode_url, {
            errorCorrectionLevel: "M",
            margin: 1,
            width: 260,
          })
        : "";
      Object.assign(clawbotLogin, {
        session: data.session,
        qrcodeUrl: data.qrcode_url,
        qrcodeImg,
        expiresAt: data.expires_at,
        status: data.status || "wait",
        message: data.message || "请使用微信扫码",
        needVerify: false,
        verifyCode: "",
      });
      scheduleClawbotLoginPoll(300);
    } catch (error) {
      clawbotLogin.message =
        error instanceof Error ? error.message : "生成二维码失败";
      message.error(clawbotLogin.message);
    } finally {
      clawbotLogin.starting = false;
    }
  }

  async function pollClawbotLogin(verifyCode = "") {
    if (!clawbotLogin.session) return;
    clawbotLogin.polling = true;
    try {
      const sessionEndpoint = `/api/admin/clawbot-login-sessions/${encodeURIComponent(clawbotLogin.session)}`;
      const res = verifyCode.trim()
        ? await post<ApiEnvelope<ClawbotLoginStatus>>(
            `${sessionEndpoint}/verification-attempts`,
            { verify_code: verifyCode.trim() },
          )
        : await get<ApiEnvelope<ClawbotLoginStatus>>(sessionEndpoint);
      const data = apiData(res);
      clawbotLogin.status = data.status || "wait";
      clawbotLogin.message = data.message || "等待扫码";
      clawbotLogin.needVerify = !!data.need_verify;
      if (!clawbotLogin.open && !data.connected) {
        clearClawbotLoginPoll();
        return;
      }
      if (data.connected) {
        message.success("ClawBot Token 已保存");
        clearClawbotLoginPoll();
        await loadBots();
        await loadUser(false);
        return;
      }
      if (
        data.already_bound ||
        ["expired", "verify_code_blocked"].includes(data.status)
      ) {
        clearClawbotLoginPoll();
        return;
      }
      if (!data.need_verify && clawbotLogin.open) {
        scheduleClawbotLoginPoll(1000);
      }
    } catch (error) {
      clearClawbotLoginPoll();
      clawbotLogin.message =
        error instanceof Error ? error.message : "轮询扫码状态失败";
      message.error(clawbotLogin.message);
    } finally {
      clawbotLogin.polling = false;
    }
  }

  async function submitClawbotVerifyCode() {
    const code = clawbotLogin.verifyCode.trim();
    if (!code) {
      message.error("请输入验证码");
      return;
    }
    clawbotLogin.needVerify = false;
    clawbotLogin.verifyCode = "";
    await pollClawbotLogin(code);
  }

  const botStatusRows = computed(() => overviewAdapters.value);
  const oneBotReceiveURL = computed(() => {
    const wsProtocol = window.location.protocol === "https:" ? "wss" : "ws";
    const host =
      window.location.port === "5173"
        ? `${window.location.hostname}:8080`
        : window.location.host;
    return `${wsProtocol}://${host}/qq/receive`;
  });
  const qqGuildWebhookURL = computed(() => {
    const host =
      window.location.port === "5173"
        ? `${window.location.hostname}:8080`
        : window.location.host;
    return `${window.location.protocol}//${host}/qqguild/webhook`;
  });
  const webChatEndpointURL = computed(() => {
    const host =
      window.location.port === "5173"
        ? `${window.location.hostname}:8080`
        : window.location.host;
    return `${window.location.protocol}//${host}/api/web-chat/messages`;
  });
  const pagermaidBridgeURL = computed(() => {
    const wsProtocol = window.location.protocol === "https:" ? "wss" : "ws";
    const host =
      window.location.port === "5173"
        ? `${window.location.hostname}:8080`
        : window.location.host;
    const token = botSettings.form.pagermaid_token.trim();
    const query = token ? `?token=${encodeURIComponent(token)}` : "";
    return `${wsProtocol}://${host}/pagermaid/receive${query}`;
  });

  function boolSetting(value: unknown, fallback = false) {
    if (value === undefined || value === null || value === "") return fallback;
    return value === true || String(value).toLowerCase() === "true";
  }

  async function loadBots() {
    botSettings.loading = true;
    try {
      const res =
        await get<
          ApiEnvelope<{ settings: Record<string, any>; statuses: any[] }>
        >("/api/admin/bots");
      const resource = apiData(res);
      const data = resource?.settings || {};
      if (user.value && Array.isArray(resource?.statuses)) {
        user.value = { ...user.value, adapters: resource.statuses };
      }
      Object.assign(botSettings.form, {
        clawbot_enable: boolSetting(data["clawbot.enable"], true),
        clawbot_token: data["clawbot.token"] || "",
        clawbot_api_base:
          data["clawbot.api_base"] || "https://ilinkai.weixin.qq.com",
        clawbot_debug: boolSetting(data["clawbot.debug"]),
        qq_enable: boolSetting(data["qq.enable"], true),
        qq_token: data["qq.token"] || "",
        qq_debug: boolSetting(data["qq.debug"]),
        telegram_token: data["telegram.token"] || "",
        telegram_enable: boolSetting(data["telegram.enable"], true),
        telegram_api_base:
          data["telegram.api_base"] || "https://api.telegram.org",
        telegram_debug: boolSetting(data["telegram.debug"]),
        dingtalk_enable: boolSetting(data["dingtalk.enable"], true),
        dingtalk_client_id: data["dingtalk.client_id"] || "",
        dingtalk_client_secret: data["dingtalk.client_secret"] || "",
        dingtalk_debug: boolSetting(data["dingtalk.debug"]),
        qqguild_enable: boolSetting(data["qqguild.enable"], true),
        qqguild_mode:
          data["qqguild.mode"] === "websocket" ? "websocket" : "webhook",
        qqguild_app_id: data["qqguild.app_id"] || "",
        qqguild_app_secret: data["qqguild.app_secret"] || "",
        qqguild_sandbox: boolSetting(data["qqguild.sandbox"]),
        qqguild_debug: boolSetting(data["qqguild.debug"]),
        pagermaid_enable: boolSetting(data["pagermaid.enable"], true),
        pagermaid_token: data["pagermaid.token"] || "",
        pagermaid_debug: boolSetting(data["pagermaid.debug"]),
        web_chat_public: boolSetting(data["sillyGirl.web_chat_public"]),
      });
    } finally {
      botSettings.loading = false;
    }
  }

  async function refreshBots() {
    await loadBots();
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
        "clawbot.enable": !!v.clawbot_enable,
        "clawbot.token": v.clawbot_token || "",
        "clawbot.api_base":
          v.clawbot_api_base || "https://ilinkai.weixin.qq.com",
        "clawbot.debug": !!v.clawbot_debug,
        "qq.enable": !!v.qq_enable,
        "qq.token": v.qq_token || "",
        "qq.debug": !!v.qq_debug,
        "telegram.token": v.telegram_token || "",
        "telegram.enable": !!v.telegram_enable,
        "telegram.api_base": v.telegram_api_base || "https://api.telegram.org",
        "telegram.debug": !!v.telegram_debug,
        "dingtalk.enable": !!v.dingtalk_enable,
        "dingtalk.client_id": v.dingtalk_client_id || "",
        "dingtalk.client_secret": v.dingtalk_client_secret || "",
        "dingtalk.debug": !!v.dingtalk_debug,
        "qqguild.enable": !!v.qqguild_enable,
        "qqguild.mode":
          v.qqguild_mode === "websocket" ? "websocket" : "webhook",
        "qqguild.app_id": v.qqguild_app_id || "",
        "qqguild.app_secret": v.qqguild_app_secret || "",
        "qqguild.sandbox": !!v.qqguild_sandbox,
        "qqguild.debug": !!v.qqguild_debug,
        "pagermaid.enable": !!v.pagermaid_enable,
        "pagermaid.token": v.pagermaid_token || "",
        "pagermaid.debug": !!v.pagermaid_debug,
        "sillyGirl.web_chat_public": !!v.web_chat_public,
      });
      message.success("BOT 配置已保存");
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
    if (platform === "clawbot") return "clawbot_enable";
    if (platform === "qq") return "qq_enable";
    if (platform === "telegram") return "telegram_enable";
    if (platform === "dingtalk") return "dingtalk_enable";
    if (platform === "qqguild") return "qqguild_enable";
    if (platform === "pagermaid") return "pagermaid_enable";
    return "";
  }

  function botEnabled(row: { platform: string; enabled?: boolean }) {
    const key = botFormEnableKey(row.platform);
    if (key) return !!botSettings.form[key as keyof BotSettingsForm];
    return row.enabled !== false;
  }

  async function setBotEnabled(
    row: { platform: string; label: string; enabled?: boolean },
    enabled: boolean,
  ) {
    const key = botFormEnableKey(row.platform);
    if (!key) return;
    const previous = botSettings.form[key as keyof BotSettingsForm];
    (botSettings.form as Record<string, unknown>)[key] = enabled;
    try {
      await saveStorage({ [`${row.platform}.enable`]: enabled });
      message.success(`${row.label}${enabled ? "已开启" : "已关闭"}`);
      await refreshBots();
    } catch (error) {
      (botSettings.form as Record<string, unknown>)[key] = previous;
      message.error(
        error instanceof Error ? error.message : `${row.label}操作失败`,
      );
    }
  }

  type SettingsOptionKind = "github" | "pnpm" | "pipx";

  const builtinGithubProxyOptions = [
    "https://gh-proxy.org",
    "https://ghproxy.net",
    "https://cdn.gh-proxy.org",
    "http://jp-proxy.gitwarp.top:3000",
    "http://kr1-proxy.gitwarp.top:8081",
    "http://kr2-proxy.gitwarp.top:9980",
    "http://jp1-proxy.gitwarp.top:8123",
  ];
  const builtinPnpmRegistryOptions = [
    "https://registry.npmmirror.com",
    "https://registry.npmjs.org",
    "https://registry.npm.taobao.org",
    "https://mirrors.cloud.tencent.com/npm",
  ];
  const builtinPipxRegistryOptions = [
    "https://pypi.tuna.tsinghua.edu.cn/simple",
    "https://mirrors.aliyun.com/pypi/simple",
    "https://pypi.doubanio.com/simple",
    "https://mirrors.ustc.edu.cn/pypi/simple",
    "https://pypi.org/simple",
  ];

  const settings = reactive({
    form: {} as any,
    githubProxyOptions: [] as string[],
    pnpmRegistryOptions: [] as string[],
    pipxRegistryOptions: [] as string[],
    customGithubProxy: "",
    customPnpmRegistry: "",
    customPipxRegistry: "",
    optionSaving: {} as Record<SettingsOptionKind, boolean>,
  });
  const systemBackup = reactive({ downloading: false });
  const storageBackendOptions = [
    { label: "BoltDB", value: "boltdb" },
    { label: "Redis", value: "redis" },
  ];
  const announcementFormatOptions = [
    { label: "纯文本", value: "text" },
    { label: "Markdown", value: "markdown" },
    { label: "HTML", value: "html" },
  ];
  function uniqueSettingOptions(values: string[]) {
    return Array.from(
      new Set(values.map((item) => String(item || "").trim()).filter(Boolean)),
    );
  }

  const VNodes = defineComponent({
    props: ["vnodes"],
    setup(props) {
      return () => props.vnodes;
    },
  });

  function settingSelectOptions(values: string[]) {
    return uniqueSettingOptions(values).map((value) => ({
      value,
      label: value,
    }));
  }

  function settingOptionDeletable(kind: SettingsOptionKind) {
    const value = String(
      kind === "github"
        ? settings.form.github_proxy
        : kind === "pnpm"
          ? settings.form.pnpm_registry
          : settings.form.pipx_registry || "",
    ).trim();
    if (!value) return false;
    const builtins =
      kind === "github"
        ? builtinGithubProxyOptions
        : kind === "pnpm"
          ? builtinPnpmRegistryOptions
          : builtinPipxRegistryOptions;
    return !builtins.includes(value);
  }

  async function loadSettings() {
    const res = await get<
      ApiEnvelope<{
        values: Record<string, any>;
        github_proxy: { value: string; options: string[] };
        pnpm_registry: { value: string; options: string[] };
        pipx_registry: { value: string; options: string[] };
      }>
    >("/api/admin/settings");
    const resource = apiData(res);
    const data = resource?.values || {};
    const githubProxyData = resource?.github_proxy || {
      value: "",
      options: [],
    };
    const pnpmRegistryData = resource?.pnpm_registry || {
      value: builtinPnpmRegistryOptions[0],
      options: builtinPnpmRegistryOptions,
    };
    const pipxRegistryData = resource?.pipx_registry || {
      value: builtinPipxRegistryOptions[0],
      options: builtinPipxRegistryOptions,
    };
    settings.githubProxyOptions = uniqueSettingOptions(
      githubProxyData.options || [],
    );
    settings.pnpmRegistryOptions = uniqueSettingOptions(
      pnpmRegistryData.options || [
        pnpmRegistryData.value || builtinPnpmRegistryOptions[0],
      ],
    );
    settings.pipxRegistryOptions = uniqueSettingOptions(
      pipxRegistryData.options || [
        pipxRegistryData.value || builtinPipxRegistryOptions[0],
      ],
    );
    settings.form = {
      name: data["sillyGirl.name"],
      password: "",
      port: Number(data["sillyGirl.port"] || 8080),
      api_key: data["sillyGirl.api_key"],
      user_announcement_enable:
        data["sillyGirl.user_announcement_enable"] === true ||
        data["sillyGirl.user_announcement_enable"] === "true",
      user_announcement: data["sillyGirl.user_announcement"] || "",
      user_announcement_format: ["text", "markdown", "html"].includes(
        String(data["sillyGirl.user_announcement_format"] || ""),
      )
        ? data["sillyGirl.user_announcement_format"]
        : "text",
      debug:
        data["sillyGirl.debug"] === true || data["sillyGirl.debug"] === "true",
      listen_admin:
        data["sillyGirl.listen_admin"] !== false &&
        data["sillyGirl.listen_admin"] !== "false",
      recall: data["sillyGirl.recall"],
      storage: data["sillyGirl.storage"] === "redis" ? "redis" : "boltdb",
      redis_addr: data["sillyGirl.redis_addr"],
      redis_password: data["sillyGirl.redis_password"],
      github_proxy: githubProxyData.value || "",
      pnpm_registry: pnpmRegistryData.value || builtinPnpmRegistryOptions[0],
      pipx_registry: pipxRegistryData.value || builtinPipxRegistryOptions[0],
    };
  }
  async function saveSettings() {
    const v = settings.form;
    const updates: Record<string, unknown> = {
      "sillyGirl.name": v.name || "",
      "sillyGirl.port": v.port || 8080,
      "sillyGirl.api_key": v.api_key || "",
      "sillyGirl.user_announcement_enable": !!v.user_announcement_enable,
      "sillyGirl.user_announcement": v.user_announcement || "",
      "sillyGirl.user_announcement_format":
        v.user_announcement_format || "text",
      "sillyGirl.debug": !!v.debug,
      "sillyGirl.listen_admin": !!v.listen_admin,
      "sillyGirl.recall": v.recall || "",
      "sillyGirl.storage": v.storage || "boltdb",
      "sillyGirl.redis_addr": v.redis_addr || "",
      "sillyGirl.redis_password": v.redis_password || "",
    };
    if (v.password) updates["sillyGirl.password"] = v.password;
    const res = await post<ApiEnvelope<any>>("/api/admin/settings", {
      values: updates,
      github_proxy: String(v.github_proxy || "").trim(),
      pnpm_registry: String(v.pnpm_registry || "").trim(),
      pipx_registry: String(v.pipx_registry || "").trim(),
    });
    const data = apiData(res) || {};
    const firstError = Object.values(data.errors || {}).find(Boolean);
    if (firstError) throw new ApiError(200, String(firstError));
    settings.form.github_proxy = data.github_proxy?.value || "";
    settings.form.pnpm_registry =
      data.pnpm_registry?.value || settings.form.pnpm_registry;
    settings.form.pipx_registry =
      data.pipx_registry?.value || settings.form.pipx_registry;
    nodeDeps.pnpm.registry = settings.form.pnpm_registry;
    nodeDeps.pipx.registry = settings.form.pipx_registry;
    nodeDeps.registry =
      currentDependencyTool.value.registry || nodeDeps.registry;
    message.success("设置已保存");
    loadUser();
  }

  async function addSettingsOption(kind: SettingsOptionKind) {
    const draftKey =
      kind === "github"
        ? "customGithubProxy"
        : kind === "pnpm"
          ? "customPnpmRegistry"
          : "customPipxRegistry";
    const value = String(settings[draftKey] || "").trim();
    if (!value) {
      message.error("请输入地址");
      return;
    }
    settings.optionSaving[kind] = true;
    try {
      if (kind === "github") {
        const res = await post<
          ApiEnvelope<{ added?: string; options?: string[] }>
        >("/api/admin/plugin-market/github-proxy-options", { proxy: value });
        const data = apiData(res) || {};
        settings.githubProxyOptions = uniqueSettingOptions(
          data.options || [...settings.githubProxyOptions, data.added || value],
        );
        settings.form.github_proxy = data.added || value;
      } else if (kind === "pnpm") {
        const res = await post<
          ApiEnvelope<{ added?: string; options?: string[] }>
        >("/api/admin/dependency-registries/node/options", { registry: value });
        const data = apiData(res) || {};
        settings.pnpmRegistryOptions = uniqueSettingOptions(
          data.options || [
            ...settings.pnpmRegistryOptions,
            data.added || value,
          ],
        );
        settings.form.pnpm_registry = data.added || value;
      } else {
        const res = await post<
          ApiEnvelope<{ added?: string; options?: string[] }>
        >("/api/admin/dependency-registries/python/options", {
          registry: value,
        });
        const data = apiData(res) || {};
        settings.pipxRegistryOptions = uniqueSettingOptions(
          data.options || [
            ...settings.pipxRegistryOptions,
            data.added || value,
          ],
        );
        settings.form.pipx_registry = data.added || value;
      }
      settings[draftKey] = "";
      message.success("地址已添加");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "地址添加失败");
    } finally {
      settings.optionSaving[kind] = false;
    }
  }

  async function removeSettingsOption(kind: SettingsOptionKind) {
    const value = String(
      kind === "github"
        ? settings.form.github_proxy
        : kind === "pnpm"
          ? settings.form.pnpm_registry
          : settings.form.pipx_registry || "",
    ).trim();
    if (!value) {
      message.error("请选择要删除的地址");
      return;
    }
    settings.optionSaving[kind] = true;
    try {
      if (kind === "github") {
        const res = await post<
          ApiEnvelope<{ proxy?: string; options?: string[] }>
        >(
          `/api/admin/plugin-market/github-proxy-option-deletions/${encodeURIComponent(value)}`,
        );
        const data = apiData(res) || {};
        settings.githubProxyOptions = uniqueSettingOptions(
          data.options ||
            settings.githubProxyOptions.filter((item) => item !== value),
        );
        settings.form.github_proxy = data.proxy || "";
      } else if (kind === "pnpm") {
        const res = await post<
          ApiEnvelope<{ registry?: string; options?: string[] }>
        >(
          `/api/admin/dependency-registries/node/option-deletions/${encodeURIComponent(value)}`,
        );
        const data = apiData(res) || {};
        settings.pnpmRegistryOptions = uniqueSettingOptions(
          data.options ||
            settings.pnpmRegistryOptions.filter((item) => item !== value),
        );
        settings.form.pnpm_registry =
          data.registry || builtinPnpmRegistryOptions[0];
        nodeDeps.pnpm.registry = settings.form.pnpm_registry;
      } else {
        const res = await post<
          ApiEnvelope<{ registry?: string; options?: string[] }>
        >(
          `/api/admin/dependency-registries/python/option-deletions/${encodeURIComponent(value)}`,
        );
        const data = apiData(res) || {};
        settings.pipxRegistryOptions = uniqueSettingOptions(
          data.options ||
            settings.pipxRegistryOptions.filter((item) => item !== value),
        );
        settings.form.pipx_registry =
          data.registry || builtinPipxRegistryOptions[0];
        nodeDeps.pipx.registry = settings.form.pipx_registry;
      }
      message.success("地址已删除");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "地址删除失败");
    } finally {
      settings.optionSaving[kind] = false;
    }
  }

  function systemBackupFilename(contentDisposition: string) {
    const encoded = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1];
    if (encoded) {
      try {
        return decodeURIComponent(encoded.replace(/^"|"$/g, ""));
      } catch (_) {
        // Fall through to the plain filename value.
      }
    }
    const fallbackTime = new Date().toISOString().replace(/[:.]/g, "-");
    return (
      contentDisposition.match(/filename="?([^";]+)"?/i)?.[1] ||
      `sillygirl-backup-${fallbackTime}.zip`
    );
  }

  async function downloadSystemBackup() {
    systemBackup.downloading = true;
    try {
      const response = await fetch("/api/admin/system-backups/current", {
        headers: { token: getAuthToken() },
        cache: "no-store",
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(
          payload?.message || `备份下载失败（HTTP ${response.status}）`,
        );
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = systemBackupFilename(
        response.headers.get("content-disposition") || "",
      );
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      message.success("备份已生成并开始下载");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "备份下载失败");
    } finally {
      systemBackup.downloading = false;
    }
  }

  const {
    messageBuckets,
    msgState,
    loadMessages,
    openMessage,
    saveMessageRow,
    removeMessageRow,
  } = useMessageRulesAdmin();

  const messageToolOptions = [
    { label: "转发", value: "carry" },
    { label: "回复", value: "reply" },
    { label: "监听", value: "messages" },
  ] as { label: string; value: MessageToolKind }[];
  const messageToolHelpText = computed(() => {
    if (messageToolKind.value === "carry")
      return "选择平台、群号、工作机器人和处理脚本。";
    if (messageToolKind.value === "reply")
      return "按关键词或正则维护自动回复规则。";
    return "维护监听群组、禁言群组和屏蔽用户。";
  });
  const messageToolAddLabel = computed(() => {
    if (messageToolKind.value === "carry") return "新增转发群组";
    if (messageToolKind.value === "reply") return "新增回复";
    return "新增监听规则";
  });

  function loadActiveMessageTool() {
    if (messageToolKind.value === "carry") return loadCarry();
    if (messageToolKind.value === "reply") return loadReplies();
    return loadMessages();
  }

  function openActiveMessageTool() {
    if (messageToolKind.value === "carry") {
      openCarry();
      return;
    }
    if (messageToolKind.value === "reply") {
      openReply();
      return;
    }
    openMessage();
  }

  watch(
    [page, () => Boolean(user.value)],
    ([p, authenticated]) => {
      if (!authenticated) return;
      if (p === "bots") loadBots();
      if (p === "users") loadNormalUsers();
      if (p === "masters") loadMasters();
      if (p === "tasks") loadTasks();
      if (p === "message-tools") loadActiveMessageTool();
      if (p === "containers") loadActiveContainerPanels();
      if (p === "dependencies") loadNodeDependencies();
      if (p === "plugins") {
        loadPlugins(1, 16, true, true);
      }
      if (p === "storage") {
        loadStorage(1, storageState.pageSize, true);
      }
      if (p === "settings") loadSettings();
    },
    { immediate: true },
  );
  watch(containerKind, (kind) => {
    if (page.value !== "containers") return;
    window.history.replaceState({}, "", `/admin/containers/${kind}`);
  });
  watch(messageToolKind, (kind) => {
    if (page.value !== "message-tools") return;
    window.history.replaceState({}, "", `/admin/message-tools/${kind}`);
    loadActiveMessageTool();
  });
  watch(() => plugins.keyword, schedulePluginSearch);
  watch(
    () => plugins.tab,
    () => {
      cancelPluginSearch();
      loadPlugins();
    },
  );
  watch(
    () => plugins.klass,
    () => {
      cancelPluginSearch();
      loadPlugins();
    },
  );
  onBeforeUnmount(cancelPluginSearch);
  watch(
    () => nodeDeps.plugin,
    (plugin) => {
      if (page.value === "dependencies") loadNodeDependencies(plugin);
    },
  );
  watch(
    () => nodeDeps.runtime,
    () => {
      nodeDeps.plugin = "";
      nodeDeps.packageName = "";
      if (page.value === "dependencies") loadNodeDependencies("");
    },
  );
  watch(
    () => msgState.active,
    () => {
      if (
        page.value === "message-tools" &&
        messageToolKind.value === "messages"
      )
        loadMessages();
    },
  );

  function optionMap(values?: string[]) {
    return (values || []).map((value) => ({ value, label: value }));
  }
  function recordOptions(record?: Record<string, string>) {
    return Object.entries(record || {}).map(([value, label]) => ({
      value,
      label,
    }));
  }
  function smallcatOpenids(record?: AdminUserRow) {
    const rows = [] as string[];
    if (record?.bindings?.smallcat_openid)
      rows.push(record.bindings.smallcat_openid);
    for (const item of record?.bindings?.smallcat_openids || []) {
      if (item) rows.push(item);
    }
    return Array.from(new Set(rows.map((item) => item.trim()).filter(Boolean)));
  }

  return {
    VNodes,
    addPluginSource,
    addSettingsOption,
    announcementFormatOptions,
    booting,
    botEnabled,
    botSettings,
    botSettingsModal,
    botStatusRows,
    canRemoveStorageBucket,
    cancelCurrentBotSettings,
    cancelUninstallPluginModal,
    carry,
    changeCarryPlatform,
    changeStoragePage,
    clawbotLogin,
    closeClawbotLogin,
    closeMarketPluginEditor,
    confirmUninstallPlugin,
    containerAddLabel,
    containerHelpText,
    containerKind,
    containerOptions,
    createStorageBucket,
    createStorageEntry,
    currentDependencyTool,
    daidai,
    deleteMarketPluginEditor,
    dependencyPackagePlaceholder,
    dependencyPluginOptions,
    dependencyRegistryLabel,
    dependencyRuntimeLabel,
    dependencyRuntimeOptions,
    dependencySharedPath,
    downloadSystemBackup,
    fieldOptions,
    fieldType,
    filterPluginClassOption,
    formatMarketPluginEditor,
    handlePluginEditorOpenChange,
    installNodeDependency,
    installNodeDependencyRow,
    installPlugin,
    isPluginCronTask,
    loadActiveContainerPanels,
    refreshActiveContainerPanels,
    loadActiveMessageTool,
    loadCarry,
    loadMasters,
    loadNodeDependencies,
    loadNormalUsers,
    loadPlugins,
    loadReplies,
    loadStorage,
    loadTasks,
    login,
    loginModel,
    logout,
    masters,
    menuItems,
    messageBuckets,
    messageToolAddLabel,
    messageToolHelpText,
    messageToolKind,
    messageToolOptions,
    mobileMenuOpen,
    msgState,
    navigate,
    nodeDeps,
    normalUsers,
    normalUserPluginAuthorizations,
    oneBotReceiveURL,
    openActiveContainerPanel,
    openActiveMessageTool,
    openBotSettings,
    openCarry,
    openCreateStorageBucket,
    openDaidaiPanel,
    openMarketPluginConfig,
    openMarketPluginEditor,
    openMessage,
    openNewMarketPluginEditor,
    openNormalUser,
    openNormalUserPluginAuthorizations,
    openPluginDetail,
    openPluginSourceManager,
    openQinglongPanel,
    openReply,
    openSmallcatPanel,
    openTask,
    openUninstallPluginModal,
    optionMap,
    overviewIntegrations,
    overviewUserStats,
    overviewVersion,
    page,
    pagermaidBridgeURL,
    pluginCanManage,
    pluginClassOptions,
    pluginClassTags,
    pluginConfigFieldVisible,
    pluginConfigs,
    pluginDependencies,
    pluginEditor,
    pluginEditorHost,
    pluginHasSchedule,
    pluginIconIsImage,
    pluginInitial,
    pluginInstalled,
    pluginRuntimeEnabled,
    pluginOpenAvailable,
    pluginPanelChoices,
    pluginPanelEmptyText,
    pluginPanelKind,
    pluginSettingsCanSave,
    pluginStatusColor,
    pluginStatusLabel,
    pluginTriggerText,
    pluginUpgradable,
    plugins,
    pluginAuthorizations,
    qinglong,
    qqGuildWebhookURL,
    realScripts,
    recordOptions,
    refreshBots,
    removeCarry,
    removeDaidaiPanel,
    removeMaster,
    removeMessageRow,
    removeNodeDependency,
    removeNormalUser,
    removePluginSource,
    removeQinglongPanel,
    removeReply,
    removeSettingsOption,
    removeSmallcatPanel,
    removeStorageBucket,
    removeTask,
    replies,
    restartAfterUpdate,
    runTask,
    saveCarry,
    saveCurrentBotSettings,
    saveDaidaiPanel,
    saveMarketPluginEditor,
    saveMaster,
    saveMessageRow,
    saveNormalUser,
    saveNormalUserPluginAuthorization,
    savePluginConfig,
    saveQinglongPanel,
    saveReply,
    saveSettings,
    saveSmallcatPanel,
    saveStorageRow,
    saveTask,
    schemaFields,
    scripts,
    searchPluginsNow,
    selectStorageBucket,
    selectedStorageBucket,
    sendWebChat,
    setBotEnabled,
    settingOptionDeletable,
    settingSelectOptions,
    settings,
    setupAdmin,
    setupModel,
    setupRequired,
    showSmallcatOpenids,
    smallcat,
    smallcatOpenids,
    smallcatQuotaText,
    startClawbotLogin,
    startOnlineUpdate,
    storageBackendOptions,
    storageState,
    submitClawbotVerifyCode,
    syncPluginEditorLanguage,
    systemBackup,
    systemUpdate,
    tasks,
    testDaidaiPanel,
    testQinglongPanel,
    testSmallcatPanel,
    togglePluginEditorTheme,
    togglePluginStatus,
    toggleTaskEnabled,
    toggleWebChat,
    user,
    webChat,
    webChatEndpointURL,
    webChatMessagesEl,
  };
}
