<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import { Play, Plus, RefreshCw, Trash2 } from "lucide-vue-next";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Select from "ant-design-vue/es/select";
import Switch from "ant-design-vue/es/switch";
import Table from "ant-design-vue/es/table";
import { python } from "@codemirror/lang-python";
import { timestamp } from "../../../utils";
import { useAdminViewContext } from "../adminViewContext";

const {
  loadTasks,
  openTask,
  page,
  removeTask,
  runTask,
  saveTask,
  scripts,
  tasks,
  toggleTaskEnabled,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'tasks'" class="panel">
    <div class="toolbar-left" style="margin-bottom: 12px">
      <Button type="primary" @click="openTask()"
        ><template #icon><Plus :size="16" /></template>新增定时任务</Button
      >
      <Button @click="loadTasks()"
        ><template #icon><RefreshCw :size="16" /></template>刷新</Button
      >
    </div>
    <Table
      row-key="task_id"
      :data-source="tasks.rows"
      :pagination="{ total: tasks.total, pageSize: 20, onChange: loadTasks }"
    >
      <Table.Column title="#" data-index="id" :width="64" />
      <Table.Column title="标题" data-index="title" :width="180" />
      <Table.Column title="Cron" data-index="schedule" :width="180" />
      <Table.Column title="命令" data-index="command" ellipsis />
      <Table.Column title="状态" data-index="enable" :width="80" align="center">
        <template #default="{ record }">
          <Switch
            :checked="record.enable"
            :loading="tasks.toggling[record.task_id]"
            :aria-label="`${record.title}启用状态`"
            @change="(checked: boolean) => toggleTaskEnabled(record, checked)"
          />
        </template>
      </Table.Column>
      <Table.Column title="创建时间" data-index="created_at" :width="180"
        ><template #default="{ text }">{{
          timestamp(text)
        }}</template></Table.Column
      >
      <Table.Column title="操作" :width="180"
        ><template #default="{ record }"
          ><Button type="text" @click="runTask(record)"
            ><Play :size="16" /></Button
          ><Button type="text" @click="openTask(record)">编辑</Button
          ><Popconfirm title="确认删除？" @confirm="removeTask(record)"
            ><Button type="text" danger
              ><Trash2 :size="16" /></Button></Popconfirm></template
      ></Table.Column>
    </Table>
  </section>

  <Modal
    :open="!!tasks.editing"
    title="定时任务"
    width="640px"
    @cancel="tasks.editing = null"
    @ok="saveTask"
  >
    <Form layout="vertical">
      <Form.Item
        label="标题"
        html-for="task-title"
        required
        help="定时任务标题不能为空"
        ><Input
          id="task-title"
          v-model:value="tasks.form.title"
          name="task-title"
          placeholder="例如：每小时检查 IP"
      /></Form.Item>
      <Form.Item
        label="Cron 表达式"
        html-for="task-schedule"
        required
        help="例如：0 * * * *，也支持带秒字段的 6 段表达式"
        ><Input
          id="task-schedule"
          v-model:value="tasks.form.schedule"
          name="task-schedule"
          placeholder="0 * * * *"
      /></Form.Item>
      <Form.Item label="触发命令" html-for="task-command"
        ><Select
          id="task-command"
          v-model:value="tasks.form.command"
          show-search
          :options="tasks.scripts"
          placeholder="node xxx.js 或 python xxx.py"
      /></Form.Item>
      <Form.Item label="接收平台" html-for="task-platform"
        ><Select
          id="task-platform"
          v-model:value="tasks.form.platform"
          allow-clear
          :options="tasks.platforms"
          placeholder="选择 BOT 平台"
      /></Form.Item>
      <Form.Item
        label="接收人 ID"
        html-for="task-recipient"
        help="填写该平台的用户 ID；插件调用 s.reply() 时会私聊此账号"
        ><Input
          id="task-recipient"
          v-model:value="tasks.form.recipient"
          name="task-recipient"
          allow-clear
          placeholder="请输入用户 ID / OpenID"
      /></Form.Item>
    </Form>
  </Modal>
</template>
