<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import Alert from 'ant-design-vue/es/alert';
import AntApp from 'ant-design-vue/es/app';
import Avatar from 'ant-design-vue/es/avatar';
import Button from 'ant-design-vue/es/button';
import Card from 'ant-design-vue/es/card';
import Col from 'ant-design-vue/es/col';
import ConfigProvider from 'ant-design-vue/es/config-provider';
import Empty from 'ant-design-vue/es/empty';
import Form from 'ant-design-vue/es/form';
import Input from 'ant-design-vue/es/input';
import Modal from 'ant-design-vue/es/modal';
import Row from 'ant-design-vue/es/row';
import Select from 'ant-design-vue/es/select';
import Space from 'ant-design-vue/es/space';
import Switch from 'ant-design-vue/es/switch';
import Tag from 'ant-design-vue/es/tag';
import Typography from 'ant-design-vue/es/typography';
import message from 'ant-design-vue/es/message';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import { Link, LogOut, Plug, QrCode, ShieldCheck } from 'lucide-vue-next';

type ApiEnvelope<T> = {
  status: boolean;
  message: string;
  data: T;
};

type PublicUser = {
  id: string;
  username: string;
  nickname: string;
  created_at: number;
};

type Bindings = {
  qq?: string;
  telegram?: string;
  smallcat_openid?: string;
  smallcat_openids?: string[];
  updated_at?: number;
};

type SmallcatPanel = {
  index: number;
  id: string;
  name: string;
  status: string;
  message: string;
};

type OpenPlugin = {
  id: string;
  title: string;
  desc?: string;
  icon?: string;
  version?: string;
  author?: string;
  class?: string;
  rule?: string;
  dependencies?: string[];
  authorized?: boolean;
  authorization_scope?: string;
  smallcat_account_count?: number;
};

const tokenKey = 'sillygirl_user_token';
const token = ref(localStorage.getItem(tokenKey) || '');
const loading = ref(true);
const user = ref<PublicUser | null>(null);
const bindings = reactive<Bindings>({});
const panels = ref<SmallcatPanel[]>([]);
const openPlugins = ref<OpenPlugin[]>([]);
const authorizingPluginIDs = ref<Set<string>>(new Set());
const selectedPanel = ref(1);
const bindForm = reactive({
  qq: '',
  telegram: '',
});
const smallcat = reactive({
  qrType: 1,
  uuid: '',
  qrOpen: false,
  qrLoading: false,
  confirmLoading: false,
  qrResult: null as unknown,
});

const userInitial = computed(() => {
  const name = user.value?.nickname || user.value?.username || 'U';
  return name.slice(0, 1).toUpperCase();
});

const panelOptions = computed(() => panels.value.map((item) => ({
  value: item.index,
  label: `编号 ${item.index}`,
})));
const qrTypeOptions = [
  { value: 1, label: '应用宝' },
  { value: 2, label: '手游助手' },
];

const selectedPanelText = computed(() => `编号 ${selectedPanel.value || 1}`);
const smallcatOpenids = computed(() => normalizeOpenids(bindings));

function pluginIconIsImage(plugin: OpenPlugin) {
  const icon = String(plugin.icon || '').trim();
  return /^https?:\/\//i.test(icon) || icon.startsWith('/') || icon.startsWith('data:image/');
}

function pluginInitial(plugin: OpenPlugin) {
  const text = String(plugin.title || plugin.id || 'P').trim();
  return (text ? text.slice(0, 1) : 'P').toUpperCase();
}

