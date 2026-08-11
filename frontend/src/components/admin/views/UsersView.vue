<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import Button from "ant-design-vue/es/button";
import { Edit3, Plus, RefreshCw, ShieldCheck, Trash2 } from "lucide-vue-next";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Select from "ant-design-vue/es/select";
import Space from "ant-design-vue/es/space";
import Spin from "ant-design-vue/es/spin";
import Switch from "ant-design-vue/es/switch";
import Table from "ant-design-vue/es/table";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import { timestamp } from "../../../utils";
import { useAdminViewContext } from "../adminViewContext";

const {
  loadNormalUsers,
  normalUserPluginAuthorizations,
  normalUsers,
  openNormalUserPluginAuthorizations,
  openNormalUser,
  page,
  removeNormalUser,
  saveNormalUser,
  saveNormalUserPluginAuthorization,
  pluginAuthorizations,
  smallcat,
  smallcatOpenids,
  user,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'users'" class="panel">
    <div class="toolbar">
      <div class="toolbar-left">
        <Typography.Text strong>普通用户</Typography.Text>
        <Tag>{{ normalUsers.total }}</Tag>
      </div>
      <Space>
        <Button type="primary" @click="openNormalUser()"
          ><template #icon><Plus :size="16" /></template>新增账号</Button
        >
        <Button @click="loadNormalUsers"
          ><template #icon><RefreshCw :size="16" /></template>刷新</Button
        >
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
            <Typography.Text class="muted">{{
              record.nickname || "-"
            }}</Typography.Text>
          </Space>
        </template>
      </Table.Column>
      <Table.Column title="smallcat openid" :width="300">
        <template #default="{ record }">
          <Space
            v-if="smallcatOpenids(record).length"
            direction="vertical"
            size="small"
          >
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
          <Typography.Text class="mono">{{
            record.bindings?.qq || "-"
          }}</Typography.Text>
        </template>
      </Table.Column>
      <Table.Column title="TGID" :width="170">
        <template #default="{ record }">
          <Typography.Text class="mono">{{
            record.bindings?.telegram || "-"
          }}</Typography.Text>
        </template>
      </Table.Column>
      <Table.Column title="已授权插件" :width="360">
        <template #default="{ record }">
          <Space
            v-if="normalUserPluginAuthorizations(record).length"
            wrap
            size="small"
          >
            <Tag
              v-for="plugin in normalUserPluginAuthorizations(record).slice(
                0,
                4,
              )"
              :key="plugin.uuid"
              :color="plugin.authorized ? 'blue' : 'default'"
            >
              {{ plugin.title || plugin.uuid }}
            </Tag>
            <Tag
              v-if="normalUserPluginAuthorizations(record).length > 4"
              color="default"
            >
              +{{ normalUserPluginAuthorizations(record).length - 4 }}
            </Tag>
          </Space>
          <Typography.Text v-else class="muted">-</Typography.Text>
        </template>
      </Table.Column>
      <Table.Column title="绑定更新时间" :width="180">
        <template #default="{ record }">{{
          timestamp(record.bindings?.updated_at)
        }}</template>
      </Table.Column>
      <Table.Column title="注册时间" data-index="created_at" :width="180">
        <template #default="{ text }">{{ timestamp(text) }}</template>
      </Table.Column>
      <Table.Column title="状态" :width="100">
        <template #default="{ record }">
          <Tag :color="record.disabled ? 'default' : 'green'">{{
            record.disabled ? "禁用" : "正常"
          }}</Tag>
        </template>
      </Table.Column>
      <Table.Column title="操作" fixed="right" :width="130">
        <template #default="{ record }">
          <Space size="small">
            <Button
              type="text"
              title="管理插件授权"
              :aria-label="`管理插件授权 ${record.username}`"
              @click="openNormalUserPluginAuthorizations(record)"
            >
              <ShieldCheck :size="16" />
            </Button>
            <Button
              type="text"
              title="编辑账号"
              :aria-label="`编辑账号 ${record.username}`"
              @click="openNormalUser(record)"
            >
              <Edit3 :size="16" />
            </Button>
            <Popconfirm
              :title="`确认删除账号「${record.username}」？`"
              description="账号、openid/QQ/TGID 绑定和插件授权将一并删除。"
              ok-text="确认删除"
              cancel-text="取消"
              @confirm="removeNormalUser(record)"
            >
              <Button
                type="text"
                danger
                :loading="normalUsers.deleting[record.id]"
                title="删除账号"
                :aria-label="`删除账号 ${record.username}`"
              >
                <Trash2 :size="16" />
              </Button>
            </Popconfirm>
          </Space>
        </template>
      </Table.Column>
    </Table>
  </section>

  <Modal
    v-model:open="normalUsers.modalOpen"
    :title="
      normalUsers.editing
        ? `编辑账号：${normalUsers.editing.username}`
        : '新增账号'
    "
    width="620px"
    ok-text="保存"
    cancel-text="取消"
    :confirm-loading="normalUsers.saving"
    @ok="saveNormalUser"
  >
    <Form layout="vertical">
      <Form.Item
        label="账号"
        html-for="normal-user-username"
        required
        extra="3-32 位字母、数字、下划线、横线或点；创建后不可修改。"
      >
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
        <Input
          id="normal-user-nickname"
          v-model:value="normalUsers.form.nickname"
          name="normal-user-nickname"
          maxlength="64"
          placeholder="留空则使用账号名"
        />
      </Form.Item>
      <Form.Item
        label="smallcat openid 列表"
        extra="可输入多个 openid，按回车确认；保存时自动去重。"
      >
        <Select
          v-model:value="normalUsers.form.smallcat_openids"
          mode="tags"
          :token-separators="[',', ';', ' ']"
          :options="[]"
          placeholder="输入 openid 后按回车"
        />
      </Form.Item>
      <Form.Item label="绑定 QQ" html-for="normal-user-qq">
        <Input
          id="normal-user-qq"
          v-model:value="normalUsers.form.qq"
          name="normal-user-qq"
          inputmode="numeric"
          placeholder="5-12 位 QQ 号；留空解除绑定"
        />
      </Form.Item>
      <Form.Item label="绑定 TGID" html-for="normal-user-tgid">
        <Input
          id="normal-user-tgid"
          v-model:value="normalUsers.form.telegram"
          name="normal-user-tgid"
          placeholder="Telegram 用户 ID；留空解除绑定"
        />
      </Form.Item>
      <Form.Item label="禁用账号" html-for="normal-user-disabled">
        <Switch
          id="normal-user-disabled"
          v-model:checked="normalUsers.form.disabled"
        />
      </Form.Item>
    </Form>
  </Modal>

  <Modal
    v-model:open="pluginAuthorizations.modalOpen"
    :title="
      pluginAuthorizations.user
        ? `插件授权：${pluginAuthorizations.user.username}`
        : '插件授权'
    "
    width="920px"
    :footer="null"
    @cancel="pluginAuthorizations.modalOpen = false"
  >
    <Spin :spinning="pluginAuthorizations.loading">
      <Space direction="vertical" style="width: 100%" size="middle">
        <Alert
          type="info"
          show-icon
          message="切换开关即可为该用户添加或移除插件授权。"
        />
        <Table
          row-key="uuid"
          size="small"
          :data-source="pluginAuthorizations.rows"
          :pagination="{ pageSize: 10 }"
        >
          <Table.Column title="插件" :width="320">
            <template #default="{ record }">
              <Space direction="vertical" size="small">
                <Typography.Text strong>{{
                  record.title || record.uuid
                }}</Typography.Text>
                <Typography.Text class="muted mono">{{
                  record.uuid
                }}</Typography.Text>
              </Space>
            </template>
          </Table.Column>
          <Table.Column title="说明">
            <template #default="{ record }">
              <Typography.Paragraph class="muted" :ellipsis="{ rows: 2 }">
                {{ record.desc || "该插件暂未填写介绍。" }}
              </Typography.Paragraph>
            </template>
          </Table.Column>
          <Table.Column title="状态" :width="180">
            <template #default="{ record }">
              <Space wrap size="small">
                <Tag :color="record.installed ? 'green' : 'default'">{{
                  record.installed ? "已安装" : "已失效"
                }}</Tag>
                <Tag :color="record.open ? 'blue' : 'default'">{{
                  record.open ? "已开放" : "未开放"
                }}</Tag>
                <Tag v-if="record.uses_smallcat" color="cyan"
                  >smallcat</Tag
                >
              </Space>
            </template>
          </Table.Column>
          <Table.Column title="授权" :width="140">
            <template #default="{ record }">
              <Switch
                :checked="!!record.authorized"
                :loading="pluginAuthorizations.saving[record.uuid]"
                checked-children="已授权"
                un-checked-children="未授权"
                @change="
                  (checked: boolean) =>
                    saveNormalUserPluginAuthorization(record, checked)
                "
              />
            </template>
          </Table.Column>
        </Table>
      </Space>
    </Spin>
  </Modal>
</template>
