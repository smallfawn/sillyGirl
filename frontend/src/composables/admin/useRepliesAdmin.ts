import { reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { Reply } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useRepliesAdmin() {
  const replies = reactive({
    rows: [] as Reply[],
    total: 0,
    editing: null as Reply | null,
    form: {} as Reply,
  });
  async function loadReplies(current = 1, pageSize = 20) {
    const res = await get<ApiEnvelope<{ list: Reply[]; total: number }>>(
      `/api/admin/replies?page=${current}&page_size=${pageSize}`,
    );
    const data = apiData(res);
    replies.rows = data?.list || [];
    replies.total = data?.total || 0;
  }
  function openReply(row?: Reply) {
    replies.editing = row || { id: 0, priority: 0, platforms: [] };
    replies.form = { ...replies.editing };
  }
  async function saveReply() {
    if (replies.form.id) {
      await post(
        `/api/admin/replies/${encodeURIComponent(replies.form.id)}`,
        replies.form,
      );
    } else {
      await post("/api/admin/replies", replies.form);
    }
    replies.editing = null;
    message.success("已保存");
    loadReplies();
  }
  async function removeReply(row: Reply) {
    await post(`/api/admin/replies/${row.id}/deletions`);
    message.success("已删除");
    loadReplies();
  }

  return {
    replies,
    loadReplies,
    openReply,
    saveReply,
    removeReply,
  };
}
