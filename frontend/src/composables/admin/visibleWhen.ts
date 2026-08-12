// 插件配置字段的条件可见性（V1.1.8 通用 visibleWhen 机制）
// - evalVisibleWhen：解释 schema["ui:visibleWhen"] 声明的规则，不再依赖前端硬编码特例。
// - pluginConfigFieldVisible：插件配置弹窗字段是否可见，优先按 schema 规则，
//   旧 account_mode / sync_panel 硬编码分支仅作历史兼容（未来可移除）。

const VISIBLE_WHEN_OP_MAP: Record<string, string> = {
  "==": "==", "===": "==",
  "!=": "!=", "!==": "!=",
  ">": ">", ">=": ">=", "<": "<", "<=": "<=",
};

function normalizeVisibleOp(op: any): string {
  return VISIBLE_WHEN_OP_MAP[String(op == null ? "==" : op)] || "==";
}

function compareValues(a: any, b: any): number {
  const na = Number(a);
  const nb = Number(b);
  if (!Number.isNaN(na) && !Number.isNaN(nb)) return na - nb;
  return String(a ?? "").localeCompare(String(b ?? ""));
}

export function evalVisibleWhen(
  rule: any,
  form: Record<string, any> | undefined,
): boolean {
  if (!rule) return true;
  const list = Array.isArray(rule) ? rule : [rule];
  if (!list.length) return true;
  return (list as Array<any>).every((c: any) => {
    const op = normalizeVisibleOp(c.op);
    const actual = form ? form[c.field] : undefined;
    const target = c.value;
    switch (op) {
      case "!=": return String(actual ?? "") !== String(target ?? "");
      case ">": return compareValues(actual, target) > 0;
      case ">=": return compareValues(actual, target) >= 0;
      case "<": return compareValues(actual, target) < 0;
      case "<=": return compareValues(actual, target) <= 0;
      case "==":
      default: return String(actual ?? "") === String(target ?? "");
    }
  });
}

export type PluginConfigFieldVisibleField = { key: string; prop: any };

export type PluginConfigFieldVisibleCtx = {
  form: Record<string, any> | undefined;
  schemaProperties: Record<string, any> | undefined;
};

export function pluginConfigFieldVisible(
  field: PluginConfigFieldVisibleField,
  ctx: PluginConfigFieldVisibleCtx,
): boolean {
  const prop = (field && field.prop) || {};
  // 通用：schema 声明的条件可见规则（ui:visibleWhen），由插件自行声明，不再依赖前端硬编码特例。
  // 规则格式：{ field, op: "=="|"!="|">"|">="|"<"|"<=", value } 或这类对象的数组（多条件 AND）。
  if (prop["ui:visibleWhen"] !== undefined) {
    return evalVisibleWhen(prop["ui:visibleWhen"], ctx.form);
  }
  // 以下为旧版硬编码兼容分支，保留以兼容历史插件（未来可移除）。
  const properties = ctx.schemaProperties || {};
  if (Object.prototype.hasOwnProperty.call(properties, "sync_panel")) {
    const syncPanel = String(ctx.form?.sync_panel || "");
    if (field.key === "qinglong_id") return syncPanel === "qinglong";
    if (field.key === "daidai_id") return syncPanel === "daidai";
  }
  if (!Object.prototype.hasOwnProperty.call(properties, "account_mode"))
    return true;
  const title = String(field.prop?.title || "");
  const description = String(field.prop?.description || "");
  const isManualOnly =
    field.key === "openid" ||
    field.key === "manual_openids" ||
    field.key === "accounts_json" ||
    /手动\s*openid/i.test(title) ||
    /仅手动填写模式生效/.test(description);
  return !isManualOnly || String(ctx.form?.account_mode || "") === "manual";
}
