import { Modal } from "ant-design-vue";
import { h } from "vue";

import type { PluginInfo } from "../../types";

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

export function confirmPluginDependencyInstall(
  row: PluginInfo,
  packages: string[],
  onConfirm: () => Promise<void>,
) {
  const modules = [
    ...new Set(
      (row.module_dependencies || [])
        .map((item) => String(item).trim())
        .filter(Boolean),
    ),
  ];
  if (packages.length === 0 && modules.length === 0) return false;
  Modal.confirm({
    title: `${row.title || row.id} 需要安装依赖`,
    content: h("div", { class: "plugin-dependency-confirm" }, [
      modules.length
        ? h("p", `依赖模块（优先自动安装）：${modules.join("、")}`)
        : null,
      packages.length
        ? h("p", `运行依赖（随后自动安装）：${packages.join("、")}`)
        : null,
      h("p", "继续后将自动处理以上依赖并安装插件。"),
    ]),
    okText: "自动安装",
    cancelText: "取消安装",
    centered: true,
    onOk: onConfirm,
  });
  return true;
}
