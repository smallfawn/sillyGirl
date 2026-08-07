<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import { Plus, RefreshCw, Save, Search, Trash2 } from "lucide-vue-next";
import Popconfirm from "ant-design-vue/es/popconfirm";
import Select from "ant-design-vue/es/select";
import Space from "ant-design-vue/es/space";
import Table from "ant-design-vue/es/table";
import { useAdminViewContext } from "../adminViewContext";

const {
  canRemoveStorageBucket,
  changeStoragePage,
  createStorageBucket,
  createStorageEntry,
  loadStorage,
  openCreateStorageBucket,
  page,
  removeStorageBucket,
  saveStorageRow,
  selectStorageBucket,
  selectedStorageBucket,
  storageState,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'storage'" class="panel">
    <div class="toolbar-left" style="margin-bottom: 12px">
      <Select
        :value="storageState.bucket"
        style="width: 220px"
        show-search
        allow-clear
        placeholder="选择存储桶"
        :loading="storageState.loadingBuckets"
        :options="storageState.buckets"
        @change="selectStorageBucket"
      />
      <Input
        v-model:value="storageState.search"
        allow-clear
        style="width: 360px"
        placeholder="按 Key 或 Value 查询"
        :disabled="!selectedStorageBucket"
        autocomplete="off"
        @press-enter="loadStorage(1)"
      />
      <Button type="primary" @click="loadStorage(1)"
        ><template #icon><Search :size="16" /></template>查询</Button
      >
      <Button @click="loadStorage(storageState.current)"
        ><template #icon><RefreshCw :size="16" /></template>刷新</Button
      >
      <Button
        :loading="storageState.creatingBucket"
        @click="openCreateStorageBucket"
      >
        <template #icon><Plus :size="16" /></template>新建桶
      </Button>
      <Popconfirm
        :title="`确认删除存储桶 ${selectedStorageBucket}？`"
        description="删除后该桶内所有键值都会被移除，无法恢复。"
        ok-text="确认删除"
        cancel-text="取消"
        @confirm="removeStorageBucket"
      >
        <Button
          danger
          :disabled="!canRemoveStorageBucket"
          :loading="storageState.deletingBucket"
        >
          <template #icon><Trash2 :size="16" /></template>删除桶
        </Button>
      </Popconfirm>
    </div>
    <Space.Compact style="width: 100%; margin-bottom: 12px">
      <Input
        v-model:value="storageState.entryKey"
        style="width: 260px"
        placeholder="Key"
        :disabled="!selectedStorageBucket"
        @press-enter="createStorageEntry"
      />
      <Input
        v-model:value="storageState.entryValue"
        placeholder="Value"
        :disabled="!selectedStorageBucket"
        @press-enter="createStorageEntry"
      />
      <Button
        type="primary"
        :loading="storageState.savingEntry"
        :disabled="!selectedStorageBucket"
        @click="createStorageEntry"
      >
        <template #icon><Plus :size="16" /></template>
      </Button>
    </Space.Compact>
    <Table
      :row-key="(row: any) => `${row.bucket}.${row.key}`"
      :loading="storageState.loading"
      :data-source="storageState.rows"
      :pagination="{
        current: storageState.current,
        pageSize: storageState.pageSize,
        total: storageState.total,
        showSizeChanger: true,
      }"
      @change="changeStoragePage"
    >
      <Table.Column title="#" data-index="index" :width="64" />
      <Table.Column title="Bucket" data-index="bucket" :width="160" />
      <Table.Column title="Key" data-index="key" :width="220" />
      <Table.Column title="Value">
        <template #default="{ record }">
          <Space.Compact style="width: 100%">
            <Input.TextArea
              v-model:value="record.value"
              :auto-size="{ minRows: 1, maxRows: 6 }"
            />
            <Button @click="saveStorageRow(record)"><Save :size="16" /></Button>
          </Space.Compact>
        </template>
      </Table.Column>
    </Table>
  </section>

  <Modal
    v-model:open="storageState.createBucketOpen"
    title="新建存储桶"
    ok-text="确认创建"
    cancel-text="取消"
    :confirm-loading="storageState.creatingBucket"
    @ok="createStorageBucket"
  >
    <Form layout="vertical">
      <Form.Item
        label="存储桶名称"
        required
        extra="不能包含点号、逗号或空白字符。"
      >
        <Input
          v-model:value="storageState.newBucketName"
          placeholder="例如：myPlugin"
          @press-enter="createStorageBucket"
        />
      </Form.Item>
    </Form>
  </Modal>
</template>
