import { reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { Master } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useMastersAdmin() {
  const masters = reactive({
    rows: [] as Master[],
    platforms: [] as any[],
    editing: false,
    form: {} as Master,
  });
  async function loadMasters() {
    const res =
      await get<ApiEnvelope<{ list: Master[]; platforms: any[] }>>(
        "/api/admin/masters",
      );
    const data = apiData(res);
    masters.rows = data?.list || [];
    masters.platforms = data?.platforms || [];
  }
  async function saveMaster() {
    await post("/api/admin/masters", masters.form);
    masters.editing = false;
    message.success("已保存");
    loadMasters();
  }
  async function removeMaster(row: Master) {
    await post(
      `/api/admin/masters/${encodeURIComponent(String(row.platform || ""))}/${encodeURIComponent(String(row.number || ""))}/deletions`,
    );
    message.success("已删除");
    loadMasters();
  }

  return {
    masters,
    loadMasters,
    saveMaster,
    removeMaster,
  };
}
