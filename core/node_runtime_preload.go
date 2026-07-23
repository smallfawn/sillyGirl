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
    sg = require("sillygirl");
  } catch (error) {
    return;
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

  class qinglong {
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

  class smallcat {
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
  }

  sg.qinglong = sg.qinglong || qinglong;
  sg.smallcat = sg.smallcat || smallcat;
  globalThis.qinglong = sg.qinglong;
  globalThis.smallcat = sg.smallcat;
})();
`
