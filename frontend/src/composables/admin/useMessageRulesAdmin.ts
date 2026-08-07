import { reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useMessageRulesAdmin() {
  const messageBuckets = {
    listen: { label: "监听群组", resource: "listening" },
    noreply: { label: "禁言群组", resource: "muted" },
    private: { label: "屏蔽用户", resource: "blocked" },
  };
  const msgState = reactive({
    active: "listen" as keyof typeof messageBuckets,
    rows: [] as any[],
    editing: null as any,
    form: {} as any,
    platforms: [] as any[],
  });
  async function loadMessages() {
    const resource = messageBuckets[msgState.active].resource;
    const res = await get<ApiEnvelope<{ list: any[]; platforms: any[] }>>(
      `/api/admin/message-rules/${resource}`,
    );
    const data = apiData(res);
    msgState.rows = (data?.list || []).map((row) => {
      try {
        return { ...row, ...JSON.parse(row.value || "{}") };
      } catch {
        return row;
      }
    });
    msgState.platforms = data?.platforms || [];
  }
  function openMessage(row?: any) {
    msgState.editing = row || { key: "", enable: true };
    msgState.form = { ...msgState.editing };
  }
  async function saveMessageRow() {
    const resource = messageBuckets[msgState.active].resource;
    await post(
      `/api/admin/message-rules/${resource}/${encodeURIComponent(msgState.form.key)}`,
      {
        platform: msgState.form.platform || "",
        enable: !!msgState.form.enable,
        desc: msgState.form.desc || "",
      },
    );
    msgState.editing = null;
    message.success("已保存");
    loadMessages();
  }
  async function removeMessageRow(row: any) {
    const resource = messageBuckets[msgState.active].resource;
    await post(
      `/api/admin/message-rules/${resource}/${encodeURIComponent(row.key)}/deletions`,
    );
    message.success("已删除");
    loadMessages();
  }

  return {
    messageBuckets,
    msgState,
    loadMessages,
    openMessage,
    saveMessageRow,
    removeMessageRow,
  };
}
