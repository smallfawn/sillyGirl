<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import Button from "ant-design-vue/es/button";
import Card from "ant-design-vue/es/card";
import { Download, Plus, Save } from "lucide-vue-next";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import InputNumber from "ant-design-vue/es/input-number";
import Select from "ant-design-vue/es/select";
import Space from "ant-design-vue/es/space";
import Switch from "ant-design-vue/es/switch";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import type { Directive } from "vue";
import { useAdminViewContext } from "../adminViewContext";

const selectA11yObservers = new WeakMap<HTMLElement, MutationObserver>();

function patchSelectComboboxA11y(element: HTMLElement) {
  const input = element.querySelector<HTMLInputElement>('input[role="combobox"]');
  if (!input) return;
  const expanded = element.classList.contains("ant-select-open");
  const expandedValue = String(expanded);
  if (input.getAttribute("aria-expanded") !== expandedValue) {
    input.setAttribute("aria-expanded", expandedValue);
  }
  if (!expanded) {
    for (const name of ["aria-owns", "aria-controls", "aria-activedescendant"]) {
      if (input.hasAttribute(name)) input.removeAttribute(name);
    }
  }
}

const vSelectA11y: Directive<HTMLElement> = {
  mounted(element) {
    patchSelectComboboxA11y(element);
    const observer = new MutationObserver(() => patchSelectComboboxA11y(element));
    observer.observe(element, { attributes: true, childList: true, subtree: true });
    selectA11yObservers.set(element, observer);
  },
  updated: patchSelectComboboxA11y,
  beforeUnmount(element) {
    selectA11yObservers.get(element)?.disconnect();
    selectA11yObservers.delete(element);
  },
};

