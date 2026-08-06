package core

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/smallfawn/sillyGirl/utils"
)

var nodeRuntimePreloadCache sync.Map

func ensureNodeRuntimePreload() (string, error) {
	dir := filepath.Join(utils.ExecPath, "language", "node")
	path := filepath.Join(dir, "sillygirl-runtime-preload.js")
	if _, ok := nodeRuntimePreloadCache.Load(path); ok {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	if err := writeFileIfChanged(path, []byte(nodeRuntimePreloadScript), 0644); err != nil {
		return "", err
	}
	nodeRuntimePreloadCache.Store(path, true)
	return path, nil
}

const nodeRuntimePreloadScript = `
(function () {
  if (process.env.SILLYGIRL_CONFIG_REGISTER_ONLY === "true") {
    const fs = require("fs");
    const expectedPlugin = process.env.SILLYGIRL_EXPECT_PLUGIN_FORM === "true";
    const expectedUser = process.env.SILLYGIRL_EXPECT_USER_FORM === "true";
    const exported = { plugin: null, user: null };
    function isSchemaNode(value) { return !!(value && value.__schemaNode && value.schema); }
    function normalizeFormField(value, path) {
      if (isSchemaNode(value)) return value.schema;
      throw new Error("Form schema " + (path || "field") + " must use .Form.string()/.Form.boolean()/.Form.select() helpers");
    }
    function normalizeSchema(fields) {
      if (!fields || typeof fields !== "object" || Array.isArray(fields) || isSchemaNode(fields)) throw new Error("Form(...) only accepts an object of schema fields");
      const properties = {};
      for (const key of Object.keys(fields)) if (!key.startsWith("_")) properties[key] = normalizeFormField(fields[key], key);
      return { type: "object", properties, propertyOrder: Object.keys(properties) };
    }
    function defaults(schema) {
      schema = isSchemaNode(schema) ? schema.schema : (schema || {});
      if (Object.prototype.hasOwnProperty.call(schema, "default")) return schema.default;
      if (schema.type === "object" || schema.properties) {
        const result = {};
        for (const key of Object.keys(schema.properties || {})) { const value = defaults(schema.properties[key]); if (value !== undefined) result[key] = value; }
        return result;
      }
      if (schema.type === "array") return [];
      return undefined;
    }
    function SchemaNode(type, extra) { this.__schemaNode = true; this.schema = Object.assign({ type }, extra || {}); this.validators = []; this.lastRule = ""; }
    SchemaNode.prototype.title = function (value) { this.schema.title = value; return this; };
    SchemaNode.prototype.description = function (value) { this.schema.description = value; return this; };
    SchemaNode.prototype.default = function (value) { this.schema.default = value; return this; };
    SchemaNode.prototype.options = function (value) { return applyOptions(this, value); };
    SchemaNode.prototype.required = function (value) { this.schema.required = value === undefined ? true : !!value; this.lastRule = "required"; return this; };
    SchemaNode.prototype.match = function (value) { this.schema.pattern = value instanceof RegExp ? value.source : String(value || ""); this.lastRule = "match"; return this; };
    SchemaNode.prototype.test = function (callback) { if (typeof callback !== "function") throw new Error("test() requires a function"); this.validators.push({ runtime: "node", source: callback.toString(), message: "" }); this.lastRule = "test"; return this; };
    SchemaNode.prototype.err = function (value) { if (!this.lastRule) throw new Error("err() must follow required(), match() or test()"); if (this.lastRule === "test") this.validators[this.validators.length - 1].message = String(value || ""); else (this.schema.errorMessages ||= {})[this.lastRule] = String(value || ""); return this; };
    SchemaNode.prototype.format = function (value) { this.schema.format = value; return this; };
    SchemaNode.prototype.min = function (value) { this.schema.minimum = value; return this; };
    SchemaNode.prototype.max = function (value) { this.schema.maximum = value; return this; };
    SchemaNode.prototype.widget = function (value) { this.schema["ui:widget"] = value; return this; };
    SchemaNode.prototype.toJSON = function () { return this.schema; };
    function applyOptions(node, options) {
      const values = [], names = [];
      if (Array.isArray(options)) for (const item of options) {
        const value = item && typeof item === "object" ? (Object.prototype.hasOwnProperty.call(item, "value") ? item.value : (item.id ?? item.key ?? item.name ?? item.label)) : item;
        values.push(value); names.push(String(item && typeof item === "object" ? (item.label ?? item.name ?? value) : value));
      }
      else if (options && typeof options === "object") for (const key of Object.keys(options)) { values.push(key); names.push(String(options[key])); }
      node.schema.enum = values; if (names.some((name, i) => name !== String(values[i]))) node.schema.enumNames = names; return node;
    }
    const helpers = {
      string: () => new SchemaNode("string"), number: () => new SchemaNode("number"), integer: () => new SchemaNode("integer"), boolean: () => new SchemaNode("boolean"),
      array: item => new SchemaNode("array", item === undefined ? {} : { items: normalizeFormField(item, "array item") }),
      object: props => new SchemaNode("object", { properties: Object.fromEntries(Object.entries(props || {}).map(([key, value]) => [key, normalizeFormField(value, key)])) }),
      select: options => applyOptions(new SchemaNode("string"), options),
    };
    let finishScheduled = false;
    function finishIfReady() {
      if ((expectedPlugin && !exported.plugin) || (expectedUser && !exported.user)) return;
      const target = process.env.SILLYGIRL_CONFIG_SCHEMA_FILE || "";
      const data = JSON.stringify(exported);
      if (target) fs.writeFileSync(target, data); else console.log("__SILLYGIRL_CONFIG_SCHEMA__" + data);
      process.exit(0);
    }
    function scheduleFinish() { if (finishScheduled) return; finishScheduled = true; process.nextTick(function () { finishScheduled = false; finishIfReady(); }); }
    function PluginForm(schema) { exported.plugin = normalizeSchema(schema); scheduleFinish(); return this; }
    function UserForm(fields) { const validators = {}; for (const key of Object.keys(fields || {})) if (isSchemaNode(fields[key]) && fields[key].validators.length) validators[key] = fields[key].validators; this.definition = { schema: normalizeSchema(fields), multiple: 1, key_by: [], validators }; exported.user = this.definition; scheduleFinish(); }
    UserForm.prototype.multiple = function (limit) { this.definition.multiple = Math.max(1, Number(limit || 1) | 0); scheduleFinish(); return this; };
    UserForm.prototype.keyBy = function (fields) { this.definition.key_by = (Array.isArray(fields) ? fields : [fields]).map(String); scheduleFinish(); return this; };
    const pluginForm = Object.assign(function (schema) { return new PluginForm(schema); }, helpers, { defaults: fields => defaults(normalizeSchema(fields)) });
    const userForm = Object.assign(function (schema) { return new UserForm(schema); }, helpers);
    const dummy = new Proxy(function () {}, { get: () => dummy, apply: () => dummy, construct: () => dummy });
    const sg = {
      Adapter: dummy, Bucket: dummy, sender: dummy, container: dummy, console: dummy,
      plugin: { Form: pluginForm },
      user: { Form: userForm, getUserList: async () => [], getUser: async () => null },
      utils: { sleep: async () => {}, version: async () => ({}), restart: async () => ({}), update: async () => ({}), buildCQTag: () => "", parseCQText: () => [], image: url => "[CQ:image,url=" + String(url || "") + "]", video: url => "[CQ:video,url=" + String(url || "") + "]" },
    };
    const Module = require("module");
    const originalLoad = Module._load;
    Module._load = function (request, parent, isMain) {
      if (request === "sillygirl") return sg;
      try {
        return originalLoad.apply(this, arguments);
      } catch (err) {
        if (err && err.code === "MODULE_NOT_FOUND") return dummy;
        throw err;
      }
    };
    return;
  }
  let sg;
  try {
    sg = require(require("path").join(process.cwd(), "node_modules", "sillygirl"));
  } catch (error) {
    try {
      sg = require("sillygirl");
    } catch (_) {
      sg = {};
    }
  }
  const Module = require("module");
  const originalLoad = Module._load;
  Module._load = function (request, parent, isMain) {
    if (request === "sillygirl") return sg;
    return originalLoad.apply(this, arguments);
  };
})();
`
