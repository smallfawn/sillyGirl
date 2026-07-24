package core

import (
	"os"
	"path/filepath"

	"github.com/smallfawn/sillyGirl/utils"
)

func ensureNodeRuntimePreload() (string, error) {
	dir := filepath.Join(utils.ExecPath, "language", "node")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "sillygirl-runtime-preload.js")
	if err := os.WriteFile(path, []byte(nodeRuntimePreloadScript), 0644); err != nil {
		return "", err
	}
	return path, nil
}

const nodeRuntimePreloadScript = `
(function () {
  let sg;
  try {
    sg = require(require("path").join(process.cwd(), "node_modules", "sillygirl"));
  } catch (error) {
    try {
      sg = require("sillygirl");
    } catch (_) {
      sg = globalThis.sillygirl || {};
    }
  }
  const Bucket = sg && sg.Bucket;
  if (!Bucket) return;

  async function readPanels(key) {
    const raw = await new Bucket("sillyGirl").get(key, []);
    if (Array.isArray(raw)) return raw;
    if (typeof raw === "string" && raw.trim()) {
      const text = raw.startsWith("o:") ? raw.slice(2) : raw;
      try {
        const panels = JSON.parse(text);
        return Array.isArray(panels) ? panels : [];
      } catch (error) {
        return [];
      }
    }
    return [];
  }

  function panelIndex(ref) {
    const index = Number(ref && typeof ref === "object" ? ref.id ?? ref.ID : ref);
    return Number.isInteger(index) ? index : 0;
  }

  function queryString(query) {
    const values = new URLSearchParams();
    for (const key of Object.keys(query || {})) {
      if (query[key] !== undefined && query[key] !== null) values.set(key, String(query[key]));
    }
    const encoded = values.toString();
    return encoded ? "?" + encoded : "";
  }

  function apiPath(path, prefix) {
    path = String(path || "").trim();
    if (!path) path = prefix;
    if (!path.startsWith("/")) path = "/" + path;
    if (prefix && !path.startsWith(prefix + "/") && path !== prefix) path = prefix + path;
    return path;
  }

  function ids(value) {
    if (Array.isArray(value)) return value;
    if (typeof value === "string") {
      const values = value.split(/[,\s]+/).map((item) => item.trim()).filter(Boolean).map((item) => Number.isNaN(Number(item)) ? item : Number(item));
      if (values.length) return values;
    }
    return [value];
  }

  function normalizeSchema(value) {
    if (value && value.__schemaNode && value.schema) return value.schema;
    if (value && typeof value.toJSON === "function") return value.toJSON();
    if (Array.isArray(value)) return value.map((item) => normalizeSchema(item));
    if (value && typeof value === "object") {
      const result = {};
      for (const key of Object.keys(value)) {
        if (key.startsWith("_") || key === "__schemaNode") continue;
        result[key] = normalizeSchema(value[key]);
      }
      return result;
    }
    return value;
  }

  function collectSchemaDefaults(schema) {
    schema = normalizeSchema(schema) || {};
    if (Object.prototype.hasOwnProperty.call(schema, "default")) return schema.default;
    if (schema.type === "object" || schema.properties) {
      const values = {};
      for (const key of Object.keys(schema.properties || {})) {
        const value = collectSchemaDefaults(schema.properties[key]);
        if (value !== undefined) values[key] = value;
      }
      return values;
    }
    if (schema.type === "array") return [];
    return undefined;
  }

  function SchemaNode(type, extra) {
    this.__schemaNode = true;
    this.schema = Object.assign({ type: type }, extra || {});
  }
  SchemaNode.prototype.setTitle = function (value) { this.schema.title = value; return this; };
  SchemaNode.prototype.setDescription = function (value) { this.schema.description = value; return this; };
  SchemaNode.prototype.setDefault = function (value) { this.schema.default = value; return this; };
  SchemaNode.prototype.setEnum = function (value) { this.schema.enum = value; return this; };
  SchemaNode.prototype.setEnumNames = function (value) { this.schema.enumNames = value; return this; };
  SchemaNode.prototype.setRequired = function (value) { this.schema.required = value; return this; };
  SchemaNode.prototype.setFormat = function (value) { this.schema.format = value; return this; };
  SchemaNode.prototype.setMin = function (value) { this.schema.minimum = value; return this; };
  SchemaNode.prototype.setMax = function (value) { this.schema.maximum = value; return this; };
  SchemaNode.prototype.setMinLength = function (value) { this.schema.minLength = value; return this; };
  SchemaNode.prototype.setMaxLength = function (value) { this.schema.maxLength = value; return this; };
  SchemaNode.prototype.setPattern = function (value) { this.schema.pattern = value; return this; };
  SchemaNode.prototype.setWidget = function (value) { this.schema["ui:widget"] = value; return this; };
  SchemaNode.prototype.toJSON = function () { return this.schema; };

  const sillyGirlCreateSchema = {
    string: function () { return new SchemaNode("string"); },
    number: function () { return new SchemaNode("number"); },
    integer: function () { return new SchemaNode("integer"); },
    boolean: function () { return new SchemaNode("boolean"); },
    array: function (item) { return new SchemaNode("array", { items: normalizeSchema(item) || {} }); },
    object: function (props) {
      const properties = {};
      for (const key of Object.keys(props || {})) properties[key] = normalizeSchema(props[key]);
      return new SchemaNode("object", { properties });
    },
  };

  class SillyGirlPluginConfig {
    constructor(schema) {
      this.uuid = process.env.PLUGIN_ID || "";
      this.jsonSchema = normalizeSchema(schema) || {};
      if (!this.jsonSchema.type) this.jsonSchema.type = "object";
      this.userConfig = {};
      if (process.env.PLUGIN_CONFIG_JSON) {
        try {
          const value = JSON.parse(process.env.PLUGIN_CONFIG_JSON);
          if (value && typeof value === "object" && !Array.isArray(value)) this.userConfig = value;
        } catch (_) {}
      }
      this.ready = this.init();
    }
    async init() {
      if (!this.uuid) return this.userConfig;
      await new Bucket("plugin_config_schemas").set(this.uuid, this.jsonSchema);
      this.userConfig = await new Bucket("plugin_config_values").get(this.uuid, {});
      return this.userConfig;
    }
    async get() {
      await this.ready;
      this.userConfig = await new Bucket("plugin_config_values").get(this.uuid, {});
      return this.userConfig;
    }
    async Get() {
      return this.get();
    }
    async set(values) {
      await this.ready;
      if (values && typeof values === "object") this.userConfig = values;
      await new Bucket("plugin_config_values").set(this.uuid, this.userConfig || {});
      return { error: "" };
    }
    async Set(values) {
      return this.set(values);
    }
  }

  function form(schema) {
    return new SillyGirlPluginConfig(schema);
  }

  class QingLong {
    constructor(options) {
      this.id = 0;
      this.uuid = "";
      this.name = "";
      this.address = "";
      this.token = "";
      this.expiration = 0;
      this.ready = this.init(options);
    }
    async init(options) {
      const panels = await readPanels("qinglong_panels");
      const index = panelIndex(options);
      if (index < 1 || index > panels.length) throw new Error("青龙编号 " + (index || "") + " 不存在");
      this.panel = panels[index - 1];
      this.id = index;
      this.uuid = this.panel.id || "";
      this.name = this.panel.name || "";
      this.address = String(this.panel.address || "").replace(/\/+$/, "");
    }
    async ensureToken() {
      await this.ready;
      const now = Math.floor(Date.now() / 1000);
      if (this.token && this.expiration > now + 60) return;
      const resp = await fetch(this.address + "/open/auth/token" + queryString({ client_id: this.panel.client_id, client_secret: this.panel.client_secret }));
      const result = await resp.json();
      if (!resp.ok || result.code !== 200 || !result.data || !result.data.token) throw new Error(result.message || ("青龙认证失败：HTTP " + resp.status));
      this.token = result.data.token;
      this.expiration = Number(result.data.expiration || 0);
    }
    async request(method, path, body, query) {
      await this.ensureToken();
      const resp = await fetch(this.address + apiPath(path, "/open") + queryString(query), {
        method: String(method || "GET").toUpperCase(),
        headers: Object.assign({ Authorization: "Bearer " + this.token }, body == null ? {} : { "Content-Type": "application/json" }),
        body: body == null ? undefined : JSON.stringify(body),
      });
      const text = await resp.text();
      const result = text ? JSON.parse(text) : {};
      if (!resp.ok) throw new Error(result.message || ("青龙接口 HTTP " + resp.status));
      if (result.code !== undefined && result.code !== 200) throw new Error(result.message || "青龙接口请求失败");
      return result;
    }
    async getEnvs(options) { const r = await this.request("GET", "/envs", undefined, typeof options === "string" ? { searchValue: options } : options || {}); return r.data ?? r; }
    async getEnvById(id) { const r = await this.request("GET", "/envs/" + id); return r.data ?? r; }
    async createEnv(env) { const r = await this.request("POST", "/envs", Array.isArray(env) ? env : [env]); return r.data ?? r; }
    async updateEnv(env) { const r = await this.request("PUT", "/envs", env); return r.data ?? r; }
    async deleteEnvs(value) { const r = await this.request("DELETE", "/envs", ids(value)); return r.data ?? r; }
    async moveEnv(id, arg1, arg2) { const r = await this.request("PUT", "/envs/" + id + "/move", typeof arg1 === "object" ? arg1 : { fromIndex: arg1, toIndex: arg2 }); return r.data ?? r; }
    async disableEnvs(value) { const r = await this.request("PUT", "/envs/disable", ids(value)); return r.data ?? r; }
    async enableEnvs(value) { const r = await this.request("PUT", "/envs/enable", ids(value)); return r.data ?? r; }
    async updateEnvNames(arg1, arg2) { const r = await this.request("PUT", "/envs/name", typeof arg1 === "object" && arg2 === undefined ? arg1 : { ids: ids(arg1), name: arg2 }); return r.data ?? r; }
    async systemNotify(title, content) { const r = await this.request("PUT", "/system/notify", { title, content }); return r.data ?? r; }
  }

  class SmallCat {
    constructor(options) {
      this.id = 0;
      this.uuid = "";
      this.name = "";
      this.address = "";
      this.ready = this.init(options);
    }
    async init(options) {
      const panels = await readPanels("smallcat_panels");
      const index = panelIndex(options);
      if (index < 1 || index > panels.length) throw new Error("smallcat 编号 " + (index || "") + " 不存在");
      this.panel = panels[index - 1];
      this.id = index;
      this.uuid = this.panel.id || "";
      this.name = this.panel.name || "";
      this.address = String(this.panel.address || "").replace(/\/+$/, "");
    }
    async request(method, path, body, query) {
      await this.ready;
      const resp = await fetch(this.address + apiPath(path, "") + queryString(query), {
        method: String(method || "GET").toUpperCase(),
        headers: Object.assign({ auth: this.panel.api_auth || "" }, body == null ? {} : { "Content-Type": "application/json" }),
        body: body == null ? undefined : JSON.stringify(body),
      });
      const text = await resp.text();
      return text ? JSON.parse(text) : {};
    }
    createQr(type) { return this.request("POST", "/api/qr/start", type && typeof type === "object" ? type : { type }); }
    checkQr(uuid) { return this.request("GET", "/api/qr/status", undefined, { uuid }); }
    addUser(options) { return this.request("POST", "/api/accounts/add", options || {}); }
    userList() { return this.request("GET", "/api/accounts"); }
    getCode(options) {
      const body = Object.assign({}, options || {});
      if (!body.openid && body.ref) body.openid = body.ref;
      if (!body.appid) body.appid = body.app_id || body.target_appid;
      return this.request("POST", "/wx/code", body);
    }
  }

  class DaiDai {
    constructor(options) {
      this.id = 0;
      this.uuid = "";
      this.name = "";
      this.address = "";
      this.token = "";
      this.expiration = 0;
      this.ready = this.init(options);
    }
    async init(options) {
      const panels = await readPanels("daidai_panels");
      const index = panelIndex(options);
      if (index < 1 || index > panels.length) throw new Error("呆呆面板编号 " + (index || "") + " 不存在");
      this.panel = panels[index - 1];
      this.id = index;
      this.uuid = this.panel.id || "";
      this.name = this.panel.name || "";
      this.address = String(this.panel.address || "").replace(/\/+$/, "");
    }
    async ensureToken() {
      await this.ready;
      const now = Math.floor(Date.now() / 1000);
      if (this.token && this.expiration > now + 60) return;
      const resp = await fetch(this.address + "/api/open-api/token", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ app_key: this.panel.app_key, app_secret: this.panel.app_secret }),
      });
      const result = await resp.json();
      const data = result.data || {};
      if (!resp.ok || !data.access_token) throw new Error(result.message || result.error || ("呆呆面板认证失败：HTTP " + resp.status));
      this.token = data.access_token;
      this.expiration = now + Number(data.expires_in || 86400);
    }
    async request(method, path, body, query) {
      await this.ensureToken();
      const resp = await fetch(this.address + apiPath(path, "/api") + queryString(query), {
        method: String(method || "GET").toUpperCase(),
        headers: Object.assign({ Authorization: "Bearer " + this.token }, body == null ? {} : { "Content-Type": "application/json" }),
        body: body == null ? undefined : JSON.stringify(body),
      });
      const text = await resp.text();
      const result = text ? JSON.parse(text) : {};
      if (!resp.ok) throw new Error(result.message || result.error || ("呆呆面板接口 HTTP " + resp.status));
      if (result.success === false) throw new Error(result.message || result.error || "呆呆面板接口请求失败");
      return result;
    }
    async getEnvs(options) { const r = await this.request("GET", "/envs", undefined, typeof options === "string" ? { keyword: options } : options || {}); return r.data ?? r; }
    async getEnvById(id) { const r = await this.request("GET", "/envs/" + id); return r.data ?? r; }
    async createEnv(env) { const r = await this.request("POST", "/envs", env); return r.data ?? r; }
    async updateEnv(env) {
      const id = env && (env.id ?? env.ID);
      const body = Object.assign({}, env || {});
      delete body.id;
      delete body.ID;
      const r = await this.request("PUT", id ? "/envs/" + id : "/envs", body);
      return r.data ?? r;
    }
    async deleteEnv(id) { return this.request("DELETE", "/envs/" + id); }
    async deleteEnvs(value) { return this.request("DELETE", "/envs/batch", { ids: ids(value) }); }
    async enableEnv(id) { const r = await this.request("PUT", "/envs/" + id + "/enable"); return r.data ?? r; }
    async disableEnv(id) { const r = await this.request("PUT", "/envs/" + id + "/disable"); return r.data ?? r; }
    async enableEnvs(value) { return this.request("PUT", "/envs/batch/enable", { ids: ids(value) }); }
    async disableEnvs(value) { return this.request("PUT", "/envs/batch/disable", { ids: ids(value) }); }
    async getTasks(options) { const r = await this.request("GET", "/tasks", undefined, typeof options === "string" ? { keyword: options } : options || {}); return r.data ?? r; }
    async getTaskById(id) { const r = await this.request("GET", "/tasks/" + id); return r.data ?? r; }
    async createTask(task) { const r = await this.request("POST", "/tasks", task); return r.data ?? r; }
    async updateTask(task) {
      const id = task && (task.id ?? task.ID);
      const body = Object.assign({}, task || {});
      delete body.id;
      delete body.ID;
      const r = await this.request("PUT", id ? "/tasks/" + id : "/tasks", body);
      return r.data ?? r;
    }
    async deleteTask(id) { return this.request("DELETE", "/tasks/" + id); }
    async runTask(id) { return this.request("PUT", "/tasks/" + id + "/run"); }
    async stopTask(id) { return this.request("PUT", "/tasks/" + id + "/stop"); }
    async enableTask(id) { return this.request("PUT", "/tasks/" + id + "/enable"); }
    async disableTask(id) { return this.request("PUT", "/tasks/" + id + "/disable"); }
    async systemNotify(title, content) { return this.request("POST", "/notifications/send", { title, content }); }
  }

  sg.QingLong = sg.QingLong || QingLong;
  sg.SmallCat = sg.SmallCat || SmallCat;
  sg.DaiDai = sg.DaiDai || DaiDai;
  sg.sillyGirlCreateSchema = sg.sillyGirlCreateSchema || sillyGirlCreateSchema;
  sg.SillyGirlPluginConfig = sg.SillyGirlPluginConfig || SillyGirlPluginConfig;
  sg.form = sg.form || form;
  sg.pluginConfigDefaults = sg.pluginConfigDefaults || collectSchemaDefaults;
  if (!sg.express) {
    Object.defineProperty(sg, "express", {
      enumerable: true,
      configurable: true,
      get: function () { return require("express"); },
    });
  }
  globalThis.QingLong = sg.QingLong;
  globalThis.SmallCat = sg.SmallCat;
  globalThis.DaiDai = sg.DaiDai;
  globalThis.sillyGirlCreateSchema = sg.sillyGirlCreateSchema;
  globalThis.SillyGirlPluginConfig = sg.SillyGirlPluginConfig;
  globalThis.form = sg.form;
  globalThis.pluginConfigDefaults = sg.pluginConfigDefaults;
  Object.defineProperty(globalThis, "express", {
    enumerable: true,
    configurable: true,
    get: function () { return sg.express; },
  });
})();
`