function pluginClassTags(plugin: OpenPlugin) {
  return String(plugin.class || '')
    .split(/[,，\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

const qrImage = computed(() => {
  const value = findValueByKey(smallcat.qrResult, [
    'qrcodeUrl',
    'qrCodeDataUrl',
    'qrcode',
    'qrCode',
    'qr_code',
    'image',
    'url',
    'qrUrl',
    'qr_url',
  ]);
  if (typeof value !== 'string') return '';
  if (/^(https?:\/\/|data:image\/)/i.test(value)) return value;
  return '';
});

async function requestJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }
  if (token.value && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token.value}`);
  }
  const res = await fetch(url, {
    credentials: 'include',
    ...options,
    headers,
  });
  const payload = (await res.json().catch(() => ({
    status: false,
    message: '服务响应异常',
    data: null,
  }))) as ApiEnvelope<T>;
  if (!res.ok || payload.status === false) {
    throw new Error(payload.message || '请求失败');
  }
  return payload.data;
}

function fillProfile(data: { user: PublicUser; bindings: Bindings; smallcat_panels: SmallcatPanel[] }) {
  user.value = data.user;
  Object.assign(bindings, data.bindings || {});
  bindForm.qq = bindings.qq || '';
  bindForm.telegram = bindings.telegram || '';
  panels.value = data.smallcat_panels || [];
  selectedPanel.value = panels.value[0]?.index || 1;
}

async function loadProfile() {
  loading.value = true;
  try {
    const data = await requestJSON<{ user: PublicUser; bindings: Bindings; smallcat_panels: SmallcatPanel[] }>('/api/user/profile');
    fillProfile(data);
  } catch (error) {
    localStorage.removeItem(tokenKey);
    token.value = '';
    user.value = null;
    message.error(error instanceof Error ? error.message : '请先登录');
  } finally {
    loading.value = false;
  }
}

async function loadOpenPlugins() {
  try {
    openPlugins.value = await requestJSON<OpenPlugin[]>('/api/user/plugins');
  } catch (_) {
    openPlugins.value = [];
  }
}

function pluginAuthorizationLoading(uuid: string) {
  return authorizingPluginIDs.value.has(uuid);
}

async function togglePluginAuthorization(plugin: OpenPlugin, authorized: boolean) {
  const previous = !!plugin.authorized;
  plugin.authorized = authorized;
  authorizingPluginIDs.value = new Set([...authorizingPluginIDs.value, plugin.id]);
  try {
    await requestJSON('/api/user/plugin/authorization', {
      method: 'PUT',
      body: JSON.stringify({ uuid: plugin.id, authorized }),
    });
    message.success(authorized
      ? `已允许「${plugin.title || plugin.id}」读取你的 smallcat 账号`
      : `已取消「${plugin.title || plugin.id}」的 smallcat 读取授权`);
  } catch (error) {
    plugin.authorized = previous;
    message.error(error instanceof Error ? error.message : '插件授权保存失败');
  } finally {
    const next = new Set(authorizingPluginIDs.value);
    next.delete(plugin.id);
    authorizingPluginIDs.value = next;
  }
}

async function logout() {
  try {
    await requestJSON<null>('/api/user/outlogin', {
      method: 'POST',
      body: '{}',
    });
  } catch (_) {
  } finally {
    localStorage.removeItem(tokenKey);
    window.location.href = '/';
  }
}

async function saveBinding(platform: 'qq' | 'telegram') {
  const value = platform === 'qq' ? bindForm.qq : bindForm.telegram;
  const data = await requestJSON<Bindings>('/api/user/bind', {
    method: 'PUT',
    body: JSON.stringify({ platform, value }),
  });
  Object.assign(bindings, data);
  message.success('绑定已保存');
}

async function removeBinding(platform: 'qq' | 'telegram') {
  const data = await requestJSON<Bindings>('/api/user/bind', {
    method: 'DELETE',
    body: JSON.stringify({ platform }),
  });
  Object.assign(bindings, data);
  if (platform === 'qq') bindForm.qq = '';
  if (platform === 'telegram') bindForm.telegram = '';
  message.success('绑定已解除');
}

async function openSmallcatLogin() {
  if (!panels.value.length) {
    message.error('后台还没有绑定 smallcat');
    return;
  }
  smallcat.qrOpen = true;
  smallcat.qrResult = null;
  smallcat.uuid = '';
  smallcat.qrLoading = true;
  try {
    const data = await requestJSON<unknown>('/api/user/smallcat/qr/start', {
      method: 'POST',
      body: JSON.stringify({ panel: selectedPanel.value, type: smallcat.qrType }),
    });
    smallcat.qrResult = data;
    const uuid = findValueByKey(data, ['uuid', 'qrUuid', 'qr_uuid']);
    if (typeof uuid === 'string') smallcat.uuid = uuid;
    if (!smallcat.uuid) {
      message.warning('二维码已生成，但未识别到 uuid');
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '生成二维码失败');
    smallcat.qrOpen = false;
  } finally {
    smallcat.qrLoading = false;
  }
}

async function confirmSmallcatLogin() {
  if (!smallcat.uuid.trim()) {
    message.error('缺少二维码 uuid，请重新生成二维码');
    return;
  }
  smallcat.confirmLoading = true;
  try {
    const data = await requestJSON<{ openid: string; bindings: Bindings }>('/api/user/smallcat/login/confirm', {
      method: 'POST',
      body: JSON.stringify({ panel: selectedPanel.value, uuid: smallcat.uuid.trim() }),
    });
    Object.assign(bindings, data.bindings || { smallcat_openid: data.openid, smallcat_openids: data.openid ? [data.openid] : [] });
    smallcat.qrOpen = false;
    message.success('smallcat 登录成功');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '未检测到 smallcat 登录');
  } finally {
    smallcat.confirmLoading = false;
  }
}

function findValueByKey(value: unknown, keys: string[]): unknown {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  for (const key of keys) {
    if (record[key] !== undefined && record[key] !== null && record[key] !== '') {
      return record[key];
    }
  }
  for (const item of Object.values(record)) {
    const found = findValueByKey(item, keys);
    if (found !== undefined && found !== null && found !== '') return found;
  }
  return undefined;
}

function normalizeOpenids(value: Bindings) {
  const rows = [] as string[];
  if (value.smallcat_openid) rows.push(value.smallcat_openid);
  for (const item of value.smallcat_openids || []) {
    if (item) rows.push(item);
  }
  return Array.from(new Set(rows.map((item) => item.trim()).filter(Boolean)));
}

onMounted(() => {
  loadProfile();
  loadOpenPlugins();
});
</script>

<template>
  <ConfigProvider :locale="zhCN">
    <AntApp>
      <div class="user-page">
        <header class="user-topbar">
          <a class="user-brand" href="/">
            <span class="user-brand-mark" role="img" aria-label="傻妞 Logo"></span>
            <span>SillyGirl</span>
          </a>
          <Space v-if="user" align="center">
            <Avatar :size="34" class="user-avatar">{{ userInitial }}</Avatar>
            <span class="user-name">{{ user.nickname || user.username }}</span>
            <Button @click="logout"><template #icon><LogOut :size="16" /></template>退出</Button>
          </Space>
        </header>

        <main class="user-content">
          <Card v-if="!loading && !user" class="user-login-card" :bordered="false">
            <Empty description="请先登录普通用户账号" />
            <Button type="primary" href="/">返回登录</Button>
          </Card>

          <template v-if="user">
            <section class="user-summary">
              <Card :bordered="false">
                <Space align="center">
                  <Avatar :size="56" class="user-avatar">{{ userInitial }}</Avatar>
                  <span>
                    <Typography.Title :level="3" class="user-title">{{ user.nickname || user.username }}</Typography.Title>
                    <Typography.Text class="muted">@{{ user.username }}</Typography.Text>
                  </span>
                </Space>
              </Card>
              <Card :bordered="false">
                <Space direction="vertical" size="small">
                  <Typography.Text strong>绑定状态</Typography.Text>
                  <Space wrap>
                    <Tag :color="bindings.qq ? 'green' : 'default'">QQ {{ bindings.qq || '未绑定' }}</Tag>
                    <Tag :color="bindings.telegram ? 'green' : 'default'">TG {{ bindings.telegram || '未绑定' }}</Tag>
                    <Tag :color="smallcatOpenids.length ? 'green' : 'default'">smallcat {{ smallcatOpenids.length ? `${smallcatOpenids.length} 个账号` : '未登录' }}</Tag>
                  </Space>
                </Space>
              </Card>
            </section>

            <Card class="user-panel user-plugin-panel" :bordered="false">
              <div class="user-plugin-toolbar">
                <Space size="small">
                  <Plug :size="18" />
                  <Typography.Text strong>开放插件</Typography.Text>
                  <Typography.Text class="muted">管理员已开放给普通用户的插件</Typography.Text>
                </Space>
                <Tag color="green">{{ openPlugins.length }} 个</Tag>
              </div>
              <div class="user-plugin-auth-tip">
                <ShieldCheck :size="16" />
                授权开关只控制插件读取你绑定的 smallcat 账号；获取 code 等后续操作默认同意，不再重复询问。
              </div>
              <div v-if="openPlugins.length" class="user-plugin-grid">
                <article v-for="plugin in openPlugins" :key="plugin.id" class="user-plugin-card">
                  <div class="user-plugin-icon" aria-hidden="true">
                    <img v-if="pluginIconIsImage(plugin)" :src="plugin.icon" alt="" />
                    <span v-else>{{ pluginInitial(plugin) }}</span>
                  </div>
                  <div class="user-plugin-main">
                    <Typography.Text strong class="user-plugin-title">{{ plugin.title || plugin.id }}</Typography.Text>
                    <Typography.Paragraph class="user-plugin-desc">
                      {{ plugin.desc || '该插件暂未填写介绍。' }}
                    </Typography.Paragraph>
                    <Space wrap size="small">
                      <Tag v-if="plugin.version" color="blue">{{ plugin.version }}</Tag>
                      <Tag v-for="item in pluginClassTags(plugin)" :key="item">{{ item }}</Tag>
                      <Tag v-if="plugin.author">{{ plugin.author }}</Tag>
                      <Tag v-if="plugin.rule" color="green">{{ plugin.rule }}</Tag>
                    </Space>
                    <div class="user-plugin-authorization">
                      <span class="user-plugin-scope">
                        <ShieldCheck :size="15" />
                        仅授权读取你绑定的 smallcat 账号
                      </span>
                      <Switch
                        :checked="!!plugin.authorized"
                        :loading="pluginAuthorizationLoading(plugin.id)"
                        checked-children="已授权"
                        un-checked-children="未授权"
                        @change="(checked: boolean) => togglePluginAuthorization(plugin, checked)"
                      />
                    </div>
                  </div>
                </article>
              </div>
              <Empty v-else image="simple" description="暂时没有开放插件" />
            </Card>

            <Row :gutter="[16, 16]">
              <Col :xs="24" :lg="15">
                <Card class="user-panel" :bordered="false">
                  <template #title>
                    <Space><QrCode :size="18" />smallcat 账号</Space>
                  </template>

                  <Alert
                    v-if="!panels.length"
                    type="warning"
                    show-icon
                    message="后台还没有绑定 smallcat"
                    description="请管理员先在后台添加 smallcat 面板后，普通用户才能添加 smallcat 账号。"
                  />

                  <div v-else class="smallcat-account">
                    <div class="smallcat-status">
                      <Typography.Text strong>当前 smallcat openid</Typography.Text>
                      <Space v-if="smallcatOpenids.length" direction="vertical" size="small">
                        <Typography.Text v-for="openid in smallcatOpenids" :key="openid" class="mono">{{ openid }}</Typography.Text>
                      </Space>
                      <Typography.Text v-else class="muted">未登录</Typography.Text>
                    </div>
                    <Form layout="vertical" class="smallcat-form">
                      <Form.Item label="smallcat 面板">
                        <Select v-model:value="selectedPanel" :options="panelOptions" />
                      </Form.Item>
                      <Form.Item label="二维码类型">
                        <Select v-model:value="smallcat.qrType" :options="qrTypeOptions" />
                      </Form.Item>
                      <Button type="primary" size="large" @click="openSmallcatLogin">
                        添加 smallcat 账号
                      </Button>
                    </Form>
                  </div>
                </Card>
              </Col>

              <Col :xs="24" :lg="9">
                <Card class="user-panel" :bordered="false">
                  <template #title>
                    <Space><Link :size="18" />账号绑定</Space>
                  </template>
                  <Form layout="vertical">
                    <template v-if="bindings.qq">
                      <Form.Item label="QQ 号">
                        <Space class="bound-row">
                          <Typography.Text class="mono">{{ bindings.qq }}</Typography.Text>
                          <Button @click="removeBinding('qq')">解绑</Button>
                        </Space>
                      </Form.Item>
                    </template>
                    <template v-else>
                      <Form.Item label="QQ 号">
                        <Input v-model:value="bindForm.qq" placeholder="例如：860562056" />
                      </Form.Item>
                      <Space class="bind-actions">
                        <Button type="primary" @click="saveBinding('qq')">绑定 QQ</Button>
                      </Space>
                    </template>

                    <template v-if="bindings.telegram">
                      <Form.Item label="Telegram ID" class="bind-field">
                        <Space class="bound-row">
                          <Typography.Text class="mono">{{ bindings.telegram }}</Typography.Text>
                          <Button @click="removeBinding('telegram')">解绑</Button>
                        </Space>
                      </Form.Item>
                    </template>
                    <template v-else>
                      <Form.Item label="Telegram ID" class="bind-field">
                        <Input v-model:value="bindForm.telegram" placeholder="例如：123456789" />
                      </Form.Item>
                      <Space class="bind-actions">
                        <Button type="primary" @click="saveBinding('telegram')">绑定 TG</Button>
                      </Space>
                    </template>
                  </Form>
                </Card>
              </Col>
            </Row>
          </template>
        </main>

        <Modal
          v-model:open="smallcat.qrOpen"
          title="添加 smallcat 账号"
          ok-text="确认登录"
          cancel-text="取消"
          :confirm-loading="smallcat.confirmLoading"
          @ok="confirmSmallcatLogin"
        >
          <Space direction="vertical" size="middle" class="qr-modal">
            <Alert type="info" show-icon :message="`请在 2 分钟内使用 ${selectedPanelText} 扫码登录，完成后点击确认登录。`" />
            <div class="qr-box">
              <span v-if="smallcat.qrLoading" class="muted">二维码生成中...</span>
              <img v-else-if="qrImage" :src="qrImage" alt="smallcat 二维码" />
              <span v-else class="muted">未识别到二维码图片，请检查 smallcat 返回。</span>
            </div>
            <Typography.Text v-if="smallcat.uuid" class="mono">UUID: {{ smallcat.uuid }}</Typography.Text>
          </Space>
        </Modal>
      </div>
    </AntApp>
  </ConfigProvider>
</template>

<style scoped>
.user-page {
  min-height: 100vh;
  background: #f5f7fb;
}

.user-topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 56px;
  padding: 0 20px;
  background: #ffffff;
  border-bottom: 1px solid #edf0f5;
}

.user-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #1f2937;
  font-weight: 700;
}

.user-brand-mark,
.user-avatar {
  background: #111827;
  color: #ffffff;
}

.user-brand-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #ffffff url("../logo.png") center 45% / 150% auto no-repeat;
  box-shadow: 0 1px 4px rgb(148 81 48 / 18%);
}

.user-content {
  width: min(1180px, calc(100% - 32px));
  margin: 0 auto;
  padding: 18px 0 36px;
}

.user-summary {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 420px;
  gap: 16px;
  margin-bottom: 16px;
}

.user-panel,
.user-summary :deep(.ant-card),
.user-login-card {
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.user-plugin-panel {
  margin-bottom: 16px;
}

.user-plugin-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.user-plugin-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.user-plugin-auth-tip {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 10px 12px;
  margin: -2px 0 14px;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  background: #f0fdf4;
  color: #166534;
  font-size: 13px;
}

.user-plugin-card {
  display: flex;
  gap: 14px;
  min-width: 0;
  padding: 16px;
  border: 1px solid #e7ebf0;
  border-radius: 10px;
  background: #ffffff;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.user-plugin-card:hover {
  border-color: #86d5ad;
  box-shadow: 0 6px 18px rgb(22 163 74 / 9%);
}

.user-plugin-icon {
  display: grid;
  flex: 0 0 46px;
  place-items: center;
  width: 46px;
  height: 46px;
  overflow: hidden;
  border-radius: 12px;
  background: #ecfdf3;
  color: #15803d;
  font-size: 20px;
  font-weight: 700;
}

.user-plugin-icon img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.user-plugin-main {
  min-width: 0;
  flex: 1;
}

.user-plugin-title {
  display: block;
  margin-bottom: 5px;
  font-size: 16px;
}

.user-plugin-desc {
  min-height: 44px;
  margin-bottom: 10px !important;
  color: #64748b;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.user-plugin-authorization {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 12px;
  margin-top: 12px;
  border-top: 1px solid #eef2f6;
}

.user-plugin-scope {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #64748b;
  font-size: 12px;
}

.user-login-card {
  max-width: 420px;
  margin: 80px auto 0;
  text-align: center;
}

.user-title {
  margin: 0 !important;
}

.user-name {
  font-weight: 700;
}

.smallcat-account {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 280px;
  gap: 16px;
  align-items: start;
}

.smallcat-status {
  display: grid;
  gap: 10px;
  min-height: 136px;
  padding: 16px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #f8fafc;
}

.smallcat-form {
  padding: 16px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #ffffff;
}

.qr-modal {
  width: 100%;
}

.qr-box {
  display: grid;
  place-items: center;
  min-height: 260px;
  padding: 16px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #ffffff;
}

.qr-box img {
  max-width: 240px;
  width: 100%;
  height: auto;
}

.bind-actions {
  margin-bottom: 14px;
}

.bound-row {
  width: 100%;
  justify-content: space-between;
}

.bind-field {
  margin-top: 12px;
}

.muted {
  color: #6b7280;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  word-break: break-all;
}

.mb0 {
  margin-bottom: 0;
}

@media (max-width: 920px) {
  .user-summary,
  .smallcat-account,
  .user-plugin-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .user-topbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
    padding: 12px 16px;
  }

  .user-content {
    width: min(100% - 24px, 1180px);
    padding-top: 12px;
  }

  .user-plugin-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
