<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import {
  ArrowUp,
  CloudDownload,
  Home,
  Moon,
  Plus,
  RefreshCw,
  Save,
  Search,
  Settings,
  Sun,
  Trash2,
  Wand2,
} from "lucide-vue-next";
import Button from "ant-design-vue/es/button";
import Empty from "ant-design-vue/es/empty";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import InputNumber from "ant-design-vue/es/input-number";
import Modal from "ant-design-vue/es/modal";
import Pagination from "ant-design-vue/es/pagination";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Radio from "ant-design-vue/es/radio";
import Select from "ant-design-vue/es/select";
import Space from "ant-design-vue/es/space";
import Spin from "ant-design-vue/es/spin";
import Switch from "ant-design-vue/es/switch";
import Tabs, { TabPane } from "ant-design-vue/es/tabs";
import Table from "ant-design-vue/es/table";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import { python } from "@codemirror/lang-python";
import { ref } from "vue";
import { useAdminViewContext } from "../adminViewContext";
import PluginUninstallModal from "../PluginUninstallModal.vue";

const {
  addPluginSource,
  cancelUninstallPluginModal,
  closeMarketPluginEditor,
  confirmUninstallPlugin,
  deleteMarketPluginEditor,
  fieldOptions,
  fieldType,
  filterPluginClassOption,
  formatMarketPluginEditor,
  handlePluginEditorOpenChange,
  installPlugin,
  loadPlugins,
  openMarketPluginConfig,
  openMarketPluginEditor,
  openNewMarketPluginEditor,
  openPluginDetail,
  openPluginSourceManager,
  openUninstallPluginModal,
  page,
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
  removePluginSource,
  saveMarketPluginEditor,
  savePluginConfig,
  schemaFields,
  searchPluginsNow,
  settings,
  syncPluginEditorLanguage,
  togglePluginEditorTheme,
  togglePluginStatus,
  user,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'plugins'" class="panel">
    <Tabs v-model:active-key="plugins.tab">
      <TabPane key="all" :tab="`全部 ${plugins.meta.all ?? ''}`" />
      <TabPane key="latest" :tab="`最新发布 ${plugins.meta.latest ?? ''}`" />
      <TabPane key="private" :tab="`非公开 ${plugins.meta.private ?? ''}`" />
      <TabPane key="module" :tab="`依赖模块 ${plugins.meta.modules ?? ''}`" />
      <TabPane key="tab1" :tab="`已安装 ${plugins.meta.tab1 ?? ''}`" />
      <TabPane key="tab2" :tab="`未安装 ${plugins.meta.tab2 ?? ''}`" />
      <TabPane key="tab3" :tab="`可更新 ${plugins.meta.tab3 ?? ''}`" />
    </Tabs>
    <Alert
      class="plugin-market-hint"
      type="info"
      show-icon
      description="点击插件图标可以打开插件源码编辑器；点击插件名字可以查看插件介绍。"
      style="margin-bottom: 12px"
    />
    <div class="toolbar-left" style="margin-bottom: 12px">
      <Input
        id="plugin-market-search"
        v-model:value="plugins.keyword"
        name="plugin-market-search"
        allow-clear
        style="width: 260px"
        placeholder="输入即搜索，支持模糊匹配"
        @press-enter="searchPluginsNow"
      />
      <Select
        id="plugin-market-class"
        v-model:value="plugins.klass"
        show-search
        style="width: 180px"
        placeholder="插件分类"
        :options="pluginClassOptions"
        :filter-option="filterPluginClassOption"
      />
      <Button type="primary" @click="searchPluginsNow"
        ><template #icon><Search :size="16" /></template>搜索</Button
      >
      <Button type="primary" @click="openPluginSourceManager">
        <template #icon><Settings :size="16" /></template>管理插件源
      </Button>
      <Button @click="loadPlugins(1, 16, true)"
        ><template #icon><RefreshCw :size="16" /></template>刷新</Button
      >
      <Button type="primary" @click="openNewMarketPluginEditor"
        ><template #icon><Plus :size="16" /></template>新增插件</Button
      >
      <span class="plugin-admin-legend" aria-label="淡蓝色卡片表示管理员插件">
        <span class="plugin-admin-legend-swatch" aria-hidden="true"></span>
        <span>管理员插件</span>
      </span>
    </div>
    <Spin :spinning="plugins.loading">
      <div v-if="plugins.rows.length" class="plugin-market-grid">
        <article
          v-for="record in plugins.rows"
          :key="record.id"
          class="plugin-market-card"
          :class="{ 'plugin-market-card--admin': record.admin }"
        >
          <button
            type="button"
            class="plugin-card-icon"
            :aria-label="`编辑 ${record.title || record.id} 源码`"
            @click="openMarketPluginEditor(record)"
          >
            <img
              v-if="pluginIconIsImage(record)"
              :src="record.icon"
              :alt="record.title || record.id"
            />
            <span v-else>{{ pluginInitial(record) }}</span>
          </button>
          <div class="plugin-card-main">
            <div class="plugin-card-title-row">
              <button
                type="button"
                class="plugin-card-title"
                @click="openPluginDetail(record)"
              >
                {{ record.title || record.id }}
              </button>
              <Tag
                v-if="pluginHasSchedule(record)"
                class="plugin-card-cron-badge"
                >定时</Tag
              >
              <Tag v-if="record.module" color="purple">依赖模块</Tag>
            </div>
            <div class="plugin-card-meta">
              <Tag>{{
                record.latest_version ||
                record.version ||
                record.current_version ||
                "-"
              }}</Tag>
              <Tag color="cyan">{{
                pluginClassTags(record).join(" / ") || "-"
              }}</Tag>
              <Tag color="gold">{{ pluginTriggerText(record) || "-" }}</Tag>
              <Tag color="blue">{{ record.author || "-" }}</Tag>
            </div>
          </div>
          <div class="plugin-card-actions">
            <Switch
              v-if="pluginInstalled(record) && !record.module"
              class="plugin-card-status-switch"
              :checked="pluginRuntimeEnabled(record)"
              :loading="plugins.toggling[record.id]"
              :disabled="
                Boolean(
                  plugins.installing[record.id] ||
                  plugins.uninstalling[record.id],
                )
              "
              :title="`${record.title || record.id}启用状态`"
              :aria-label="`${record.title || record.id}启用状态`"
              @change="
                (checked: boolean) => togglePluginStatus(record, checked)
              "
            />
            <Button
              v-if="pluginCanManage(record)"
              class="plugin-card-settings"
              shape="circle"
              :loading="pluginConfigs.opening === record.id"
              title="插件设置"
              :aria-label="`${record.title || record.id}插件设置`"
              @click="openMarketPluginConfig(record)"
            >
              <template #icon><Settings :size="18" /></template>
            </Button>
            <Button
              v-if="pluginUpgradable(record)"
              class="plugin-card-upgrade"
              shape="circle"
              :loading="plugins.installing[record.id]"
              :title="`升级 ${record.title || record.id}`"
              :aria-label="`升级 ${record.title || record.id}`"
              @click="installPlugin(record)"
            >
              <template #icon><ArrowUp :size="20" /></template>
            </Button>
            <Button
              v-else-if="pluginInstalled(record)"
              class="plugin-card-remove"
              shape="circle"
              danger
              :loading="
                plugins.uninstalling[record.id] ||
                plugins.dependencyChecking[record.id]
              "
              :title="`卸载 ${record.title || record.id}`"
              :aria-label="`卸载 ${record.title || record.id}`"
              @click="openUninstallPluginModal(record)"
            >
              <template #icon><Trash2 :size="18" /></template>
            </Button>
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

  <Modal
    v-model:open="pluginEditor.open"
    :title="
      pluginEditor.isNew
        ? '新增本地插件'
        : `编辑插件：${pluginEditor.title || pluginEditor.id}`
    "
    width="1080px"
    :footer="null"
    :destroy-on-close="true"
    @cancel="closeMarketPluginEditor"
    @after-open-change="handlePluginEditorOpenChange"
  >
    <Spin :spinning="pluginEditor.loading">
      <Space direction="vertical" style="width: 100%" size="middle">
        <Alert
          type="info"
          show-icon
          message="本地新增插件会自动作为非公开插件进入插件市场；保存前必须包含 [title: xxx]、[name: 文件名]、[desc: xxx]、[version: vx.y.z]，以及 [rule: xxx] 或 [cron: xxx]/[on_start: true]/[web: true]/[module: true]。"
        />
        <Form layout="inline" class="plugin-editor-meta">
          <Form.Item
            label="插件名称"
            required
            extra="新增时必须和源码里的 [name: xxx] 一致"
          >
            <Input
              id="plugin-editor-name"
              name="plugin-editor-name"
              v-model:value="pluginEditor.name"
              style="width: 260px"
              placeholder="例如 localPlugin"
              :disabled="pluginEditor.installed && !pluginEditor.isNew"
              @input="pluginEditor.name = $event.target.value"
              @change="pluginEditor.name = $event.target.value"
            />
          </Form.Item>
          <Form.Item label="类型" required>
            <Select
              id="plugin-editor-type"
              v-model:value="pluginEditor.type"
              style="width: 160px"
              :options="[
                { label: 'NodeJS', value: 'node' },
                { label: 'Python', value: 'python' },
              ]"
              @change="syncPluginEditorLanguage"
            />
          </Form.Item>
          <Form.Item>
            <Tag :color="pluginEditor.installed ? 'green' : 'blue'">{{
              pluginEditor.installed ? "本地已安装" : "未安装/远程源码"
            }}</Tag>
          </Form.Item>
          <Form.Item>
            <Button @click="togglePluginEditorTheme">
              <template #icon
                ><Moon v-if="pluginEditor.theme === 'dark'" :size="16" /><Sun
                  v-else
                  :size="16"
              /></template>
              {{ pluginEditor.theme === "dark" ? "黑底" : "白底" }}
            </Button>
          </Form.Item>
        </Form>
        <div
          ref="pluginEditorHost"
          class="script-code-editor plugin-market-code-editor"
          :class="{
            'plugin-market-code-editor-dark': pluginEditor.theme === 'dark',
            'plugin-market-code-editor-light': pluginEditor.theme === 'light',
          }"
        ></div>
        <Space style="justify-content: flex-end; width: 100%">
          <Popconfirm
            v-if="pluginEditor.installed && !pluginEditor.isNew"
            title="确认删除这个本地插件文件？"
            @confirm="deleteMarketPluginEditor"
          >
            <Button danger :loading="pluginEditor.deleting"
              ><template #icon><Trash2 :size="16" /></template>删除</Button
            >
          </Popconfirm>
          <Button @click="formatMarketPluginEditor"
            ><template #icon><Wand2 :size="16" /></template>格式化</Button
          >
          <Button @click="closeMarketPluginEditor">取消</Button>
          <Button
            type="primary"
            :loading="pluginEditor.saving"
            @click="saveMarketPluginEditor"
            ><template #icon><Save :size="16" /></template>保存</Button
          >
        </Space>
      </Space>
    </Spin>
  </Modal>

  <Modal
    :open="plugins.sourceModal"
    title="管理插件源"
    width="820px"
    :footer="null"
    @cancel="plugins.sourceModal = false"
  >
    <Space direction="vertical" style="width: 100%" size="middle">
      <Form layout="vertical">
        <Form.Item label="新增插件源" required style="margin-bottom: 0">
          <Space.Compact style="width: 100%">
            <Input
              id="plugin-source-address"
              name="plugin-source-address"
              v-model:value="plugins.sourceAddress"
              placeholder="https://github.com/smallfawn/sillyGirl_Plugins 或 link://..."
              @press-enter="addPluginSource"
            />
            <Button
              type="primary"
              :loading="plugins.sourceSaving"
              @click="addPluginSource"
            >
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
            <Popconfirm
              title="确认删除这个插件源？"
              @confirm="removePluginSource(record.address)"
            >
              <Button
                type="text"
                danger
                :title="`删除插件源 ${record.address}`"
                :aria-label="`删除插件源 ${record.address}`"
                :loading="plugins.sourceRemoving[record.address]"
              >
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
          <img
            v-if="pluginIconIsImage(plugins.detail)"
            :src="plugins.detail.icon"
            alt=""
          />
          <span v-else>{{ pluginInitial(plugins.detail) }}</span>
        </div>
        <div class="plugin-detail-heading">
          <Typography.Title :level="4">{{
            plugins.detail.title || plugins.detail.id
          }}</Typography.Title>
          <Space wrap size="small">
            <Tag :color="pluginStatusColor(plugins.detail)">{{
              pluginStatusLabel(plugins.detail)
            }}</Tag>
            <Tag v-if="plugins.detail.module" color="purple">依赖模块</Tag>
            <Tag v-if="plugins.detail.version">{{
              plugins.detail.version
            }}</Tag>
            <Tag
              v-for="klass in pluginClassTags(plugins.detail)"
              :key="klass"
              color="cyan"
              >{{ klass }}</Tag
            >
          </Space>
        </div>
      </div>

      <Typography.Paragraph class="plugin-detail-description">
        {{ plugins.detail.desc || "该插件暂未填写介绍。" }}
      </Typography.Paragraph>

      <Alert
        v-if="
          plugins.detail.install_status === 1 && plugins.detail.update_content
        "
        type="success"
        show-icon
        :message="`更新内容：${plugins.detail.update_content}`"
      />

      <dl class="plugin-detail-meta">
        <template v-if="pluginTriggerText(plugins.detail)">
          <dt>{{ plugins.detail.module ? "用途" : "触发口令" }}</dt>
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
        <template v-if="plugins.detail.install_status === 1">
          <dt>版本</dt>
          <dd>
            当前 {{ plugins.detail.current_version || "-" }} / 最新
            {{ plugins.detail.latest_version || plugins.detail.version || "-" }}
          </dd>
        </template>
        <template v-if="plugins.detail.organization">
          <dt>来源</dt>
          <dd>{{ plugins.detail.organization }}</dd>
        </template>
      </dl>
    </div>
  </Modal>

  <PluginUninstallModal
    :state="plugins.uninstallModal"
    :loading="
      plugins.uninstallModal.row
        ? Boolean(plugins.uninstalling[plugins.uninstallModal.row.id])
        : false
    "
    @confirm="confirmUninstallPlugin"
    @cancel="cancelUninstallPluginModal"
    @update-delete-config="plugins.uninstallModal.deleteConfig = $event"
  />

  <Modal
    v-model:open="pluginConfigs.modalOpen"
    :title="`${pluginConfigs.selected?.plugin || pluginConfigs.selected?.title || '插件'} 设置`"
    width="720px"
    ok-text="保存"
    cancel-text="取消"
    :confirm-loading="pluginConfigs.saving"
    :ok-button-props="{ disabled: !pluginSettingsCanSave }"
    @ok="savePluginConfig"
  >
    <Spin :spinning="pluginConfigs.loading">
      <div v-if="pluginConfigs.selected" class="plugin-settings-modal-body">
        <section v-if="pluginOpenAvailable" class="plugin-user-open-setting">
          <div class="plugin-user-open-copy">
            <Typography.Text strong>是否开放为普通用户</Typography.Text>
            <Typography.Text type="secondary">
              开启后普通用户可在 Home 页面看到该插件，并自行决定是否授权其读取
              SmallCat 账号列表。
            </Typography.Text>
          </div>
          <Switch
            id="plugin-open-to-users"
            v-model:checked="pluginConfigs.openToUsers"
            aria-label="是否开放为普通用户"
          />
        </section>
        <div
          v-if="
            pluginConfigs.configurable ||
            pluginConfigs.selected.registered === false
          "
          class="config-form plugin-config-modal-form"
        >
          <Typography.Text class="muted mono">{{
            pluginConfigs.selected.file || "main.js"
          }}</Typography.Text>
          <Alert
            v-if="pluginConfigs.selected.registered === false"
            type="warning"
            show-icon
            style="margin-top: 16px"
            message="该插件检测到配置代码，但安装时没有成功导出配置表单。请确认 new form({...}) 在脚本顶层执行，且脚本初始化没有报错。"
          />
          <Form
            v-if="pluginConfigs.configurable"
            layout="vertical"
            style="margin-top: 16px"
          >
            <template v-for="field in schemaFields" :key="field.key">
              <Form.Item
                v-if="pluginConfigFieldVisible(field)"
                :label="field.prop.title || field.key"
                :html-for="`plugin-config-${field.key}`"
                :extra="field.prop.description"
              >
                <template v-if="pluginPanelKind(field)">
                  <Radio.Group
                    :id="`plugin-config-${field.key}`"
                    v-model:value="pluginConfigs.form[field.key]"
                    class="plugin-panel-id-picker"
                    button-style="solid"
                    :aria-label="field.prop.title || field.key"
                  >
                    <Radio.Button
                      v-for="id in pluginPanelChoices(field)"
                      :key="id"
                      :value="id"
                      >{{ id }}</Radio.Button
                    >
                  </Radio.Group>
                  <div
                    v-if="!pluginPanelChoices(field).length"
                    class="plugin-panel-empty"
                  >
                    {{ pluginPanelEmptyText(field) }}
                  </div>
                </template>
                <Select
                  v-else-if="fieldType(field.prop) === 'enum'"
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
                  v-else-if="
                    fieldType(field.prop) === 'number' ||
                    fieldType(field.prop) === 'integer'
                  "
                  :id="`plugin-config-${field.key}`"
                  v-model:value="pluginConfigs.form[field.key]"
                  style="width: 100%"
                  :min="field.prop.minimum"
                  :max="field.prop.maximum"
                />
                <Input.TextArea
                  v-else-if="
                    fieldType(field.prop) === 'object' ||
                    fieldType(field.prop) === 'array'
                  "
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.text[field.key]"
                  :rows="6"
                  class="mono"
                />
                <Input.Password
                  v-else-if="
                    field.prop.format === 'password' ||
                    field.prop['ui:widget'] === 'password'
                  "
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.form[field.key]"
                />
                <Input.TextArea
                  v-else-if="
                    field.prop.format === 'textarea' ||
                    field.prop['ui:widget'] === 'textarea'
                  "
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.form[field.key]"
                  :rows="4"
                />
                <Input
                  v-else
                  :id="`plugin-config-${field.key}`"
                  :name="field.key"
                  v-model:value="pluginConfigs.form[field.key]"
                />
              </Form.Item>
            </template>
          </Form>
        </div>
        <Empty v-else description="该插件没有其他配置项" />
      </div>
    </Spin>
  </Modal>
</template>
