import { h } from "vue";
import Modal from "ant-design-vue/es/modal";
import { get } from "../../api";
import type { PluginInfo } from "../../types";
import { apiData, type ApiEnvelope } from "./adminApi";

export type DownloadedPluginDependencyPlan = {
  runtime: "node" | "python";
  plugin: string;
  pluginTitle: string;
  dependencies: Array<{
    name: string;
    version: string;
    dev: boolean;
    installed: boolean;
    source?: string;
    plugin: string;
    plugin_title?: string;
    plugin_file?: string;
    type?: string;
  }>;
  moduleDependencies: string[];
  tool: { available: boolean; message?: string };
};

export async function resolveDownloadedPluginDependencyPlan(
  row: PluginInfo,
): Promise<DownloadedPluginDependencyPlan> {
  const res = await get<
    ApiEnvelope<{
      runtime: "node" | "python";
      plugin: string;
      plugin_title: string;
      dependencies: DownloadedPluginDependencyPlan["dependencies"];
      module_dependencies: string[];
      tool: DownloadedPluginDependencyPlan["tool"];
    }>
  >(`/api/admin/local-plugins/${encodeURIComponent(row.id)}/dependency-plan`);
  const data = apiData(res);
  return {
    runtime: data.runtime,
    plugin: data.plugin,
    pluginTitle: data.plugin_title || row.title || data.plugin,
    dependencies: data.dependencies || [],
    moduleDependencies: data.module_dependencies || [],
    tool: data.tool || { available: false },
  };
}

export function confirmDownloadedPluginDependencyInstall(
  plan: DownloadedPluginDependencyPlan,
  install: () => Promise<void>,
) {
  const packageNames = plan.dependencies.map((item) => item.name).join("、");
  const toolWarning =
    plan.dependencies.length > 0 && plan.tool?.available === false
      ? `安装工具当前不可用：${plan.tool.message || (plan.runtime === "python" ? "pipx/Python 未就绪" : "pnpm 未就绪")}`
      : "";
  Modal.confirm({
    title: `${plan.pluginTitle} 下载完成，检测到未安装依赖`,
    content: h("div", { class: "plugin-dependency-confirm" }, [
      plan.moduleDependencies.length
        ? h("p", `依赖模块：${plan.moduleDependencies.join("、")}`)
        : null,
      plan.dependencies.length ? h("p", `运行依赖：${packageNames}`) : null,
      toolWarning ? h("p", { style: "color: #d46b08" }, toolWarning) : null,
      h("p", "插件源码已下载，是否现在自动安装以上缺失依赖？"),
    ]),
    okText: "自动安装",
    cancelText: "暂不安装",
    centered: true,
    onOk: install,
  });
}

export function pluginDependencies(row: PluginInfo) {
  return [
    ...new Set(
      (row.dependencies || [])
        .map((item) => String(item).trim())
        .filter(Boolean),
    ),
  ];
}

export function declaredPluginDependenciesFromContent(content: string) {
  const dependencies = new Set<string>();
  const addRaw = (rawValue: string) => {
    const raw = String(rawValue || "").trim();
    if (!raw) return;
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        parsed.forEach((item) => {
          const value = String(item || "").trim();
          if (value) dependencies.add(value);
        });
        return;
      }
      if (parsed && typeof parsed === "object") {
        Object.keys(parsed).forEach((key) => {
          const value = String(key || "").trim();
          if (value) dependencies.add(value);
        });
        return;
      }
    } catch {
      // 兼容 [depe: axios, ipp] 手写格式；规范格式仍然是 JSON array。
    }
    raw.split(/[,，\s]+/).forEach((item) => {
      const value = item.trim().replace(/^['"]|['"]$/g, "");
      if (value && value !== "[" && value !== "]") dependencies.add(value);
    });
  };
  const pattern =
    /^[ \t]*(?:\/\/|#+)[ \t]*\[[ \t]*depe[ \t]*:[ \t]*(.*)[ \t]*\][^\r\n]*$/gim;
  const atPattern = /^[ \t]*(?:\*[ \t]*)?@depe(?:[ \t]+(.+?))?[ \t]*$/gim;
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(content || ""))) addRaw(match[1]);
  while ((match = atPattern.exec(content || ""))) addRaw(match[1]);
  return [...dependencies].filter((item) => !item.startsWith("./"));
}
