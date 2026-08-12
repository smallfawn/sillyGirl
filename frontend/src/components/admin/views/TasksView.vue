<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import { computed } from "vue";
import { Play, Plus, RefreshCw, Trash2 } from "lucide-vue-next";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Select from "ant-design-vue/es/select";
import Switch from "ant-design-vue/es/switch";
import Table from "ant-design-vue/es/table";
import { python } from "@codemirror/lang-python";
import { timestamp } from "../../../utils";
import { useAdminViewContext } from "../adminViewContext";

const {
  isPluginCronTask,
  loadPlugins,
  loadTasks,
  openTask,
  page,
  pluginTriggerText,
  plugins,
  removeTask,
  runTask,
  saveTask,
  scripts,
  tasks,
  toggleTaskEnabled,
} = useAdminViewContext();

// 定时任务“触发口令”下方的参考口令：绑定“触发命令”选择。
// 选择触发命令后，自动按该命令对应插件的 [rule] 显示参考口令；未匹配到插件时回退列出全部带 rule 插件作参考。
const pluginRuleOptions = computed(() => {
  const list = (plugins.list || []) as Array<any>;
  return list
    .filter((p) => !p.module && p.rule && String(p.rule).trim())
    .map((p) => ({ id: p.id, title: p.title, ruleText: pluginTriggerText(p) }))
    .filter((x) => x.ruleText);
});

const selectedCommand = computed(() => String(tasks.form.command || "").trim());

const currentPluginRule = computed(() => {
  const cmd = selectedCommand.value;
  const base = cmd
    .replace(/^(node|python)\s+/i, "")
    .replace(/\.(js|py)$/i, "")
    .trim()
    .toLowerCase();
  if (!base) return null;
  return (
    pluginRuleOptions.value.find(
      (x) =>
        String(x.id || "").toLowerCase() === base ||
        String(x.id || "").toLowerCase().includes(base) ||
        String(x.title || "").toLowerCase().includes(base),
    ) || null
  );
});

// 仅在选择了触发命令后才展示参考口令；优先展示当前命令插件的 rule，未匹配时回退到全部插件列表。
const showRuleHint = computed(() => !!selectedCommand.value);
const ruleHintItems = computed(() =>
  currentPluginRule.value ? [currentPluginRule.value] : pluginRuleOptions.value,
);

function ensurePluginRules() {
  if (!((plugins.list || []) as Array<any>).length) {
    void loadPlugins(1, 200);
  }
}

function fillTrigger(ruleText: string) {
  tasks.form.trigger = ruleText;
}
</script>

