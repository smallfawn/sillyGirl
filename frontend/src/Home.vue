<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import AntApp from 'ant-design-vue/es/app';
import Avatar from 'ant-design-vue/es/avatar';
import Button from 'ant-design-vue/es/button';
import Card from 'ant-design-vue/es/card';
import Col from 'ant-design-vue/es/col';
import ConfigProvider from 'ant-design-vue/es/config-provider';
import Form from 'ant-design-vue/es/form';
import Input from 'ant-design-vue/es/input';
import Row from 'ant-design-vue/es/row';
import Space from 'ant-design-vue/es/space';
import Tag from 'ant-design-vue/es/tag';
import Typography from 'ant-design-vue/es/typography';
import message from 'ant-design-vue/es/message';
import zhCN from 'ant-design-vue/es/locale/zh_CN';
import { Bell, MessageSquare, Plug, ShieldCheck, User } from 'lucide-vue-next';

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

type AuthPayload = {
  token: string;
  expiresIn: number;
  user: PublicUser;
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
};

const tokenKey = 'sillygirl_user_token';
const authMode = ref<'login' | 'register'>('login');
const loading = ref(false);
const currentUser = ref<PublicUser | null>(null);
const openPlugins = ref<OpenPlugin[]>([]);
const token = ref(localStorage.getItem(tokenKey) || '');
const loginForm = reactive({
  username: '',
  password: '',
});
const registerForm = reactive({
  username: '',
  nickname: '',
  password: '',
});

const userInitial = computed(() => {
  const name = currentUser.value?.nickname || currentUser.value?.username || 'U';
  return name.slice(0, 1).toUpperCase();
});

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

function setAuth(data: AuthPayload) {
  token.value = data.token;
  currentUser.value = data.user;
  localStorage.setItem(tokenKey, data.token);
}

function clearAuth() {
  token.value = '';
  currentUser.value = null;
  localStorage.removeItem(tokenKey);
}

async function login() {
  loading.value = true;
  try {
    const data = await requestJSON<AuthPayload>('/api/user/login', {
      method: 'POST',
      body: JSON.stringify({
        username: loginForm.username.trim(),
        password: loginForm.password,
      }),
    });
    setAuth(data);
    message.success('登录成功');
    window.location.href = '/user';
  } catch (error) {
    message.error(error instanceof Error ? error.message : '登录失败');
  } finally {
    loading.value = false;
  }
}

async function register() {
  loading.value = true;
  try {
    const data = await requestJSON<AuthPayload>('/api/user/register', {
      method: 'POST',
      body: JSON.stringify({
        username: registerForm.username.trim(),
        nickname: registerForm.nickname.trim(),
        password: registerForm.password,
      }),
    });
    setAuth(data);
    message.success('注册成功');
    window.location.href = '/user';
  } catch (error) {
    message.error(error instanceof Error ? error.message : '注册失败');
  } finally {
    loading.value = false;
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
    clearAuth();
    authMode.value = 'login';
    message.success('已退出登录');
  }
}

async function loadCurrentUser() {
  if (!token.value) {
    return;
  }
  try {
    const data = await requestJSON<{ user: PublicUser }>('/api/user/me');
    currentUser.value = data.user;
  } catch (_) {
    clearAuth();
  }
}

async function loadOpenPlugins() {
  try {
    openPlugins.value = await requestJSON<OpenPlugin[]>('/api/open/plugins');
  } catch (_) {
    openPlugins.value = [];
  }
}

onMounted(() => {
  loadCurrentUser();
  loadOpenPlugins();
});
</script>

