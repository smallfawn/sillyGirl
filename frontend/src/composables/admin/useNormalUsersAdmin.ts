import { reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { AdminUserRow } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useNormalUsersAdmin(
  smallcatOpenids: (record?: AdminUserRow) => string[],
) {
  type NormalUserForm = {
    username: string;
    password: string;
    nickname: string;
    qq: string;
    telegram: string;
    smallcat_openids: string[];
    disabled: boolean;
  };

  const emptyNormalUserForm = (): NormalUserForm => ({
    username: "",
    password: "",
    nickname: "",
    qq: "",
    telegram: "",
    smallcat_openids: [],
    disabled: false,
  });

  const normalUsers = reactive({
    rows: [] as AdminUserRow[],
    total: 0,
    loading: false,
    modalOpen: false,
    editing: null as AdminUserRow | null,
    saving: false,
    deleting: {} as Record<string, boolean>,
    form: emptyNormalUserForm(),
  });
  async function loadNormalUsers() {
    normalUsers.loading = true;
    try {
      const res =
        await get<ApiEnvelope<{ list: AdminUserRow[]; total: number }>>(
          "/api/admin/users",
        );
      const data = apiData(res);
      normalUsers.rows = data?.list || [];
      normalUsers.total = data?.total || normalUsers.rows.length;
    } finally {
      normalUsers.loading = false;
    }
  }
  function openNormalUser(row?: AdminUserRow) {
    normalUsers.editing = row || null;
    normalUsers.form = row
      ? {
          username: row.username,
          password: "",
          nickname: row.nickname || "",
          qq: row.bindings?.qq || "",
          telegram: row.bindings?.telegram || "",
          smallcat_openids: smallcatOpenids(row),
          disabled: !!row.disabled,
        }
      : emptyNormalUserForm();
    normalUsers.modalOpen = true;
  }
  async function saveNormalUser() {
    const form = normalUsers.form;
    const username = form.username.trim();
    if (!username) {
      message.warning("请输入账号");
      return;
    }
    if (!normalUsers.editing && form.password.length < 6) {
      message.warning("密码至少 6 位");
      return;
    }
    normalUsers.saving = true;
    try {
      const payload = {
        username,
        password: form.password,
        nickname: form.nickname.trim(),
        qq: form.qq.trim(),
        telegram: form.telegram.trim(),
        smallcat_openids: [
          ...new Set(
            form.smallcat_openids
              .map((item) => String(item).trim())
              .filter(Boolean),
          ),
        ],
        disabled: !!form.disabled,
      };
      if (normalUsers.editing) {
        await post(
          `/api/admin/users/${encodeURIComponent(payload.username)}`,
          payload,
        );
        message.success("账号已更新");
      } else {
        await post("/api/admin/users", payload);
        message.success("账号已新增");
      }
      normalUsers.modalOpen = false;
      await loadNormalUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "保存账号失败");
    } finally {
      normalUsers.saving = false;
    }
  }
  async function removeNormalUser(row: AdminUserRow) {
    normalUsers.deleting[row.id] = true;
    try {
      await post(`/api/admin/users/${encodeURIComponent(row.username)}/deletions`);
      message.success("账号已删除");
      await loadNormalUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "删除账号失败");
    } finally {
      normalUsers.deleting[row.id] = false;
    }
  }

  return {
    normalUsers,
    loadNormalUsers,
    openNormalUser,
    saveNormalUser,
    removeNormalUser,
  };
}