<template>
  <section v-if="page === 'tasks'" class="panel">
    <div class="toolbar-left" style="margin-bottom: 12px">
      <Button type="primary" @click="openTask(); ensurePluginRules()"
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
      <Table.Column title="触发口令" data-index="trigger" :width="180" ellipsis>
        <template #default="{ text }">{{ text || "—" }}</template>
      </Table.Column>
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
          ><Button
            type="text"
            :title="`运行 ${record.title}`"
            :aria-label="`运行 ${record.title}`"
            @click="runTask(record)"
            ><Play :size="16" /></Button
            ><Button type="text" @click="openTask(record); ensurePluginRules()">编辑</Button
            ><Popconfirm title="确认删除？" @confirm="removeTask(record)"
            ><Button
              type="text"
              danger
              :title="`删除 ${record.title}`"
              :aria-label="`删除 ${record.title}`"
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
        :help="
          isPluginCronTask(tasks.form)
            ? '插件任务标题与命令来自脚本注释，只能在插件编辑器中修改'
            : '定时任务标题不能为空'
        "
        ><Input
          id="task-title"
          v-model:value="tasks.form.title"
          name="task-title"
          :disabled="isPluginCronTask(tasks.form)"
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
          :disabled="isPluginCronTask(tasks.form)"
          :options="tasks.scripts"
          placeholder="node xxx.js 或 python xxx.py"
      /></Form.Item>
      <Form.Item
        label="触发口令"
        html-for="task-trigger"
        help="可选。定时运行时把该内容传给插件，并按插件规则提取参数；多口令插件可借此只执行对应业务"
        ><Input
          id="task-trigger"
          v-model:value="tasks.form.trigger"
          name="task-trigger"
          allow-clear
          placeholder="例如：查询 account-a"
        />
        <div class="trigger-rule-hint" v-if="showRuleHint">
          <div class="trigger-rule-hint__title">
            {{
              currentPluginRule
                ? "参考口令（来自当前触发命令插件，点击填入）"
                : "参考口令（点击填入）"
            }}
          </div>
          <div
            v-if="currentPluginRule"
            class="trigger-rule-hint__current"
            @click="fillTrigger(currentPluginRule.ruleText)"
            :title="`点击填入当前命令插件的口令：${currentPluginRule.ruleText}`"
          >
            <span class="trigger-rule-hint__tag">当前命令插件</span>
            <span class="mono">{{ currentPluginRule.title }}</span>
            <span class="mono trigger-rule-hint__rule">{{ currentPluginRule.ruleText }}</span>
          </div>
          <template v-else>
            <div
              v-for="opt in ruleHintItems"
              :key="opt.id"
              class="trigger-rule-hint__item"
              @click="fillTrigger(opt.ruleText)"
              :title="`点击填入：${opt.ruleText}`"
            >
              <span class="mono">{{ opt.title }}</span>
              <span class="mono trigger-rule-hint__rule">{{ opt.ruleText }}</span>
            </div>
            <div v-if="pluginRuleOptions.length" class="trigger-rule-hint__note">
              已选命令未匹配到带 rule 的插件，以上为全部可用插件口令
            </div>
            <div v-else class="trigger-rule-hint__empty">
              暂无可用插件口令，请先在插件市场安装带 [rule] 的插件
            </div>
          </template>
        </div>
      </Form.Item>
      <Form.Item label="接收平台" html-for="task-platform"
        ><Select
          id="task-platform"
          v-model:value="tasks.form.platform"
          allow-clear
          :options="tasks.platforms"
          placeholder="选择 BOT 平台"
      /></Form.Item>
      <Form.Item label="接收类型" html-for="task-recipient-type"
        ><Select
          id="task-recipient-type"
          v-model:value="tasks.form.recipient_type"
          :options="[
            { value: 'user', label: '私聊用户' },
            { value: 'group', label: '群聊' },
          ]"
      /></Form.Item>
      <Form.Item
        :label="tasks.form.recipient_type === 'group' ? '群号' : '用户 ID'"
        html-for="task-recipient"
        :help="
          tasks.form.recipient_type === 'group'
            ? '填写该平台的群号或群聊 OpenID；插件调用 s.reply() 时会发送到该群聊'
            : '填写该平台的用户 ID；插件调用 s.reply() 时会私聊此账号'
        "
        ><Input
          id="task-recipient"
          v-model:value="tasks.form.recipient"
          name="task-recipient"
          allow-clear
          :placeholder="
            tasks.form.recipient_type === 'group'
              ? '请输入群号 / 群聊 OpenID'
              : '请输入用户 ID / OpenID'
          "
      /></Form.Item>
    </Form>
  </Modal>
</template>

<style scoped>
.trigger-rule-hint {
  margin-top: 8px;
  border: 1px dashed var(--border-color, #d9d9d9);
  border-radius: 6px;
  padding: 8px;
  background: var(--bg-elevated, #fafafa);
  max-height: 240px;
  overflow: auto;
}
.trigger-rule-hint__title {
  font-size: 12px;
  color: #888;
  margin-bottom: 6px;
}
.trigger-rule-hint__current,
.trigger-rule-hint__item {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
}
.trigger-rule-hint__current {
  background: #e6f4ff;
  margin-bottom: 6px;
}
.trigger-rule-hint__item:hover,
.trigger-rule-hint__current:hover {
  background: #f0f5ff;
}
.trigger-rule-hint__tag {
  flex: none;
  background: #1677ff;
  color: #fff;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 3px;
}
.trigger-rule-hint__rule {
  color: #cf1322;
  font-weight: 600;
}
.trigger-rule-hint__empty {
  color: #bbb;
  font-size: 12px;
}
</style>
