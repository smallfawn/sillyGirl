<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import { Edit3, Plus, RefreshCw, Trash2 } from "lucide-vue-next";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import InputNumber from "ant-design-vue/es/input-number";
import Modal from "ant-design-vue/es/modal";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Segmented from "ant-design-vue/es/segmented";
import Select from "ant-design-vue/es/select";
import Switch from "ant-design-vue/es/switch";
import Tabs, { TabPane } from "ant-design-vue/es/tabs";
import Table from "ant-design-vue/es/table";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import { timestamp } from "../../../utils";
import { useAdminViewContext } from "../adminViewContext";

const {
  carry,
  changeCarryPlatform,
  loadActiveMessageTool,
  loadCarry,
  loadReplies,
  messageBuckets,
  messageToolAddLabel,
  messageToolHelpText,
  messageToolKind,
  messageToolOptions,
  msgState,
  openActiveMessageTool,
  openCarry,
  openMessage,
  openReply,
  optionMap,
  page,
  recordOptions,
  removeCarry,
  removeMessageRow,
  removeReply,
  replies,
  saveCarry,
  saveMessageRow,
  saveReply,
  scripts,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'message-tools'" class="panel">
    <div class="toolbar">
      <div class="toolbar-left">
        <Segmented
          v-model:value="messageToolKind"
          :options="messageToolOptions"
        />
        <Button type="primary" @click="openActiveMessageTool()"
          ><template #icon><Plus :size="16" /></template
          >{{ messageToolAddLabel }}</Button
        >
        <Button @click="loadActiveMessageTool"
          ><template #icon><RefreshCw :size="16" /></template>刷新</Button
        >
      </div>
      <Typography.Text class="muted">{{ messageToolHelpText }}</Typography.Text>
    </div>

    <Table
      v-if="messageToolKind === 'carry'"
      row-key="chat_id"
      :data-source="carry.rows"
      :pagination="{ total: carry.total, pageSize: 20, onChange: loadCarry }"
    >
      <Table.Column title="#" data-index="id" :width="64" />
      <Table.Column title="平台" data-index="platform" :width="100" />
      <Table.Column title="群号" data-index="chat_id" :width="160" />
      <Table.Column title="备注" data-index="remark" />
      <Table.Column title="操作" :width="150"
        ><template #default="{ record }"
          ><Button type="text" @click="openCarry(record)">编辑</Button
          ><Popconfirm title="确认删除？" @confirm="removeCarry(record)"
            ><Button type="text" danger
              ><Trash2 :size="16" /></Button></Popconfirm></template
      ></Table.Column>
    </Table>

    <Table
      v-else-if="messageToolKind === 'reply'"
      :row-key="(row: any) => String(row.id)"
      :data-source="replies.rows"
      :pagination="{
        total: replies.total,
        pageSize: 20,
        onChange: loadReplies,
      }"
    >
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
          <Button type="text" @click="openReply(record)"
            ><Edit3 :size="16"
          /></Button>
          <Popconfirm title="确认删除？" @confirm="removeReply(record)"
            ><Button type="text" danger><Trash2 :size="16" /></Button
          ></Popconfirm>
        </template>
      </Table.Column>
    </Table>

    <template v-else>
      <Tabs v-model:active-key="msgState.active">
        <TabPane
          v-for="[key, item] in Object.entries(messageBuckets)"
          :key="key"
          :tab="item.label"
        />
      </Tabs>
      <Table row-key="key" :data-source="msgState.rows">
        <Table.Column title="号码" data-index="key" :width="220" />
        <Table.Column title="平台" data-index="platform" :width="140" />
        <Table.Column title="说明" data-index="desc" />
        <Table.Column title="启用" data-index="enable" :width="90"
          ><template #default="{ text }">{{
            text ? "是" : "否"
          }}</template></Table.Column
        >
        <Table.Column title="操作" :width="150"
          ><template #default="{ record }"
            ><Button type="text" @click="openMessage(record)">编辑</Button
            ><Popconfirm title="确认删除？" @confirm="removeMessageRow(record)"
              ><Button type="text" danger
                ><Trash2 :size="16" /></Button></Popconfirm></template
        ></Table.Column>
      </Table>
    </template>
  </section>

  <Modal
    :open="!!replies.editing"
    title="回复规则"
    @cancel="replies.editing = null"
    @ok="saveReply"
  >
    <Form layout="vertical"
      ><Form.Item label="关键词/正则"
        ><Input v-model:value="replies.form.keyword" /></Form.Item
      ><Form.Item label="回复内容"
        ><Input.TextArea
          v-model:value="replies.form.value"
          :rows="6" /></Form.Item
      ><Form.Item label="限定用户/群号"
        ><Input v-model:value="replies.form.number" /></Form.Item
      ><Form.Item label="平台"
        ><Select
          v-model:value="replies.form.platforms"
          mode="tags" /></Form.Item
      ><Form.Item label="优先级"
        ><InputNumber
          v-model:value="replies.form.priority"
          style="width: 100%" /></Form.Item
    ></Form>
  </Modal>

  <Modal
    :open="!!carry.editing"
    title="搬运群组"
    width="820px"
    @cancel="carry.editing = null"
    @ok="saveCarry"
  >
    <Form layout="vertical">
      <Form.Item label="平台" required>
        <Select
          v-model:value="carry.form.platform"
          :options="optionMap(carry.selects.platforms)"
          @change="changeCarryPlatform"
        />
      </Form.Item>
      <Form.Item label="群号" required>
        <Input v-model:value="carry.form.chat_id" />
      </Form.Item>
      <Form.Item label="备注">
        <Input.TextArea v-model:value="carry.form.remark" :rows="2" />
      </Form.Item>
      <Form.Item label="工作机器人">
        <Select
          v-model:value="carry.form.bots_id"
          mode="multiple"
          :options="optionMap(carry.selects.bots_id)"
        />
      </Form.Item>
      <Form.Item label="处理脚本">
        <Select
          v-model:value="carry.form.scripts"
          mode="multiple"
          :options="recordOptions(carry.selects.scripts)"
        />
      </Form.Item>
    </Form>
  </Modal>

  <Modal
    :open="!!msgState.editing"
    :title="messageBuckets[msgState.active].label"
    @cancel="msgState.editing = null"
    @ok="saveMessageRow"
  >
    <Form layout="vertical"
      ><Form.Item :label="msgState.active === 'private' ? '用户 ID' : '群号'"
        ><Input
          v-model:value="msgState.form.key"
          :disabled="!!msgState.editing?.value" /></Form.Item
      ><Form.Item label="平台"
        ><Select
          v-model:value="msgState.form.platform"
          :options="msgState.platforms" /></Form.Item
      ><Form.Item label="说明"
        ><Input v-model:value="msgState.form.desc" /></Form.Item
      ><Form.Item label="启用"
        ><Switch v-model:checked="msgState.form.enable" /></Form.Item
    ></Form>
  </Modal>
</template>
