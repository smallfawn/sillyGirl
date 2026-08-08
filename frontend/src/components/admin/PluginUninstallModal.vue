<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import Checkbox from "ant-design-vue/es/checkbox";
import Modal from "ant-design-vue/es/modal";
import Space from "ant-design-vue/es/space";
import Typography from "ant-design-vue/es/typography";

import type { PluginInfo } from "../../types";

type PluginDependent = { id: string; title: string; type: string };
type PluginUninstallState = {
  open: boolean;
  row: PluginInfo | null;
  deleteConfig: boolean;
  dependents: PluginDependent[];
};

defineProps<{ state: PluginUninstallState; loading: boolean }>();
const emit = defineEmits<{
  confirm: [];
  cancel: [];
  "update-delete-config": [value: boolean];
}>();
</script>

<template>
  <Modal
    :open="state.open"
    title="卸载插件"
    width="620px"
    ok-text="确认卸载"
    cancel-text="取消"
    :confirm-loading="loading"
    :ok-button-props="{ danger: true, disabled: state.dependents.length > 0 }"
    @ok="emit('confirm')"
    @cancel="emit('cancel')"
  >
    <Space
      v-if="state.row"
      direction="vertical"
      size="middle"
      style="width: 100%"
    >
      <Alert
        type="warning"
        show-icon
        :message="`确认卸载「${state.row.title || state.row.id}」？`"
        :description="
          state.row.module
            ? '卸载会删除模块文件；系统会阻止删除仍被其他插件引用的依赖模块。'
            : '卸载会删除本地插件脚本文件并停止正在运行的插件任务。'
        "
      />
      <Alert
        v-if="state.dependents.length"
        type="error"
        show-icon
        message="该依赖模块仍在使用中"
        :description="`请先处理这些插件：${state.dependents.map((item) => item.title || item.id).join('、')}`"
      />
      <Checkbox
        :checked="state.deleteConfig"
        @update:checked="emit('update-delete-config', Boolean($event))"
      >
        同步删除插件配置
      </Checkbox>
      <Typography.Paragraph class="muted" style="margin-bottom: 0">
        勾选后会同时删除该插件的配置值和配置表单缓存；不勾选则保留配置，后续重新安装可继续使用。
      </Typography.Paragraph>
    </Space>
  </Modal>
</template>