<template>
  <ConfigProvider :locale="zhCN">
    <AntApp>
      <div class="home-page">
        <header class="home-topbar">
          <a class="home-brand" href="/">
            <span class="home-brand-mark" role="img" aria-label="傻妞 Logo"></span>
            <span>SillyGirl</span>
          </a>
          <Space wrap>
            <Button type="primary" href="#account">用户入口</Button>
          </Space>
        </header>

        <main class="home-content">
          <div class="home-layout">
            <section class="home-main">
              <Card class="home-hero" :bordered="false">
                <Space wrap>
                  <Tag color="blue">用户服务入口</Tag>
                  <Tag color="green">服务运行中</Tag>
                  <Tag>Vue 页面</Tag>
                </Space>
                <Typography.Title :level="1" class="home-title">SillyGirl 用户服务入口</Typography.Title>
                <Typography.Paragraph class="home-lead">
                  面向普通用户的统一入口。这里用于账号注册、登录和查看服务能力，后续可扩展个人资料、授权状态、积分记录和消息通知。
                </Typography.Paragraph>
                <Space wrap>
                  <Button type="primary" href="#account">登录或注册</Button>
                </Space>
              </Card>

              <Card class="home-panel" :bordered="false">
                <div class="home-toolbar">
                  <Space size="small">
                    <Plug :size="16" />
                    <Typography.Text strong>开放插件</Typography.Text>
                    <Typography.Text class="muted">由管理员开放的用户服务</Typography.Text>
                  </Space>
                  <Tag color="green">{{ openPlugins.length }} 个</Tag>
                </div>
                <div v-if="openPlugins.length" class="home-plugin-grid">
                  <article v-for="plugin in openPlugins" :key="plugin.id" class="home-plugin-card">
                    <div class="home-plugin-icon" aria-hidden="true">
                      <img v-if="pluginIconIsImage(plugin)" :src="plugin.icon" alt="" />
                      <span v-else>{{ pluginInitial(plugin) }}</span>
                    </div>
                    <div class="home-plugin-main">
                      <Typography.Text strong class="home-plugin-title">{{ plugin.title || plugin.id }}</Typography.Text>
                      <Typography.Paragraph class="home-plugin-desc">
                        {{ plugin.desc || '该插件暂未填写介绍。' }}
                      </Typography.Paragraph>
                      <Space wrap size="small">
                        <Tag v-if="plugin.version">{{ plugin.version }}</Tag>
                        <Tag v-for="klass in pluginClassTags(plugin)" :key="klass" color="cyan">{{ klass }}</Tag>
                        <Tag v-if="plugin.author" color="blue">{{ plugin.author }}</Tag>
                        <Tag v-if="plugin.dependencies?.length" color="green">依赖 {{ plugin.dependencies.join(' / ') }}</Tag>
                      </Space>
                    </div>
                  </article>
                </div>
                <Typography.Text v-else class="muted">暂时没有开放插件。</Typography.Text>
              </Card>

              <Card class="home-panel" :bordered="false">
                <div class="home-toolbar">
                  <Space size="small">
                    <MessageSquare :size="16" />
                    <Typography.Text strong>服务能力</Typography.Text>
                    <Typography.Text class="muted">面向用户的功能入口</Typography.Text>
                  </Space>
                </div>
                <Row :gutter="[12, 12]">
                  <Col :xs="24" :md="8">
                    <Card size="small">
                      <template #title><Space size="small"><Plug :size="16" />服务插件</Space></template>
                      <Typography.Paragraph class="muted mb0">通过插件提供不同业务能力，用户登录后可接入更多个人服务。</Typography.Paragraph>
                    </Card>
                  </Col>
                  <Col :xs="24" :md="8">
                    <Card size="small">
                      <template #title><Space size="small"><ShieldCheck :size="16" />账号体系</Space></template>
                      <Typography.Paragraph class="muted mb0">普通用户使用独立登录态，保护账号访问和个人服务数据。</Typography.Paragraph>
                    </Card>
                  </Col>
                  <Col :xs="24" :md="8">
                    <Card size="small">
                      <template #title><Space size="small"><Bell :size="16" />消息通知</Space></template>
                      <Typography.Paragraph class="muted mb0">支持通过机器人触达用户，后续可扩展通知、签到和个人提醒。</Typography.Paragraph>
                    </Card>
                  </Col>
                </Row>
              </Card>

              <Card class="home-panel" :bordered="false">
                <div class="home-toolbar">
                  <Typography.Text strong>使用流程</Typography.Text>
                </div>
                <div class="home-link-list">
                  <div class="home-link-row">
                    <span class="home-link-icon">1</span>
                    <span class="home-link-main">
                      <Typography.Text strong>创建普通用户账号</Typography.Text>
                      <Typography.Text class="muted">使用账号和密码注册，系统会自动保持登录状态。</Typography.Text>
                    </span>
                    <Tag>注册</Tag>
                  </div>
                  <div class="home-link-row">
                    <span class="home-link-icon">2</span>
                    <span class="home-link-main">
                      <Typography.Text strong>登录用户中心</Typography.Text>
                      <Typography.Text class="muted">登录后可查看个人账号信息和后续开放的用户功能。</Typography.Text>
                    </span>
                    <Tag color="blue">登录</Tag>
                  </div>
                  <div class="home-link-row">
                    <span class="home-link-icon">3</span>
                    <span class="home-link-main">
                      <Typography.Text strong>使用机器人服务</Typography.Text>
                      <Typography.Text class="muted">通过已接入的机器人和插件能力完成签到、通知、授权等操作。</Typography.Text>
                    </span>
                    <Tag color="green">服务</Tag>
                  </div>
                </div>
              </Card>
            </section>

            <aside id="account" class="home-auth">
              <Card :bordered="false">
                <template #title>
                  <Space direction="vertical" size="small">
                    <Typography.Text strong>普通用户</Typography.Text>
                    <Typography.Text class="muted">注册或登录用户账号</Typography.Text>
                  </Space>
                </template>

                <div v-if="currentUser" class="home-user-card">
                  <Space align="center">
                    <Avatar :size="48" class="home-avatar">{{ userInitial }}</Avatar>
                    <span>
                      <Typography.Text strong>{{ currentUser.nickname || currentUser.username }}</Typography.Text>
                      <Typography.Text class="muted block">@{{ currentUser.username }}</Typography.Text>
                    </span>
                  </Space>
                  <Button type="primary" block href="/user">进入用户中心</Button>
                  <Button block @click="logout">退出登录</Button>
                </div>

                <template v-else>
                  <div class="home-tabs">
                    <button type="button" class="home-tab" :class="{ active: authMode === 'login' }" @click="authMode = 'login'">登录</button>
                    <button type="button" class="home-tab" :class="{ active: authMode === 'register' }" @click="authMode = 'register'">注册</button>
                  </div>

                  <Form v-if="authMode === 'login'" layout="vertical" @finish="login">
                    <Form.Item label="账号" required>
                      <Input id="home-login-username" v-model:value="loginForm.username" name="username" autocomplete="username" aria-label="登录账号" placeholder="请输入账号">
                        <template #prefix><User :size="16" /></template>
                      </Input>
                    </Form.Item>
                    <Form.Item label="密码" required>
                      <Input.Password id="home-login-password" v-model:value="loginForm.password" name="password" autocomplete="current-password" aria-label="登录密码" placeholder="请输入密码" />
                    </Form.Item>
                    <Button type="primary" block :loading="loading" @click="login">登录</Button>
                  </Form>

                  <Form v-else layout="vertical" @finish="register">
                    <Form.Item label="账号" required>
                      <Input id="home-register-username" v-model:value="registerForm.username" name="username" autocomplete="username" aria-label="注册账号" placeholder="3-32 位字母、数字、下划线">
                        <template #prefix><User :size="16" /></template>
                      </Input>
                    </Form.Item>
                    <Form.Item label="昵称">
                      <Input id="home-register-nickname" v-model:value="registerForm.nickname" name="name" autocomplete="name" aria-label="注册昵称" placeholder="不填则使用账号" />
                    </Form.Item>
                    <Form.Item label="密码" required>
                      <Input.Password id="home-register-password" v-model:value="registerForm.password" name="new-password" autocomplete="new-password" aria-label="注册密码" placeholder="至少 6 位" />
                    </Form.Item>
                    <Button type="primary" block :loading="loading" @click="register">创建账号</Button>
                  </Form>
                </template>

                <Typography.Paragraph class="home-auth-tip muted">
                  账号仅用于普通用户服务入口，登录后会自动保持会话。
                </Typography.Paragraph>
              </Card>
            </aside>
          </div>
        </main>
      </div>
    </AntApp>
  </ConfigProvider>
