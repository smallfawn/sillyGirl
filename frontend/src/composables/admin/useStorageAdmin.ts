import { computed, reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post, saveStorage } from "../../api";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useStorageAdmin() {
  const storageState = reactive({
    bucket: "sillyGirl",
    search: "",
    newBucketName: "",
    createBucketOpen: false,
    entryBucket: "sillyGirl",
    entryKey: "",
    entryValue: "",
    rows: [] as any[],
    current: 1,
    pageSize: 20,
    total: 0,
    buckets: [] as Array<{ value: string; label: string }>,
    loading: false,
    loadingBuckets: false,
    creatingBucket: false,
    savingEntry: false,
    deletingBucket: false,
  });
  const protectedStorageBuckets = new Set(["plugins", "sillyGirl", "auths"]);
  const selectedStorageBucket = computed(() => {
    return storageState.bucket.trim();
  });
  const canRemoveStorageBucket = computed(
    () =>
      !!selectedStorageBucket.value &&
      !protectedStorageBuckets.has(selectedStorageBucket.value),
  );
  async function loadStorageBuckets() {
    storageState.loadingBuckets = true;
    try {
      const res = await get<
        ApiEnvelope<Array<{ value: string; text?: string }>>
      >("/api/admin/storage/buckets");
      storageState.buckets = (apiData(res) || []).map((item) => ({
        value: item.value,
        label: item.text || item.value,
      }));
    } finally {
      storageState.loadingBuckets = false;
    }
  }
  async function loadStorage(
    current = 1,
    pageSize = storageState.pageSize,
    includeBuckets = false,
  ) {
    storageState.loading = true;
    try {
      const params = new URLSearchParams({
        bucket: selectedStorageBucket.value,
        page: String(current),
        page_size: String(pageSize),
      });
      const search = storageState.search.trim();
      if (search) params.set("search", search);
      if (includeBuckets) params.set("include", "buckets");
      const res = await get<
        ApiEnvelope<{
          list: any[];
          total: number;
          page?: number;
          page_size?: number;
          buckets?: Array<{ value: string; text?: string }>;
        }>
      >(`/api/admin/storage/entries?${params.toString()}`);
      const data = apiData(res);
      storageState.rows = data?.list || [];
      storageState.current = data?.page || current;
      storageState.pageSize = data?.page_size || pageSize;
      storageState.total = data?.total || 0;
      if (data?.buckets) {
        storageState.buckets = data.buckets.map((item) => ({
          value: item.value,
          label: item.text || item.value,
        }));
      }
    } finally {
      storageState.loading = false;
    }
  }
  function changeStoragePage(pagination: {
    current?: number;
    pageSize?: number;
  }) {
    return loadStorage(
      pagination.current || 1,
      pagination.pageSize || storageState.pageSize,
    );
  }
  async function saveStorageRow(row: any) {
    await saveStorage({ [`${row.bucket}.${row.key}`]: row.value });
    message.success("已保存");
  }
  async function selectStorageBucket(bucket?: string) {
    storageState.bucket = bucket || "";
    storageState.search = "";
    if (!bucket) {
      storageState.rows = [];
      return;
    }
    await loadStorage(1);
  }
  async function openCreateStorageBucket() {
    storageState.newBucketName = "";
    storageState.createBucketOpen = true;
    if (!storageState.buckets.length) await loadStorageBuckets();
  }
  async function createStorageBucket() {
    const bucket = storageState.newBucketName.trim();
    if (!bucket) {
      message.error("请输入存储桶名称");
      return;
    }
    storageState.creatingBucket = true;
    try {
      await post("/api/admin/storage/buckets", { bucket });
      message.success("存储桶已创建");
      storageState.newBucketName = "";
      storageState.createBucketOpen = false;
      storageState.bucket = bucket;
      storageState.search = "";
      await loadStorageBuckets();
      await loadStorage(1);
    } finally {
      storageState.creatingBucket = false;
    }
  }
  async function createStorageEntry() {
    const bucket =
      selectedStorageBucket.value || storageState.entryBucket.trim();
    const key = storageState.entryKey.trim();
    if (!bucket) {
      message.error("请先选择单个存储桶");
      return;
    }
    if (!key) {
      message.error("请输入 Key");
      return;
    }
    if (storageState.entryValue === "") {
      message.error("请输入 Value");
      return;
    }
    storageState.savingEntry = true;
    try {
      await saveStorage({ [`${bucket}.${key}`]: storageState.entryValue });
      message.success("Key/Value 已添加");
      storageState.entryBucket = bucket;
      storageState.entryKey = "";
      storageState.entryValue = "";
      storageState.bucket = bucket;
      storageState.search = "";
      await loadStorageBuckets();
      await loadStorage(1);
    } finally {
      storageState.savingEntry = false;
    }
  }
  async function removeStorageBucket() {
    const bucket = selectedStorageBucket.value;
    if (!bucket) {
      message.error("请选择单个存储桶");
      return;
    }
    storageState.deletingBucket = true;
    try {
      await post(`/api/admin/storage/buckets/${encodeURIComponent(bucket)}/deletions`);
      message.success("存储桶已删除");
      storageState.bucket = "sillyGirl";
      storageState.search = "";
      await loadStorageBuckets();
      await loadStorage(1);
    } finally {
      storageState.deletingBucket = false;
    }
  }

  return {
    storageState,
    selectedStorageBucket,
    canRemoveStorageBucket,
    loadStorageBuckets,
    loadStorage,
    changeStoragePage,
    saveStorageRow,
    selectStorageBucket,
    openCreateStorageBucket,
    createStorageBucket,
    createStorageEntry,
    removeStorageBucket,
  };
}
