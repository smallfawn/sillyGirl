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
    function isSchemaNode(value) {
      return !!(value && value.__schemaNode && value.schema);
    }
    function normalizeFormField(value, path) {
      if (isSchemaNode(value)) return value.schema;
      throw new Error("form schema " + (path || "field") + " must use form.string()/form.boolean()/form.select() helpers");
    }
    function normalizeConfigSchema(fields) {
      if (!fields || typeof fields !== "object" || Array.isArray(fields) || isSchemaNode(fields)) {
        throw new Error("new form(...) only accepts an object like { token: form.string().title(\"Token\") }");
      }
      const properties = {};
      for (const key of Object.keys(fields)) {
        if (key.startsWith("_")) continue;
        properties[key] = normalizeFormField(fields[key], key);
      }
      return { type: "object", properties: properties };
    }
    function collectSchemaDefaults(schema) {
      schema = isSchemaNode(schema) ? schema.schema : (schema || {});
      if (!schema || typeof schema !== "object") return undefined;
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
    SchemaNode.prototype.title = function (value) { this.schema.title = value; return this; };
    SchemaNode.prototype.description = function (value) { this.schema.description = value; return this; };
    SchemaNode.prototype.default = function (value) { this.schema.default = value; return this; };
    SchemaNode.prototype.options = function (value) { return applySchemaOptions(this, value); };
    SchemaNode.prototype.required = function (value) { this.schema.required = value; return this; };
    SchemaNode.prototype.format = function (value) { this.schema.format = value; return this; };
    SchemaNode.prototype.min = function (value) { this.schema.minimum = value; return this; };
    SchemaNode.prototype.max = function (value) { this.schema.maximum = value; return this; };
    SchemaNode.prototype.minLength = function (value) { this.schema.minLength = value; return this; };
    SchemaNode.prototype.maxLength = function (value) { this.schema.maxLength = value; return this; };
    SchemaNode.prototype.pattern = function (value) { this.schema.pattern = value; return this; };
    SchemaNode.prototype.widget = function (value) { this.schema["ui:widget"] = value; return this; };
    SchemaNode.prototype.toJSON = function () { return this.schema; };
    function applySchemaOptions(node, options) {
      if (Array.isArray(options)) {
        const values = [];
        const names = [];
        for (const item of options) {
          if (item && typeof item === "object" && !Array.isArray(item)) {
            const value = Object.prototype.hasOwnProperty.call(item, "value") ? item.value : (item.id ?? item.key ?? item.name ?? item.label);
            values.push(value);
            names.push(String(item.label ?? item.name ?? value));
          } else {
            values.push(item);
            names.push(String(item));
          }
        }
        node.schema.enum = values;
        if (names.some(function (name, index) { return name !== String(values[index]); })) node.schema.enumNames = names;
        return node;
      }
      if (options && typeof options === "object") {
        const values = Object.keys(options);
        node.schema.enum = values;
        node.schema.enumNames = values.map(function (key) { return String(options[key]); });
      }
      return node;
    }
    const formHelpers = {
      string: function () { return new SchemaNode("string"); },
      number: function () { return new SchemaNode("number"); },
      integer: function () { return new SchemaNode("integer"); },
      boolean: function () { return new SchemaNode("boolean"); },
      array: function (item) { return new SchemaNode("array", item === undefined ? {} : { items: normalizeFormField(item, "array item") }); },
      object: function (props) {
        const properties = {};
        for (const key of Object.keys(props || {})) properties[key] = normalizeFormField(props[key], key);
        return new SchemaNode("object", { properties: properties });
      },
      select: function (options) { return applySchemaOptions(new SchemaNode("string"), options); },
    };
    class PluginConfigFormInstance {
      constructor(schema) {
        this.uuid = process.env.PLUGIN_ID || "";
        this.jsonSchema = normalizeConfigSchema(schema);
        try {
          const fs = require("fs");
          const target = process.env.SILLYGIRL_CONFIG_SCHEMA_FILE || "";
          if (target) fs.writeFileSync(target, JSON.stringify(this.jsonSchema));
          else console.log("__SILLYGIRL_CONFIG_SCHEMA__" + JSON.stringify(this.jsonSchema));
          process.exit(0);
        } catch (err) {
          console.error("form schema export failed:", err && err.message ? err.message : err);
          process.exit(1);
        }
      }
    }
    function form(schema) { return new PluginConfigFormInstance(schema); }
    Object.assign(form, formHelpers, { defaults: function (fields) { return collectSchemaDefaults(normalizeConfigSchema(fields)); } });
    const dummy = new Proxy(function () {}, {
      get: function () { return dummy; },
      apply: function () { return dummy; },
      construct: function () { return dummy; },
    });

    const sg = {
      Adapter: dummy,
      Bucket: dummy,
      form,
      sender: dummy,
      container: dummy,
      utils: {
        userList: async function () { return []; },
        sleep: async function () {},
        version: async function () { return {}; },
        restart: async function () { return {}; },
        update: async function () { return {}; },
        buildCQTag: function () { return ""; },
        parseCQText: function () { return []; },
        image: function (url) { return "[CQ:image,url=" + String(url || "") + "]"; },
        video: function (url) { return "[CQ:video,url=" + String(url || "") + "]"; },
      },
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
