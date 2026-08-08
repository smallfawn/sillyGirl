<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import { Download, RefreshCw, Trash2 } from "lucide-vue-next";
import Empty from "ant-design-vue/es/empty";
import Input from "ant-design-vue/es/input";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Segmented from "ant-design-vue/es/segmented";
import Select from "ant-design-vue/es/select";
import Switch from "ant-design-vue/es/switch";
import Table from "ant-design-vue/es/table";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import { python } from "@codemirror/lang-python";
import { useAdminViewContext } from "../adminViewContext";

const {
  currentDependencyTool,
  dependencyPackagePlaceholder,
  dependencyPluginOptions,
  dependencyRegistryLabel,
  dependencyRuntimeLabel,
  dependencyRuntimeOptions,
  dependencySharedPath,
  installNodeDependency,
  installNodeDependencyRow,
  loadNodeDependencies,
  nodeDeps,
  page,
  plugins,
  removeNodeDependency,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'dependencies'" class="panel">
    <div class="toolbar">
      <div class="toolbar-left">
        <Typography.Text class="muted"
          >共 {{ nodeDeps.plugins.length }} 个
          {{ dependencyRuntimeLabel }} 脚本插件，依赖共享到
          {{ dependencySharedPath }}</Typography.Text
        >
        <Typography.Text class="muted"
          >{{ dependencyRegistryLabel }}：{{
            nodeDeps.registry
          }}</Typography.Text
        >
        <Typography.Text v-if="currentDependencyTool.message" type="danger">{{
          currentDependencyTool.message
        }}</Typography.Text>
      </div>
      <div class="toolbar-right">
        <Button @click="loadNodeDependencies()"
          ><template #icon><RefreshCw :size="16" /></template>刷新</Button
        >
      </div>
    </div>
    <div class="toolbar-left" style="margin-bottom: 12px">
      <Segmented
        v-model:value="nodeDeps.runtime"
        :options="dependencyRuntimeOptions"
      />
      <Select
        id="dependency-plugin"
        v-model:value="nodeDeps.plugin"
        aria-label="依赖所属插件"
        style="width: 260px"
        show-search
        :options="dependencyPluginOptions"
        option-filter-prop="label"
      />
      <Input
        id="dependency-package"
        name="dependency-package"
        v-model:value="nodeDeps.packageName"
        aria-label="依赖名称"
        style="width: 320px"
        :placeholder="dependencyPackagePlaceholder"
        @press-enter="installNodeDependency"
      />
      <Switch
        v-if="nodeDeps.runtime === 'node'"
        id="dependency-dev-mode"
        v-model:checked="nodeDeps.dev"
        aria-label="依赖类型：Dev 或 Prod"
        checked-children="Dev"
        un-checked-children="Prod"
      />
      <Button
        type="primary"
        :disabled="!currentDependencyTool.available"
        :loading="nodeDeps.saving"
        @click="installNodeDependency"
      >
        <template #icon><Download :size="16" /></template>安装依赖
      </Button>
    </div>
    <Table
      :row-key="
        (row: any) =>
          `${row.type || nodeDeps.runtime}.${row.plugin}.${row.name}`
      "
      :loading="nodeDeps.loading"
      :data-source="nodeDeps.rows"
      :pagination="{ pageSize: 20 }"
    >
      <Table.Column title="#" :width="64">
        <template #default="{ index }">{{ index + 1 }}</template>
      </Table.Column>
      <Table.Column title="依赖名称" data-index="name" />
      <Table.Column title="版本" data-index="version" :width="180">
        <template #default="{ text }"
          ><Typography.Text class="mono">{{
            text || "-"
          }}</Typography.Text></template
        >
      </Table.Column>
      <Table.Column title="状态" :width="110">
        <template #default="{ record }"
          ><Tag :color="record.installed ? 'green' : 'orange'">{{
            record.installed ? "已安装" : "未安装"
          }}</Tag></template
        >
      </Table.Column>
      <Table.Column title="操作" :width="130">
        <template #default="{ record }">
          <Button
            v-if="!record.installed"
            type="link"
            :disabled="!currentDependencyTool.available"
            :loading="nodeDeps.saving"
            @click="installNodeDependencyRow(record)"
            >安装</Button
          >
          <Popconfirm
            v-else
            title="确认卸载这个依赖？"
            @confirm="removeNodeDependency(record)"
          >
            <Button
              type="text"
              danger
              :title="`卸载 ${record.name}`"
              :aria-label="`卸载 ${record.name}`"
              :loading="
                nodeDeps.removing[
                  `${nodeDeps.runtime}.${record.plugin}.${record.name}`
                ]
              "
              ><Trash2 :size="16"
            /></Button>
          </Popconfirm>
        </template>
      </Table.Column>
    </Table>
    <Empty
      v-if="!nodeDeps.loading && nodeDeps.rows.length === 0"
      description="暂未识别到插件需要依赖。"
    />
  </section>
</template>
