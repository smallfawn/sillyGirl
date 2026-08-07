<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import { Plus, RefreshCw, Trash2 } from "lucide-vue-next";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Select from "ant-design-vue/es/select";
import Table from "ant-design-vue/es/table";
import { timestamp } from "../../../utils";
import { useAdminViewContext } from "../adminViewContext";

const { loadMasters, masters, page, removeMaster, saveMaster } =
  useAdminViewContext();
</script>

<template>
  <section v-if="page === 'masters'" class="panel">
    <div class="toolbar-left" style="margin-bottom: 12px">
      <Button
        type="primary"
        @click="
          masters.editing = true;
          masters.form = {};
        "
        ><template #icon><Plus :size="16" /></template>新增管理员</Button
      >
      <Button @click="loadMasters"
        ><template #icon><RefreshCw :size="16" /></template>刷新</Button
      >
    </div>
    <Table
      :row-key="(row: any) => `${row.platform}.${row.number}`"
      :data-source="masters.rows"
    >
      <Table.Column title="#" data-index="id" :width="64" />
      <Table.Column title="平台" data-index="platform" :width="140" />
      <Table.Column title="账号" data-index="number" :width="180" />
      <Table.Column title="昵称" data-index="nickname" />
      <Table.Column title="记录时间" data-index="unix" :width="180"
        ><template #default="{ text }">{{
          timestamp(text)
        }}</template></Table.Column
      >
      <Table.Column title="操作" :width="100"
        ><template #default="{ record }"
          ><Popconfirm title="确认删除？" @confirm="removeMaster(record)"
            ><Button type="text" danger
              ><Trash2 :size="16" /></Button></Popconfirm></template
      ></Table.Column>
    </Table>
  </section>

  <Modal
    v-model:open="masters.editing"
    title="管理员"
    @cancel="masters.editing = false"
    @ok="saveMaster"
  >
    <Form layout="vertical"
      ><Form.Item label="平台"
        ><Select
          v-model:value="masters.form.platform"
          :options="masters.platforms" /></Form.Item
      ><Form.Item label="账号"
        ><Input v-model:value="masters.form.number" /></Form.Item
    ></Form>
  </Modal>
</template>