const {
  VNodes,
  addSettingsOption,
  announcementFormatOptions,
  downloadSystemBackup,
  page,
  removeSettingsOption,
  saveSettings,
  settingOptionDeletable,
  settingSelectOptions,
  settings,
  storageBackendOptions,
  systemBackup,
  user,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'settings'" class="panel">
    <Form layout="vertical" style="max-width: 860px">
      <Form.Item label="后台账号名" html-for="settings-name"
        ><Input
          id="settings-name"
          name="settings-name"
          v-model:value="settings.form.name"
      /></Form.Item>
      <Form.Item label="修改密码" html-for="settings-password"
        ><Input.Password
          id="settings-password"
          name="settings-password"
          autocomplete="new-password"
          v-model:value="settings.form.password"
          placeholder="留空表示不修改"
      /></Form.Item>
      <Form.Item label="HTTP 端口" html-for="settings-port"
        ><InputNumber
          id="settings-port"
          v-model:value="settings.form.port"
          style="width: 100%"
          :min="1"
          :max="65535"
      /></Form.Item>
      <Form.Item label="API Key" html-for="settings-api-key"
        ><Input
          id="settings-api-key"
          name="settings-api-key"
          v-model:value="settings.form.api_key"
      /></Form.Item>
      <Typography.Title :level="5">普通用户公告</Typography.Title>
      <Form.Item
        label="在 user 页面显示公告"
        html-for="settings-announcement-enabled"
      >
        <Switch
          id="settings-announcement-enabled"
          v-model:checked="settings.form.user_announcement_enable"
          aria-label="在 user 页面显示公告"
        />
      </Form.Item>
      <Form.Item label="公告格式" html-for="settings-announcement-format">
        <Select
          v-select-a11y
          id="settings-announcement-format"
          v-model:value="settings.form.user_announcement_format"
          :options="announcementFormatOptions"
        />
      </Form.Item>
      <Form.Item
        label="公告内容"
        html-for="settings-announcement-content"
        extra="开启后会显示在普通用户 /user 页面顶部；纯文本保留换行，Markdown 支持常用标题/列表/链接/加粗/代码，HTML 会按原样渲染。"
      >
        <Input.TextArea
          id="settings-announcement-content"
          name="settings-announcement-content"
          v-model:value="settings.form.user_announcement"
          :rows="6"
          placeholder="请输入要展示给普通用户的公告，支持纯文本 / Markdown / HTML"
        />
      </Form.Item>
      <Form.Item label="自动撤回正则" html-for="settings-recall"
        ><Input.TextArea
          id="settings-recall"
          name="settings-recall"
          v-model:value="settings.form.recall"
          :rows="2"
      /></Form.Item>
      <Form.Item label="存储后端" html-for="settings-storage"
        ><Select
          v-select-a11y
          id="settings-storage"
          v-model:value="settings.form.storage"
          :options="storageBackendOptions"
      /></Form.Item>
      <template v-if="settings.form.storage === 'redis'">
        <Form.Item label="Redis 地址" html-for="settings-redis-address"
          ><Input
            id="settings-redis-address"
            name="settings-redis-address"
            v-model:value="settings.form.redis_addr"
            placeholder="127.0.0.1:6379"
        /></Form.Item>
        <Form.Item label="Redis 密码" html-for="settings-redis-password"
          ><Input.Password
            id="settings-redis-password"
            name="settings-redis-password"
            v-model:value="settings.form.redis_password"
        /></Form.Item>
      </template>
      <Typography.Title :level="5">网络与镜像</Typography.Title>
      <Form.Item
        label="GitHub 加速"
        html-for="settings-github-proxy"
        extra="用于读取 GitHub 插件源和下载 GitHub 插件；选择关闭表示直连。"
      >
        <Space.Compact style="width: 100%">
          <Select
            v-select-a11y
            id="settings-github-proxy"
            v-model:value="settings.form.github_proxy"
            style="width: calc(100% - 92px)"
            :options="[
              { value: '', label: '关闭加速' },
              ...settingSelectOptions(settings.githubProxyOptions),
            ]"
          >
            <template #dropdownRender="{ menuNode: menu }">
              <VNodes :vnodes="menu" />
              <div class="settings-select-add" @mousedown.stop>
                <Input
                  id="settings-custom-github-proxy"
                  name="settings-custom-github-proxy"
                  aria-label="自定义 GitHub 加速地址"
                  v-model:value="settings.customGithubProxy"
                  placeholder="输入自定义 GitHub 加速地址"
                  @press-enter="addSettingsOption('github')"
                />
                <Button
                  type="primary"
                  shape="circle"
                  :loading="settings.optionSaving.github"
                  title="添加"
                  aria-label="添加 GitHub 加速地址"
                  @mousedown.prevent.stop
                  @click="addSettingsOption('github')"
                >
                  <template #icon><Plus :size="16" /></template>
                </Button>
              </div>
            </template>
          </Select>
          <Button
            :loading="settings.optionSaving.github"
            :disabled="!settingOptionDeletable('github')"
            @click="removeSettingsOption('github')"
            >删除</Button
          >
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label="pnpm 镜像"
        html-for="settings-pnpm-registry"
        extra="用于安装和更新脚本插件的 NodeJS 依赖。"
      >
        <Space.Compact style="width: 100%">
          <Select
            v-select-a11y
            id="settings-pnpm-registry"
            v-model:value="settings.form.pnpm_registry"
            show-search
            style="width: calc(100% - 92px)"
            :options="settingSelectOptions(settings.pnpmRegistryOptions)"
          >
            <template #dropdownRender="{ menuNode: menu }">
              <VNodes :vnodes="menu" />
              <div class="settings-select-add" @mousedown.stop>
                <Input
                  id="settings-custom-pnpm-registry"
                  name="settings-custom-pnpm-registry"
                  aria-label="自定义 npm/pnpm registry"
                  v-model:value="settings.customPnpmRegistry"
                  placeholder="输入自定义 npm/pnpm registry"
                  @press-enter="addSettingsOption('pnpm')"
                />
                <Button
                  type="primary"
                  shape="circle"
                  :loading="settings.optionSaving.pnpm"
                  title="添加"
                  aria-label="添加 pnpm 镜像"
                  @mousedown.prevent.stop
                  @click="addSettingsOption('pnpm')"
                >
                  <template #icon><Plus :size="16" /></template>
                </Button>
              </div>
            </template>
          </Select>
          <Button
            :loading="settings.optionSaving.pnpm"
            :disabled="!settingOptionDeletable('pnpm')"
            @click="removeSettingsOption('pnpm')"
            >删除</Button
          >
        </Space.Compact>
      </Form.Item>
      <Form.Item
        label="pipx 源"
        html-for="settings-pipx-registry"
        extra="用于安装 Python 脚本插件依赖。"
      >
        <Space.Compact style="width: 100%">
          <Select
            v-select-a11y
            id="settings-pipx-registry"
            v-model:value="settings.form.pipx_registry"
            show-search
            style="width: calc(100% - 92px)"
            :options="settingSelectOptions(settings.pipxRegistryOptions)"
          >
            <template #dropdownRender="{ menuNode: menu }">
              <VNodes :vnodes="menu" />
              <div class="settings-select-add" @mousedown.stop>
                <Input
                  id="settings-custom-pipx-registry"
                  name="settings-custom-pipx-registry"
                  aria-label="自定义 pip/PyPI 源"
                  v-model:value="settings.customPipxRegistry"
                  placeholder="输入自定义 pip/PyPI 源"
                  @press-enter="addSettingsOption('pipx')"
                />
                <Button
                  type="primary"
                  shape="circle"
                  :loading="settings.optionSaving.pipx"
                  title="添加"
                  aria-label="添加 pipx 源"
                  @mousedown.prevent.stop
                  @click="addSettingsOption('pipx')"
                >
                  <template #icon><Plus :size="16" /></template>
                </Button>
              </div>
            </template>
          </Select>
          <Button
            :loading="settings.optionSaving.pipx"
            :disabled="!settingOptionDeletable('pipx')"
            @click="removeSettingsOption('pipx')"
            >删除</Button
          >
        </Space.Compact>
      </Form.Item>
      <Form.Item label="系统调试模式" html-for="settings-debug"
        ><Switch
          id="settings-debug"
          v-model:checked="settings.form.debug"
          aria-label="系统调试模式"
      /></Form.Item>
      <Form.Item label="未监听群允许管理员触发" html-for="settings-listen-admin"
        ><Switch
          id="settings-listen-admin"
          v-model:checked="settings.form.listen_admin"
          aria-label="未监听群允许管理员触发"
      /></Form.Item>
      <Button type="primary" @click="saveSettings"
        ><template #icon><Save :size="16" /></template>保存设置</Button
      >
      <Typography.Title :level="5" class="settings-backup-title"
        >数据备份</Typography.Title
      >
      <Card size="small" class="settings-backup-card">
        <Typography.Paragraph class="settings-backup-description">
          下载包含全部存储桶、用户与 BOT 配置、插件配置及脚本源文件的 ZIP
          备份。运行时依赖、缓存、PID 和锁文件不会打包。
        </Typography.Paragraph>
        <Alert
          type="warning"
          show-icon
          message="备份包含账号绑定、Token 和密钥等敏感配置，请妥善保管。"
          style="margin-bottom: 14px"
        />
        <Button
          :loading="systemBackup.downloading"
          @click="downloadSystemBackup"
        >
          <template #icon><Download :size="16" /></template>下载备份
        </Button>
      </Card>
    </Form>
  </section>
</template>
