import { reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { CarryGroup } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useCarryAdmin() {
  const carry = reactive({
    rows: [] as CarryGroup[],
    total: 0,
    editing: null as CarryGroup | null,
    form: {} as any,
    selects: {} as any,
  });
  async function loadCarry(current = 1, pageSize = 20) {
    const res = await get<ApiEnvelope<{ list: CarryGroup[]; total: number }>>(
      `/api/admin/carry-groups?page=${current}&page_size=${pageSize}`,
    );
    const data = apiData(res);
    carry.rows = data?.list || [];
    carry.total = data?.total || 0;
  }
  async function loadCarrySelects(row?: CarryGroup) {
    const res = await get<ApiEnvelope<any>>(
      `/api/admin/carry-group-options?chat_id=${encodeURIComponent(row?.chat_id || "")}&platform=${encodeURIComponent(row?.platform || "")}`,
    );
    carry.selects = apiData(res) || {};
  }
  async function changeCarryPlatform(platform: string) {
    carry.form.platform = platform;
    carry.form.bots_id = [];
    await loadCarrySelects({ ...(carry.form as CarryGroup), platform });
  }
  async function openCarry(row?: CarryGroup) {
    const data = row || {
      chat_id: "",
      platform: "",
      remark: "",
      bots_id: [],
      scripts: [],
    };
    carry.editing = data;
    await loadCarrySelects(data);
    carry.form = {
      ...data,
      bots_id: data.bots_id || [],
      scripts: data.scripts || [],
    };
  }
  async function saveCarry() {
    if (!carry.form.chat_id?.trim()) {
      message.error("请输入群号");
      return;
    }
    if (!carry.form.platform) {
      message.error("请选择平台");
      return;
    }
    const payload = {
      chat_id: carry.form.chat_id.trim(),
      platform: carry.form.platform,
      remark: carry.form.remark || "",
      bots_id: carry.form.bots_id || [],
      scripts: carry.form.scripts || [],
    };
    if (carry.editing?.chat_id) {
      await post(
        `/api/admin/carry-groups/${encodeURIComponent(carry.editing.chat_id)}`,
        payload,
      );
    } else {
      await post("/api/admin/carry-groups", payload);
    }
    carry.editing = null;
    message.success("已保存");
    loadCarry();
  }
  async function removeCarry(row: CarryGroup) {
    await post(`/api/admin/carry-groups/${encodeURIComponent(row.chat_id)}/deletions`);
    message.success("已删除");
    loadCarry();
  }

  return {
    carry,
    loadCarry,
    loadCarrySelects,
    changeCarryPlatform,
    openCarry,
    saveCarry,
    removeCarry,
  };
}
