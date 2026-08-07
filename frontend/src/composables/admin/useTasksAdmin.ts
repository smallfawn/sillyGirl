import { reactive } from "vue";
import message from "ant-design-vue/es/message";
import { get, post } from "../../api";
import type { Task } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export function useTasksAdmin() {
  const taskPlatformLabels: Record<string, string> = {
    clawbot: "微信 ClawBot",
    qq: "QQ",
    telegram: "Telegram Bot",
    dingtalk: "钉钉机器人",
    qqguild: "QQ 官方频道机器人",
    web: "Web Bot",
    pagermaid: "Pagermaid",
  };
  const tasks = reactive({
    rows: [] as Task[],
    total: 0,
    editing: null as Task | null,
    form: {} as any,
    toggling: {} as Record<string, boolean>,
    scripts: [] as any[],
    platforms: [] as Array<{ value: string; label: string }>,
    platformBots: {} as Record<string, string[]>,
  });
  async function loadTasks(current = 1, pageSize = 20) {
    const res = await get<ApiEnvelope<{ list: Task[]; total: number }>>(
      `/api/admin/tasks?page=${current}&page_size=${pageSize}`,
    );
    const data = apiData(res);
    tasks.rows = data?.list || [];
    tasks.total = data?.total || 0;
  }
  async function loadTaskSelects(taskId = "") {
    const query = taskId ? `?task_id=${encodeURIComponent(taskId)}` : "";
    const res = await get<
      ApiEnvelope<{
        scripts?: Record<string, string>;
        platforms?: Record<string, string[]>;
      }>
    >(`/api/admin/task-options${query}`);
    const data = apiData(res);
    tasks.scripts = Object.entries(data?.scripts || {})
      .filter(([, label]) => /\.(js|py)$/i.test(String(label)))
      .map(([, label]) => {
        const text = String(label);
        const runtime = /\.py$/i.test(text) ? "python" : "node";
        return { value: `${runtime} ${text}`, label: `${runtime} ${text}` };
      });
    tasks.platformBots = data?.platforms || {};
    const platformNames = new Set([
      ...Object.keys(taskPlatformLabels),
      ...Object.keys(tasks.platformBots),
    ]);
    tasks.platforms = [...platformNames].map((platform) => ({
      value: platform,
      label: taskPlatformLabels[platform] || platform,
    }));
  }
  async function openTask(row?: Task) {
    const data = row || { enable: true, command: "" };
    tasks.editing = data;
    await loadTaskSelects(data.task_id || "");
    const target = data.senders?.[0];
    tasks.form = {
      ...data,
      platform: target?.platform || "",
      recipient: target?.user_id || target?.chat_id || "",
    };
    if (
      tasks.form.platform &&
      !tasks.platforms.some((item) => item.value === tasks.form.platform)
    ) {
      tasks.platforms.push({
        value: tasks.form.platform,
        label: taskPlatformLabels[tasks.form.platform] || tasks.form.platform,
      });
    }
  }
  function validateTaskCron(schedule?: string) {
    const value = `${schedule || ""}`.trim();
    if (!value) return false;
    const parts = value.split(/\s+/);
    if (parts.length !== 5 && parts.length !== 6) return false;
    return parts.every((part) =>
      /^[\d*,/?#LW\-\u0041-\u005A\u0061-\u007A]+$/.test(part),
    );
  }
  async function saveTask() {
    if (!`${tasks.form.title || ""}`.trim()) {
      message.error("定时任务标题不能为空");
      return;
    }
    if (!validateTaskCron(tasks.form.schedule)) {
      message.error("Cron表达式格式错误，例如：0 * * * *");
      return;
    }
    const platform = `${tasks.form.platform || ""}`.trim();
    const recipient = `${tasks.form.recipient || ""}`.trim();
    if ((platform && !recipient) || (!platform && recipient)) {
      message.error("平台和接收人必须同时填写");
      return;
    }
    const payload = {
      task_id: tasks.form.task_id,
      title: `${tasks.form.title || ""}`.trim(),
      schedule: `${tasks.form.schedule || ""}`.trim(),
      command: tasks.form.command,
      enable: tasks.form.enable,
      senders: platform && recipient ? [{ platform, user_id: recipient }] : [],
    };
    if (payload.task_id) {
      await post(
        `/api/admin/tasks/${encodeURIComponent(payload.task_id)}`,
        payload,
      );
    } else {
      await post("/api/admin/tasks", payload);
    }
    tasks.editing = null;
    message.success("已保存");
    loadTasks();
  }
  async function removeTask(row: Task) {
    await post(
      `/api/admin/tasks/${encodeURIComponent(String(row.task_id || ""))}/deletions`,
    );
    message.success("已删除");
    loadTasks();
  }
  async function runTask(row: Task) {
    await post(
      `/api/admin/tasks/${encodeURIComponent(String(row.task_id || ""))}/executions`,
      {},
    );
    message.success("已触发");
  }
  async function toggleTaskEnabled(row: Task, enabled = !row.enable) {
    const taskId = `${row.task_id || ""}`;
    if (!taskId || tasks.toggling[taskId]) return;
    tasks.toggling[taskId] = true;
    try {
      await post(`/api/admin/tasks/${encodeURIComponent(taskId)}`, {
        enable: enabled,
      });
      row.enable = enabled;
      message.success(enabled ? "定时任务已启用" : "定时任务已停用");
      await loadTasks();
    } catch (error) {
      message.error(
        error instanceof Error ? error.message : "更新定时任务状态失败",
      );
    } finally {
      tasks.toggling[taskId] = false;
    }
  }

  return {
    taskPlatformLabels,
    tasks,
    loadTasks,
    loadTaskSelects,
    openTask,
    validateTaskCron,
    saveTask,
    removeTask,
    runTask,
    toggleTaskEnabled,
  };
}