</template>

<style scoped>
.home-page {
  min-height: 100vh;
  background: #f5f7fb;
}

.home-topbar {
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

.home-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #1f2937;
  font-weight: 700;
  letter-spacing: 0;
}

.home-brand-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #ffffff url("../logo.png") center 45% / 150% auto no-repeat;
  box-shadow: 0 1px 4px rgb(148 81 48 / 18%);
}

.home-content {
  width: min(1180px, calc(100% - 32px));
  margin: 0 auto;
  padding: 18px 0 36px;
}

.home-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 380px;
  gap: 18px;
  align-items: start;
}

.home-main {
  min-width: 0;
}

.home-hero,
.home-panel,
.home-auth :deep(.ant-card) {
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.home-hero {
  min-height: 256px;
  display: grid;
  align-content: center;
}

.home-hero :deep(.ant-card-body) {
  display: grid;
  gap: 14px;
}

.home-title {
  max-width: 760px;
  margin: 0 !important;
  font-size: clamp(30px, 5vw, 48px) !important;
  line-height: 1.12 !important;
  letter-spacing: 0 !important;
}

.home-lead {
  max-width: 760px;
  color: #6b7280;
  font-size: 16px;
  line-height: 1.75;
}

.home-panel {
  margin-top: 18px;
}

.home-plugin-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.home-plugin-card {
  display: grid;
  grid-template-columns: 48px minmax(0, 1fr);
  gap: 12px;
  padding: 14px;
  background: #f8fafc;
  border: 1px solid #e6eaf0;
  border-radius: 8px;
}

.home-plugin-icon {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  overflow: hidden;
  color: #166534;
  background: #dcfce7;
  border: 1px solid #86efac;
  border-radius: 10px;
  font-size: 20px;
  font-weight: 700;
}

.home-plugin-icon img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.home-plugin-main {
  min-width: 0;
}

.home-plugin-title {
  display: block;
  margin-bottom: 5px;
  font-size: 15px;
}

.home-plugin-desc {
  display: -webkit-box;
  margin-bottom: 10px !important;
  overflow: hidden;
  color: #667085;
  line-height: 1.6;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.home-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.home-link-list {
  display: grid;
  gap: 8px;
}

.home-link-row {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  min-height: 54px;
  padding: 9px 10px;
  color: #1f2937;
  background: #f8fafc;
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.home-link-row:hover {
  color: #1677ff;
  border-color: #91caff;
}

.home-link-icon {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  color: #1677ff;
  background: #e6f4ff;
  border-radius: 8px;
  font-weight: 700;
}

.home-link-main {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.home-auth {
  position: sticky;
  top: 74px;
}

.home-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  padding: 4px;
  margin-bottom: 16px;
  background: #f5f7fb;
  border: 1px solid #edf0f5;
  border-radius: 8px;
}

.home-tab {
  min-height: 34px;
  border: 0;
  border-radius: 6px;
  color: #6b7280;
  background: transparent;
  cursor: pointer;
}

.home-tab.active {
  color: #1f2937;
  background: #ffffff;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.06);
  font-weight: 700;
}

.home-user-card {
  display: grid;
  gap: 16px;
}

.home-avatar {
  background: #111827;
}

.muted {
  color: #6b7280;
}

.block {
  display: block;
}

.mb0 {
  margin-bottom: 0;
}

@media (max-width: 920px) {
  .home-layout {
    grid-template-columns: 1fr;
  }

  .home-auth {
    position: static;
  }
}

@media (max-width: 560px) {
  .home-topbar {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
    padding: 12px 16px;
  }

  .home-topbar :deep(.ant-space) {
    width: 100%;
  }

  .home-topbar :deep(.ant-space-item) {
    flex: 1;
  }

  .home-topbar :deep(.ant-btn) {
    width: 100%;
  }

  .home-content {
    width: min(100% - 24px, 1180px);
    padding-top: 12px;
  }

  .home-hero {
    min-height: auto;
  }

  .home-plugin-grid {
    grid-template-columns: 1fr;
  }

  .home-link-row {
    grid-template-columns: 34px minmax(0, 1fr);
  }

  .home-link-row :deep(.ant-tag) {
    grid-column: 2;
    width: max-content;
  }
}
</style>
