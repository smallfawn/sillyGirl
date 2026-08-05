"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.console = exports.utils = exports.sender = exports.form = exports.container = exports.Bucket = exports.Adapter = void 0;
const srpc_1 = require("./srpc");
const grpc_1 = __importStar(require("@grpc/grpc-js"));
const util_1 = require("util");
const { execFile } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");
grpc_1.setLogVerbosity(grpc_1.logVerbosity.NONE);
let client = new srpc_1.srpc.SillyGirlServiceClient(process.env?.SILLYGIRL_GRPC_ADDR || "127.0.0.1:50051", grpc_1.credentials.createInsecure());
let senders = [];
let plugin_id = process.env?.PLUGIN_ID ?? "";
const metadata = new grpc_1.Metadata();
metadata.add("RUNTIME_ID", process.env?.RUNTIME_ID ?? "");
metadata.add("sillygirl-runtime-token", process.env?.SILLYGIRL_GRPC_TOKEN ?? "");
class Sender {
    uuid;
    destoried = false;
    constructor(uuid) {
        this.uuid = uuid;
        senders.push(this);
    }
    destroy() {
        if (this.destoried)
            return;
        this.destoried = true;
        client.SenderDestroy(new srpc_1.srpc.ReplyRequest({ uuid: sender.uuid }), metadata, (err, resp) => { });
    }
    async getUserId() {
        return new Promise((resolve, reject) => {
            client.SenderGetUserId(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async getUserName() {
        return new Promise((resolve, reject) => {
            client.SenderGetUserName(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async getChatId() {
        return new Promise((resolve, reject) => {
            client.SenderGetChatId(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async getChatName() {
        return new Promise((resolve, reject) => {
            client.SenderGetChatName(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async getMessageId() {
        return new Promise((resolve, reject) => {
            client.SenderGetMessageId(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async getPlatform() {
        return new Promise((resolve, reject) => {
            client.SenderGetPlatform(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async getBotId() {
        return new Promise((resolve, reject) => {
            client.SenderGetBotId(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async getContent() {
        return new Promise((resolve, reject) => {
            client.SenderGetContent(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async isAdmin() {
        return new Promise((resolve, reject) => {
            client.SenderIsAdmin(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? false);
                }
            });
        });
    }
    async param(key) {
        return new Promise((resolve, reject) => {
            client.SenderParam(new srpc_1.srpc.ReplyRequest({
                uuid: this.uuid,
                content: `${key}`,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async setContent(content) {
        return new Promise((resolve, reject) => {
            client.SenderSetContent(new srpc_1.srpc.SenderContentRequest({
                uuid: this.uuid,
                content,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(undefined);
                }
            });
        });
    }
    async continue() {
        return new Promise((resolve, reject) => {
            client.SenderContinue(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(undefined);
                }
            });
        });
    }
    async getAdapter() {
        return new Adapter({
            bot_id: await this.getBotId(),
            platform: await this.getPlatform(),
        });
    }
    async listen(options) {
        return new Promise(async (resolve, reject) => {
            let params = {
                uuid: this.uuid,
                rules: options?.rules,
                timeout: options?.timeout,
                listen_private: options?.listen_private,
                listen_group: options?.listen_group,
                allow_platforms: options?.allow_platforms ?? [],
                prohibit_platforms: options?.prohibit_platforms ?? [],
                allow_groups: options?.allow_groups,
                prohibit_groups: options?.prohibit_groups,
                allow_users: options?.allow_users,
                prohibit_users: options?.prohibit_users,
                plugin_id,
            };
            if (!this.uuid) {
                params.persistent = true;
            }
            const call = client.SenderListen(metadata);
            call.on("data", (response) => {
                if (response.echo == "END") {
                    call.cancel();
                    return;
                }
                let s = response.uuid ? new Sender(response.uuid) : undefined;
                if (options?.handle && s) {
                    try {
                        let obj = options?.handle(s);
                        if (typeof obj == "string") {
                            call.write(new srpc_1.srpc.SenderListenRequest({
                                uuid: response.echo,
                                value: obj,
                            }));
                        }
                        else if (obj) {
                            obj
                                .then((v) => {
                                call.write(new srpc_1.srpc.SenderListenRequest({
                                    uuid: response.echo,
                                    value: v ?? "",
                                }));
                            })
                                .catch((e) => {
                                console.error(e);
                                call.write(new srpc_1.srpc.SenderListenRequest({
                                    uuid: response.echo,
                                    value: "",
                                }));
                            });
                        }
                        else {
                            call.write(new srpc_1.srpc.SenderListenRequest({
                                uuid: response.echo,
                                value: "",
                            }));
                        }
                    }
                    catch (e) {
                        console.error(e);
                    }
                }
                else {
                    call.write(new srpc_1.srpc.SenderListenRequest({
                        uuid: response.echo,
                        value: "",
                    }));
                }
                resolve(s);
            });
            call.on("error", (err) => {
                reject(err);
            });
            call.write(new srpc_1.srpc.SenderListenRequest(params));
        });
    }
    holdOn(str) {
        return "go_again_" + str;
    }
    async reply(content) {
        return new Promise((resolve, reject) => {
            client.SenderReply(new srpc_1.srpc.ReplyRequest({
                uuid: this.uuid,
                content,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async doAction(options) {
        return new Promise((resolve, reject) => {
            client.SenderAction(new srpc_1.srpc.ReplyRequest({
                uuid: this.uuid,
                content: JSON.stringify(options),
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(JSON.parse(resp?.value ?? "{}"));
                }
            });
        });
    }
    async getEvent() {
        return new Promise((resolve, reject) => {
            client.SenderEvent(new srpc_1.srpc.SenderRequest({
                uuid: this.uuid,
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(JSON.parse(resp?.value ?? "{}"));
                }
            });
        });
    }
    async pushAdmin(content, options = {}) {
        return pushAdmin(content, options);
    }
}
class Bucket {
    name;
    constructor(name) {
        this.name = name;
    }
    transform(v) {
        if (!v) {
            return undefined;
        }
        let result;
        if (v.startsWith("f:")) {
            result = parseFloat(v.replace("f:", ""));
            return result;
        }
        if (v.startsWith("d:")) {
            result = parseInt(v.replace("d:", ""));
            return result;
        }
        if (v.startsWith("b:")) {
            result = v.replace("b:", "") === "true";
            return result;
        }
        if (v.startsWith("o:")) {
            result = JSON.parse(v.replace("o:", ""));
            return result;
        }
        return v;
    }
    reverseTransform(value) {
        if (typeof value === "number") {
            if (value % 1 === 0) {
                return `d:${value}`;
            }
            return `f:${value}`;
        }
        if (typeof value === "boolean") {
            return `b:${value}`;
        }
        if (typeof value === "object" && value !== null) {
            return "o:" + JSON.stringify(value);
        }
        if (!value) {
            return "";
        }
        return value;
    }
    async get(key, defaultValue = undefined) {
        return new Promise((resolve, reject) => {
            client.BucketGet(new srpc_1.srpc.BucketKeyRequest({ name: this.name, key }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(this.transform(resp?.value) || defaultValue);
                }
            });
        });
    }
    async set(key, value) {
        return new Promise((resolve, reject) => {
            client.BucketSet(new srpc_1.srpc.BucketSetRequest({
                name: this.name,
                key,
                value: this.reverseTransform(value),
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve({
                        message: resp?.message,
                        changed: resp?.changed,
                    });
                }
            });
        });
    }
    async getAll() {
        return new Promise((resolve, reject) => {
            client.BucketGetAll(new srpc_1.srpc.BucketRequest({ name: this.name }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    let values = {};
                    if (resp?.value) {
                        values = JSON.parse(resp?.value);
                        for (let key in values) {
                            values[key] = this.transform(values[key]);
                        }
                    }
                    resolve(values);
                }
            });
        });
    }
    async delete(key) {
        return this.set(key, "");
    }
    async deleteAll() {
        return new Promise((resolve, reject) => {
            client.BucketDelete(new srpc_1.srpc.BucketRequest({ name: this.name }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(undefined);
                }
            });
        });
    }
    async keys() {
        return new Promise((resolve, reject) => {
            client.BucketKeys(new srpc_1.srpc.BucketRequest({ name: this.name }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.keys ?? []);
                }
            });
        });
    }
    async len() {
        return new Promise((resolve, reject) => {
            client.BucketLen(new srpc_1.srpc.BucketRequest({ name: this.name }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.length ?? 0);
                }
            });
        });
    }
    async buckets() {
        return new Promise((resolve, reject) => {
            client.BucketBuckets(new srpc_1.srpc.Empty(), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.buckets ?? []);
                }
            });
        });
    }
    watch(key, handle) {
        const call = client.BucketWatch(metadata);
        call.on("data", async (response) => {
            let fin = handle(this.transform(response.old), this.transform(response.now), response.key);
            try {
                fin = await fin;
            }
            catch (e) {
                console.error(e);
            }
            let result = {
                echo: response.echo,
            };
            if (!fin) {
                result.error = "VOID";
            }
            else {
                result.now = this.reverseTransform(fin.now);
                result.message = fin.message;
                result.error = fin.error;
            }
            call.write(new srpc_1.srpc.BucketWatchRequest(result));
        });
        call.on("error", (err) => {
            // console.error(err);
        });
        call.write(new srpc_1.srpc.BucketWatchRequest({
            name: this.name,
            key: key,
            plugin_id,
        }));
    }
    async getName() {
        return this.name;
    }
}
exports.Bucket = Bucket;
/** Read ordinary users and this plugin's authorization state. */
async function userList() {
    return await new Bucket("__plugin_users__").get("list", []);
}
function isSchemaNode(value) {
    return !!(value && value.__schemaNode && value.schema);
}
function normalizeFormField(value, path = "field") {
    if (isSchemaNode(value))
        return value.schema;
    throw new Error(`form schema ${path} must use form.string()/form.boolean()/form.select() helpers`);
}
function normalizeConfigSchema(fields) {
    if (!fields || typeof fields !== "object" || Array.isArray(fields) || isSchemaNode(fields)) {
        throw new Error("new form(...) only accepts an object like { token: form.string().title(\"Token\") }");
    }
    const properties = {};
    for (const key of Object.keys(fields)) {
        if (key.startsWith("_"))
            continue;
        properties[key] = normalizeFormField(fields[key], key);
    }
    return { type: "object", properties };
}
function pluginConfigDefaults(schema) {
    const normalized = isSchemaNode(schema) ? schema.schema : schema;
    if (!normalized || typeof normalized !== "object")
        return undefined;
    if (Object.prototype.hasOwnProperty.call(normalized, "default"))
        return normalized.default;
    if (normalized.type === "object" || normalized.properties) {
        const values = {};
        for (const key of Object.keys(normalized.properties || {})) {
            const value = pluginConfigDefaults(normalized.properties[key]);
            if (value !== undefined)
                values[key] = value;
        }
        return values;
    }
    if (normalized.type === "array")
        return [];
    return undefined;
}
class SchemaNode {
    __schemaNode = true;
    schema;
    constructor(type, extra = {}) {
        this.schema = Object.assign({ type }, extra);
    }
    title(value) { this.schema.title = value; return this; }
    description(value) { this.schema.description = value; return this; }
    default(value) { this.schema.default = value; return this; }
    options(value) { return applySchemaOptions(this, value); }
    required(value) { this.schema.required = value; return this; }
    format(value) { this.schema.format = value; return this; }
    min(value) { this.schema.minimum = value; return this; }
    max(value) { this.schema.maximum = value; return this; }
    minLength(value) { this.schema.minLength = value; return this; }
    maxLength(value) { this.schema.maxLength = value; return this; }
    pattern(value) { this.schema.pattern = value; return this; }
    widget(value) { this.schema["ui:widget"] = value; return this; }
    toJSON() { return this.schema; }
}
function applySchemaOptions(node, options) {
    if (Array.isArray(options)) {
        const values = [];
        const names = [];
        for (const item of options) {
            if (item && typeof item === "object" && !Array.isArray(item)) {
                const value = Object.prototype.hasOwnProperty.call(item, "value") ? item.value : (item.id ?? item.key ?? item.name ?? item.label);
                values.push(value);
                names.push(String(item.label ?? item.name ?? value));
            }
            else {
                values.push(item);
                names.push(String(item));
            }
        }
        node.schema.enum = values;
        if (names.some((name, index) => name !== String(values[index])))
            node.schema.enumNames = names;
        return node;
    }
    if (options && typeof options === "object") {
        const values = Object.keys(options);
        node.schema.enum = values;
        node.schema.enumNames = values.map((key) => String(options[key]));
    }
    return node;
}
const formHelpers = {
    string: () => new SchemaNode("string"),
    number: () => new SchemaNode("number"),
    integer: () => new SchemaNode("integer"),
    boolean: () => new SchemaNode("boolean"),
    array: (item) => new SchemaNode("array", item === undefined ? {} : { items: normalizeFormField(item, "array item") }),
    object: (props) => {
        const properties = {};
        for (const key of Object.keys(props || {})) {
            properties[key] = normalizeFormField(props?.[key], key);
        }
        return new SchemaNode("object", { properties });
    },
    select: (options) => applySchemaOptions(new SchemaNode("string"), options),
};
class PluginConfigFormInstance {
    uuid = plugin_id;
    jsonSchema;
    userConfig = {};
    ready;
    constructor(schema) {
        this.jsonSchema = normalizeConfigSchema(schema);
        if (process.env.PLUGIN_CONFIG_JSON) {
            try {
                const value = JSON.parse(process.env.PLUGIN_CONFIG_JSON);
                if (value && typeof value === "object" && !Array.isArray(value)) {
                    this.userConfig = value;
                }
            }
            catch (_) { }
        }
        this.ready = this.init();
    }
    async init() {
        if (!this.uuid)
            return this.userConfig;
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
        if (values && typeof values === "object")
            this.userConfig = values;
        await new Bucket("plugin_config_values").set(this.uuid, this.userConfig || {});
        return { error: "" };
    }
    async Set(values) {
        return this.set(values);
    }
}
const form = Object.assign(function (fields) {
    return new PluginConfigFormInstance(fields);
}, formHelpers, { defaults: (fields) => pluginConfigDefaults(normalizeConfigSchema(fields)) });
exports.form = form;
async function readRuntimePanels(key) {
    const raw = await new Bucket("sillyGirl").get(key, []);
    if (Array.isArray(raw))
        return raw;
    if (typeof raw === "string" && raw.trim()) {
        const text = raw.startsWith("o:") ? raw.slice(2) : raw;
        try {
            const panels = JSON.parse(text);
            return Array.isArray(panels) ? panels : [];
        }
        catch {
            return [];
        }
    }
    return [];
}
function runtimePanelIndex(ref) {
    const index = Number(typeof ref === "object" && ref ? ref.id ?? ref.ID : ref);
    return Number.isInteger(index) ? index : 0;
}
const containerDefinitions = {
    smallcat: { key: "smallcat_panels", label: "smallcat" },
    qinglong: { key: "qinglong_panels", label: "青龙" },
    daidai: { key: "daidai_panels", label: "呆呆" },
};
function normalizeContainerKind(kind) {
    const value = String(kind || "").trim().toLowerCase();
    if (!value)
        return "";
    if (value === "smallcat" || value === "small_cat" || value === "sc")
        return "smallcat";
    if (value === "qinglong" || value === "qing_long" || value === "ql" || value === "青龙")
        return "qinglong";
    if (value === "daidai" || value === "dai_dai" || value === "dd" || value === "呆呆")
        return "daidai";
    throw new Error(`未知容器类型：${kind}`);
}
function publicContainerPanel(panel, index) {
    return {
        index,
        id: String(panel?.id || ""),
        name: String(panel?.name || ""),
        address: String(panel?.address || ""),
        status: String(panel?.status || ""),
        message: String(panel?.message || ""),
    };
}
function createContainerApi() {
    return {
        QingLong,
        SmallCat,
        DaiDai,
        async getList(kind) {
            const wanted = normalizeContainerKind(kind);
            const kinds = wanted ? [wanted] : ["smallcat", "qinglong", "daidai"];
            const result = {};
            for (const item of kinds) {
                const definition = containerDefinitions[item];
                const panels = await readRuntimePanels(definition.key);
                result[item] = {
                    type: item,
                    key: item,
                    label: definition.label,
                    total: panels.length,
                    list: panels.map((panel, index) => publicContainerPanel(panel, index + 1)),
                };
            }
            return wanted ? result[wanted] : result;
        },
        async count(kind) {
            const info = await this.getList(kind);
            return info.total;
        },
        async get(kind, id) {
            const info = await this.getList(kind);
            const index = runtimePanelIndex(id);
            return info.list.find((item) => item.index === index || item.id === String(id));
        },
    };
}
function normalizeRuntimePath(path, prefix) {
    path = String(path || "").trim();
    if (!path)
        path = prefix;
    if (!path.startsWith("/"))
        path = "/" + path;
    if (prefix && !path.startsWith(prefix + "/") && path !== prefix) {
        path = prefix + path;
    }
    return path;
}
function queryString(query = {}) {
    const values = new URLSearchParams();
    for (const key of Object.keys(query || {})) {
        if (query[key] !== undefined && query[key] !== null) {
            values.set(key, String(query[key]));
        }
    }
    const encoded = values.toString();
    return encoded ? "?" + encoded : "";
}
function normalizeIDs(ids) {
    if (Array.isArray(ids))
        return ids;
    if (typeof ids === "string") {
        const values = ids
            .split(/[,\s]+/)
            .map((item) => item.trim())
            .filter(Boolean)
            .map((item) => (Number.isNaN(Number(item)) ? item : Number(item)));
        if (values.length)
            return values;
    }
    return [ids];
}
class QingLong {
    id = 0;
    uuid = "";
    name = "";
    address = "";
    panel;
    token = "";
    expiration = 0;
    ready;
    constructor(options) {
        this.ready = this.init(options);
    }
    async init(options) {
        const panels = await readRuntimePanels("qinglong_panels");
        const index = runtimePanelIndex(options);
        if (index < 1 || index > panels.length) {
            throw new Error(`青龙编号 ${index || ""} 不存在`);
        }
        this.panel = panels[index - 1];
        this.id = index;
        this.uuid = this.panel.id || "";
        this.name = this.panel.name || "";
        this.address = String(this.panel.address || "").replace(/\/+$/, "");
    }
    async ensureToken() {
        await this.ready;
        const now = Math.floor(Date.now() / 1000);
        if (this.token && this.expiration > now + 60)
            return;
        const query = queryString({
            client_id: this.panel.client_id,
            client_secret: this.panel.client_secret,
        });
        const response = await fetch(`${this.address}/open/auth/token${query}`);
        const result = await response.json();
        if (!response.ok || result.code !== 200 || !result.data?.token) {
            throw new Error(result.message || `青龙认证失败：HTTP ${response.status}`);
        }
        this.token = result.data.token;
        this.expiration = Number(result.data.expiration || 0);
    }
    async request(method, path, body, query) {
        await this.ensureToken();
        const headers = { Authorization: `Bearer ${this.token}` };
        if (body !== undefined && body !== null)
            headers["Content-Type"] = "application/json";
        const response = await fetch(`${this.address}${normalizeRuntimePath(path, "/open")}${queryString(query || {})}`, {
            method: String(method || "GET").toUpperCase(),
            headers,
            body: body === undefined || body === null ? undefined : JSON.stringify(body),
        });
        const text = await response.text();
        const result = text ? JSON.parse(text) : {};
        if (!response.ok)
            throw new Error(result.message || `青龙接口 HTTP ${response.status}`);
        if (result.code !== undefined && result.code !== 200)
            throw new Error(result.message || "青龙接口请求失败");
        return result;
    }
    async getEnvs(options) {
        const query = typeof options === "string" ? { searchValue: options } : options || {};
        const result = await this.request("GET", "/envs", undefined, query);
        return result.data ?? result;
    }
    async getEnvById(id) {
        const result = await this.request("GET", `/envs/${id}`);
        return result.data ?? result;
    }
    async createEnv(env) {
        const result = await this.request("POST", "/envs", Array.isArray(env) ? env : [env]);
        return result.data ?? result;
    }
    async updateEnv(env) {
        const result = await this.request("PUT", "/envs", env);
        return result.data ?? result;
    }
    async deleteEnvs(ids) {
        const result = await this.request("DELETE", "/envs", normalizeIDs(ids));
        return result.data ?? result;
    }
    async moveEnv(id, arg1, arg2) {
        const body = typeof arg1 === "object" ? arg1 : { fromIndex: arg1, toIndex: arg2 };
        const result = await this.request("PUT", `/envs/${id}/move`, body);
        return result.data ?? result;
    }
    async disableEnvs(ids) {
        const result = await this.request("PUT", "/envs/disable", normalizeIDs(ids));
        return result.data ?? result;
    }
    async enableEnvs(ids) {
        const result = await this.request("PUT", "/envs/enable", normalizeIDs(ids));
        return result.data ?? result;
    }
    async updateEnvNames(arg1, arg2) {
        const body = typeof arg1 === "object" && arg2 === undefined ? arg1 : { ids: normalizeIDs(arg1), name: arg2 };
        const result = await this.request("PUT", "/envs/name", body);
        return result.data ?? result;
    }
    async systemNotify(title, content) {
        const result = await this.request("PUT", "/system/notify", { title, content });
        return result.data ?? result;
    }
}
function smallcatAccountOpenID(value) {
    if (!value || typeof value !== "object" || Array.isArray(value))
        return "";
    return String(value.openid ?? value.openId ?? value.open_id ?? "").trim();
}
function filterSmallcatAccountPayload(value, allowed) {
    if (Array.isArray(value)) {
        return value.map((item) => filterSmallcatAccountPayload(item, allowed)).filter((item) => item !== undefined);
    }
    if (!value || typeof value !== "object")
        return value;
    const openid = smallcatAccountOpenID(value);
    if (openid && !allowed.has(openid))
        return undefined;
    const result = {};
    for (const [key, item] of Object.entries(value)) {
        const filtered = filterSmallcatAccountPayload(item, allowed);
        if (filtered !== undefined)
            result[key] = filtered;
    }
    return result;
}
class SmallCat {
    id = 0;
    uuid = "";
    name = "";
    address = "";
    panel;
    ready;
    constructor(options) {
        this.ready = this.init(options);
    }
    async init(options) {
        const panels = await readRuntimePanels("smallcat_panels");
        const index = runtimePanelIndex(options);
        if (index < 1 || index > panels.length) {
            throw new Error(`smallcat 编号 ${index || ""} 不存在`);
        }
        this.panel = panels[index - 1];
        this.id = index;
        this.uuid = this.panel.id || "";
        this.name = this.panel.name || "";
        this.address = String(this.panel.address || "").replace(/\/+$/, "");
    }
    async request(method, path, body, query) {
        await this.ready;
        const headers = { auth: String(this.panel.api_auth || "") };
        if (body !== undefined && body !== null)
            headers["Content-Type"] = "application/json";
        const response = await fetch(`${this.address}${normalizeRuntimePath(path, "")}${queryString(query || {})}`, {
            method: String(method || "GET").toUpperCase(),
            headers,
            body: body === undefined || body === null ? undefined : JSON.stringify(body),
        });
        const text = await response.text();
        if (!String(text || "").trim())
            return {};
        try {
            return JSON.parse(text);
        }
        catch (err) {
            const start = text.indexOf("{");
            const end = text.lastIndexOf("}");
            if (start >= 0 && end > start) {
                try {
                    return JSON.parse(text.slice(start, end + 1));
                }
                catch (_) { }
            }
            throw new Error("smallcat 接口返回非 JSON：" + String(text || "").slice(0, 120));
        }
    }
    post(path, options) {
        return this.request("POST", path, Object.assign({}, options || {}));
    }
    createQr(type) {
        const body = typeof type === "object" && type !== null ? type : { type };
        return this.request("POST", "/api/qr/start", body);
    }
    checkQr(uuid) {
        return this.request("GET", "/api/qr/status", undefined, { uuid });
    }
    addUser(options) {
        return this.post("/api/accounts/add", options);
    }
    rescanUser(options) {
        return this.post("/api/accounts/rescan", options);
    }
    async authorizedUsers() {
        return await new Bucket("__plugin_smallcat_authorized__").get("records", {
            enforced: false,
            scope: "smallcat:read",
            openids: [],
            users: [],
        });
    }
    async userList() {
        const authorization = await this.authorizedUsers();
        const payload = await this.request("GET", "/api/accounts");
        if (!authorization?.enforced)
            return payload;
        const allowed = new Set((authorization.openids || []).map((item) => String(item || "").trim()).filter(Boolean));
        return filterSmallcatAccountPayload(payload, allowed);
    }
    checkUsers(options) {
        return this.post("/api/accounts/status", options);
    }
    setUserRemark(options) {
        return this.post("/api/accounts/remark", options);
    }
    setUserDisabled(options) {
        return this.post("/api/accounts/disable", options);
    }
    deleteUser(options) {
        return this.post("/api/accounts/delete", options);
    }
    proxyList() {
        return this.request("GET", "/api/proxies");
    }
    testProxy(options) {
        return this.post("/api/proxies/test", options);
    }
    addProxy(options) {
        return this.post("/api/proxies/add", options);
    }
    deleteProxy(options) {
        return this.post("/api/proxies/delete", options);
    }
    creditBalance() {
        return this.request("GET", "/credits/balance");
    }
    creditLedger(query = { limit: 50 }) {
        return this.request("GET", "/credits/ledger", undefined, typeof query === "number" ? { limit: query } : query);
    }
    getCode(options) {
        return this.post("/wx/code", options);
    }
    getSession(options) {
        return this.post("/wx/getsession", options);
    }
    refreshSession(options) {
        return this.post("/wx/refresh", options);
    }
    getUserInfo(options) {
        return this.post("/wx/getuserinfo", options);
    }
    getEncryptKey(options) {
        return this.post("/wx/encryptkey", options);
    }
    getPhoneNumber(options) {
        return this.post("/wx/getphonenumber", options);
    }
    cloud(options) {
        return this.post("/wx/cloud", options);
    }
    gateway(options) {
        return this.post("/wx/gateway", options);
    }
    qrCodeAuth(options) {
        return this.post("/wx/qrcodeauth", options);
    }
    oAuth(options) {
        return this.post("/wx/oauth", options);
    }
    translateLink(options) {
        return this.post("/wx/translatelink", options);
    }
    autoAuth(options) {
        return this.post("/wx/autoauth", options);
    }
    appMsgExt(options) {
        return this.post("/wx/appmsgext", options);
    }
    appMsgLike(options) {
        return this.post("/wx/appmsglike", options);
    }
}
class DaiDai {
    id = 0;
    uuid = "";
    name = "";
    address = "";
    panel;
    token = "";
    expiration = 0;
    ready;
    constructor(options) {
        this.ready = this.init(options);
    }
    async init(options) {
        const panels = await readRuntimePanels("daidai_panels");
        const index = runtimePanelIndex(options);
        if (index < 1 || index > panels.length) {
            throw new Error(`呆呆面板编号 ${index || ""} 不存在`);
        }
        this.panel = panels[index - 1];
        this.id = index;
        this.uuid = this.panel.id || "";
        this.name = this.panel.name || "";
        this.address = String(this.panel.address || "").replace(/\/+$/, "");
    }
    async ensureToken() {
        await this.ready;
        const now = Math.floor(Date.now() / 1000);
        if (this.token && this.expiration > now + 60)
            return;
        const response = await fetch(`${this.address}/api/open-api/token`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ app_key: this.panel.app_key, app_secret: this.panel.app_secret }),
        });
        const result = await response.json();
        const data = result.data || {};
        if (!response.ok || !data.access_token) {
            throw new Error(result.message || result.error || `呆呆面板认证失败：HTTP ${response.status}`);
        }
        this.token = data.access_token;
        this.expiration = now + Number(data.expires_in || 86400);
    }
    async request(method, path, body, query) {
        await this.ensureToken();
        const headers = { Authorization: `Bearer ${this.token}` };
        if (body !== undefined && body !== null)
            headers["Content-Type"] = "application/json";
        const response = await fetch(`${this.address}${normalizeRuntimePath(path, "/api")}${queryString(query || {})}`, {
            method: String(method || "GET").toUpperCase(),
            headers,
            body: body === undefined || body === null ? undefined : JSON.stringify(body),
        });
        const text = await response.text();
        const result = text ? JSON.parse(text) : {};
        if (!response.ok)
            throw new Error(result.message || result.error || `呆呆面板接口 HTTP ${response.status}`);
        if (result.success === false)
            throw new Error(result.message || result.error || "呆呆面板接口请求失败");
        return result;
    }
    async getEnvs(options) {
        const query = typeof options === "string" ? { keyword: options } : options || {};
        const result = await this.request("GET", "/envs", undefined, query);
        return result.data ?? result;
    }
    async getEnvById(id) {
        const result = await this.request("GET", `/envs/${id}`);
        return result.data ?? result;
    }
    async createEnv(env) {
        const result = await this.request("POST", "/envs", env);
        return result.data ?? result;
    }
    async updateEnv(env) {
        const id = env?.id ?? env?.ID;
        const body = Object.assign({}, env || {});
        delete body.id;
        delete body.ID;
        const result = await this.request("PUT", id ? `/envs/${id}` : "/envs", body);
        return result.data ?? result;
    }
    deleteEnv(id) {
        return this.request("DELETE", `/envs/${id}`);
    }
    deleteEnvs(ids) {
        return this.request("DELETE", "/envs/batch", { ids: normalizeIDs(ids) });
    }
    async enableEnv(id) {
        const result = await this.request("PUT", `/envs/${id}/enable`);
        return result.data ?? result;
    }
    async disableEnv(id) {
        const result = await this.request("PUT", `/envs/${id}/disable`);
        return result.data ?? result;
    }
    enableEnvs(ids) {
        return this.request("PUT", "/envs/batch/enable", { ids: normalizeIDs(ids) });
    }
    disableEnvs(ids) {
        return this.request("PUT", "/envs/batch/disable", { ids: normalizeIDs(ids) });
    }
    async getTasks(options) {
        const query = typeof options === "string" ? { keyword: options } : options || {};
        const result = await this.request("GET", "/tasks", undefined, query);
        return result.data ?? result;
    }
    async getTaskById(id) {
        const result = await this.request("GET", `/tasks/${id}`);
        return result.data ?? result;
    }
    async createTask(task) {
        const result = await this.request("POST", "/tasks", task);
        return result.data ?? result;
    }
    async updateTask(task) {
        const id = task?.id ?? task?.ID;
        const body = Object.assign({}, task || {});
        delete body.id;
        delete body.ID;
        const result = await this.request("PUT", id ? `/tasks/${id}` : "/tasks", body);
        return result.data ?? result;
    }
    deleteTask(id) {
        return this.request("DELETE", `/tasks/${id}`);
    }
    runTask(id) {
        return this.request("PUT", `/tasks/${id}/run`);
    }
    stopTask(id) {
        return this.request("PUT", `/tasks/${id}/stop`);
    }
    enableTask(id) {
        return this.request("PUT", `/tasks/${id}/enable`);
    }
    disableTask(id) {
        return this.request("PUT", `/tasks/${id}/disable`);
    }
    systemNotify(title, content) {
        return this.request("POST", "/notifications/send", { title, content });
    }
}
class Adapter {
    platform;
    bot_id;
    call;
    constructor(options) {
        this.platform = options.platform;
        this.bot_id = options.bot_id;
        if (options.replyHandler) {
            const call = client.AdapterRegist(metadata);
            call.on("data", async (response) => {
                let message = JSON.parse(response.value);
                const { echo, __type__ } = message;
                delete message.__type__;
                delete message.echo;
                if (__type__ == "reply" && options.replyHandler) {
                    try {
                        let v = (await options.replyHandler(message)) ?? "";
                        call.write(new srpc_1.srpc.AdapterRegistRequest({
                            bot_id: echo,
                            platform: v,
                        }));
                    }
                    catch (e) {
                        console.error(e);
                    }
                }
                if (__type__ == "action" && options.actionHandler) {
                    try {
                        let v = await options.actionHandler(message);
                        call.write(new srpc_1.srpc.AdapterRegistRequest({
                            bot_id: echo,
                            platform: v,
                        }));
                    }
                    catch (e) {
                        console.error(e);
                    }
                }
            });
            call.on("error", (err) => {
                console.error("adapter disc", err);
            });
            call.write(new srpc_1.srpc.AdapterRegistRequest({
                bot_id: options.bot_id,
                platform: options.platform,
            }));
            this.call = call;
        }
    }
    async receive(message) {
        //投递消息
        return new Promise((resolve, reject) => {
            client.AdapterReceive(new srpc_1.srpc.AdapterRequest({
                platform: this.platform,
                bot_id: this.bot_id,
                value: JSON.stringify(message),
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(undefined);
                }
            });
        });
    }
    async push(message) {
        //推送消息
        return new Promise((resolve, reject) => {
            client.AdapterPush(new srpc_1.srpc.AdapterRequest({
                platform: this.platform,
                bot_id: this.bot_id,
                value: JSON.stringify(message),
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else {
                    resolve(resp?.value ?? "");
                }
            });
        });
    }
    async destroy() {
        this.call.cancel();
    }
    async sender(options) {
        return new Promise((resolve, reject) => {
            client.AdapterSender(new srpc_1.srpc.AdapterRequest({
                platform: this.platform,
                bot_id: this.bot_id,
                value: JSON.stringify(options),
            }), metadata, (err, resp) => {
                if (err) {
                    reject(err);
                }
                else if (resp?.value) {
                    resolve(new Sender(resp.value));
                }
            });
        });
    }
}
exports.Adapter = Adapter;
let sender = new Sender(process.env?.SENDER_ID ?? "");
exports.sender = sender;
function normalizePushAdminList(value) {
    if (Array.isArray(value)) {
        return value.map((item) => String(item || "").trim()).filter(Boolean);
    }
    return String(value || "")
        .split(/[&,\s]+/)
        .map((item) => item.trim())
        .filter(Boolean);
}
function uniqueStrings(values) {
    return Array.from(new Set(values.filter(Boolean)));
}
async function pushAdmin(content, options = {}) {
    const result = [];
    const requestedPlatforms = uniqueStrings([
        ...normalizePushAdminList(options.platform),
        ...normalizePushAdminList(options.platforms),
    ]);
    const botId = String(options.botId || options.bot_id || "").trim();
    const explicitUsers = uniqueStrings([
        ...normalizePushAdminList(options.userIds),
        ...normalizePushAdminList(options.users),
    ]);
    const platformSource = requestedPlatforms.length
        ? requestedPlatforms
        : await new Bucket("sillyGirl").buckets();
    for (const platform of uniqueStrings(platformSource)) {
        const users = explicitUsers.length
            ? explicitUsers
            : normalizePushAdminList(await new Bucket(platform).get("masters", ""));
        if (!users.length)
            continue;
        const adapter = new Adapter({ platform, bot_id: botId });
        for (const userId of users) {
            try {
                const messageId = await adapter.push({
                    user_id: userId,
                    content,
                });
                result.push({ platform, bot_id: botId, user_id: userId, message_id: messageId });
            }
            catch (error) {
                result.push({
                    platform,
                    bot_id: botId,
                    user_id: userId,
                    error: error?.message || String(error),
                });
            }
        }
    }
    return result;
}
async function sleep(ms = 1000) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
function formatRuntimeDate(date) {
    const pad = (value) => String(value).padStart(2, "0");
    return [
        date.getFullYear(),
        "-",
        pad(date.getMonth() + 1),
        "-",
        pad(date.getDate()),
        " ",
        pad(date.getHours()),
        ":",
        pad(date.getMinutes()),
        ":",
        pad(date.getSeconds()),
    ].join("");
}
function restartStamp() {
    const now = new Date();
    return `${formatRuntimeDate(now)}.${String(now.getMilliseconds()).padStart(3, "0")}`;
}
async function restart() {
    return new Bucket("sillyGirl").set("started_at", restartStamp());
}
async function version() {
    const current = String(process.env?.SILLYGIRL_VERSION || "").trim() || "unknown";
    const remote = String(process.env?.SILLYGIRL_REMOTE_VERSION || "").trim() || current;
    return {
        current,
        remote,
        source: String(process.env?.SILLYGIRL_VERSION_SOURCE || "").trim(),
        repository: String(process.env?.SILLYGIRL_REPOSITORY || "").trim(),
    };
}
async function update(options = {}) {
    const timeout = clampNumber(options.timeout || 120, 10, 600);
    return updateRelease(options, timeout);
}
async function updateRelease(options, timeout) {
    const beforeInfo = await version();
    const release = await fetchReleaseMetadata(options, timeout);
    const asset = selectReleaseAsset(release, options);
    if (!asset)
        throw new Error(`未找到适配当前系统的 Release 包：${release.tag_name || release.name || "latest"}`);
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "sillygirl-update-"));
    const archive = path.join(tmpDir, safeFileName(asset.name || "sillyGirl-release"));
    try {
        await curlDownload(releaseDownloadURLs(asset.browser_download_url), archive, timeout);
        await verifyReleaseChecksum(release, asset, archive, timeout);
        await extractReleaseArchive(archive, tmpDir, timeout);
        const executablePath = releaseExecutablePath(options);
        await installReleasePayload(tmpDir, executablePath);
        const restarted = options.restart !== false;
        if (restarted)
            await restart();
        return {
            mode: "release",
            repo: release.html_url || `release:${release.tag_name || ""}`,
            before: beforeInfo.current,
            after: normalizeVersionText(release.tag_name || release.name || ""),
            changed: true,
            output: `已通过 curl 下载并安装 Release 包：${asset.name}。${restarted ? "已触发重启，请等待 1-2 分钟后刷新页面。" : "未自动重启。"}`,
            restarted,
        };
    }
    finally {
        fs.rmSync(tmpDir, { recursive: true, force: true });
    }
}
async function fetchReleaseMetadata(options, timeout) {
    const repo = String(options.releaseRepo || process.env?.SILLYGIRL_RELEASE_REPO || "smallfawn/sillyGirl").trim();
    const tag = String(options.releaseTag || "").trim();
    const apiPath = tag ? `releases/tags/${encodeURIComponent(tag)}` : "releases/latest";
    const address = `https://api.github.com/repos/${repo}/${apiPath}`;
    const text = await curlText(releaseDownloadURLs(address), timeout, ["-H", "Accept: application/vnd.github+json"]);
    try {
        return JSON.parse(text);
    }
    catch (_) {
        throw new Error(`GitHub Release 接口返回非 JSON：${text.slice(0, 200)}`);
    }
}
function selectReleaseAsset(release, options) {
    const assets = Array.isArray(release.assets) ? release.assets : [];
    const configured = String(options.releaseAsset || "").trim();
    if (configured)
        return assets.find((asset) => asset.name === configured || String(asset.name || "").includes(configured));
    const goos = releaseGOOS();
    const goarch = releaseGOARCH();
    const suffix = goos === "windows" ? ".zip" : ".tar.gz";
    return assets.find((asset) => {
        const name = String(asset.name || "");
        return name.includes(`_${goos}_${goarch}`) && name.endsWith(suffix);
    });
}
function releaseGOOS() {
    switch (process.platform) {
        case "win32":
            return "windows";
        case "darwin":
            return "darwin";
        default:
            return "linux";
    }
}
function releaseGOARCH() {
    switch (process.arch) {
        case "x64":
            return "amd64";
        case "arm64":
            return "arm64";
        default:
            return process.arch;
    }
}
function releaseExecutablePath(options) {
    const configured = String(options.executablePath || process.env?.SILLYGIRL_EXEC_PATH || "").trim();
    if (configured)
        return path.resolve(configured);
    if (process.platform === "win32")
        return path.resolve(process.cwd(), "sillyGirl.exe");
    return "/app/sillyGirl";
}
async function verifyReleaseChecksum(release, asset, archive, timeout) {
    const checksums = (Array.isArray(release.assets) ? release.assets : []).find((item) => String(item.name || "").toLowerCase() === "checksums.txt");
    if (!checksums?.browser_download_url)
        throw new Error("Release 缺少 checksums.txt，拒绝更新");
    const text = await curlText(releaseDownloadURLs(checksums.browser_download_url), timeout);
    const expected = parseReleaseChecksum(text, asset.name);
    if (!expected)
        throw new Error(`checksums.txt 中缺少 ${asset.name} 的 SHA256`);
    const actual = sha256File(archive);
    if (actual.toLowerCase() !== expected.toLowerCase())
        throw new Error(`Release 包校验失败：${asset.name}`);
}
function parseReleaseChecksum(text, fileName) {
    for (const line of String(text || "").split(/\r?\n/)) {
        const parts = line.trim().split(/\s+/);
        const checkedName = path.basename(parts[1] ? parts[1].replace(/^\*/, "") : "");
        if (parts.length >= 2 && checkedName === fileName && /^[a-f0-9]{64}$/i.test(parts[0]))
            return parts[0];
    }
    return "";
}
function sha256File(file) {
    const hash = crypto.createHash("sha256");
    hash.update(fs.readFileSync(file));
    return hash.digest("hex");
}
async function extractReleaseArchive(archive, tmpDir, timeout) {
    await validateReleaseArchiveEntries(archive, timeout);
    await runCommand("tar", ["-xf", archive, "-C", tmpDir], timeout);
}
async function validateReleaseArchiveEntries(archive, timeout) {
    const result = await runCommand("tar", ["-tf", archive], timeout);
    for (const entry of String(result.stdout || "").split(/\r?\n/)) {
        const normalized = entry.trim().replace(/\\/g, "/");
        if (!normalized)
            continue;
        if (normalized.startsWith("/") || normalized.includes(":") || normalized.split("/").includes("..")) {
            throw new Error("Release 包包含非法路径，已拒绝解压");
        }
    }
}
async function installReleasePayload(tmpDir, executablePath) {
    const binary = findReleaseBinary(tmpDir);
    if (!binary)
        throw new Error("Release 包中没有找到 sillyGirl 可执行文件");
    const targetDir = path.dirname(executablePath);
    fs.mkdirSync(targetDir, { recursive: true });
    if (process.platform === "win32") {
        const readyPath = executablePath.replace(/\.exe$/i, ".ready.exe");
        fs.copyFileSync(binary, readyPath);
        return;
    }
    const tmpTarget = `${executablePath}.new-${Date.now()}`;
    const backup = `${executablePath}.bak-${Date.now()}`;
    fs.copyFileSync(binary, tmpTarget);
    fs.chmodSync(tmpTarget, 0o755);
    let backedUp = false;
    try {
        if (fs.existsSync(executablePath)) {
            fs.renameSync(executablePath, backup);
            backedUp = true;
        }
        fs.renameSync(tmpTarget, executablePath);
        fs.rmSync(backup, { force: true });
    }
    catch (error) {
        fs.rmSync(tmpTarget, { force: true });
        if (backedUp && fs.existsSync(backup) && !fs.existsSync(executablePath))
            fs.renameSync(backup, executablePath);
        throw error;
    }
    const proto3 = findReleaseProto3(tmpDir);
    if (proto3)
        copyDir(proto3, path.join(targetDir, "proto3"));
}
function findReleaseBinary(root) {
    const suffix = process.platform === "win32" ? ".exe" : "";
    return walkFiles(root).find((file) => {
        const name = path.basename(file);
        return name.startsWith("sillyGirl_") && (suffix ? name.endsWith(suffix) : !name.endsWith(".zip") && !name.endsWith(".tar.gz"));
    });
}
function findReleaseProto3(root) {
    return walkDirs(root).find((dir) => path.basename(dir) === "proto3" && fs.existsSync(path.join(dir, "sillygirl.js")));
}
function walkFiles(root) {
    const out = [];
    for (const item of fs.readdirSync(root, { withFileTypes: true })) {
        const full = path.join(root, item.name);
        if (item.isDirectory())
            out.push(...walkFiles(full));
        else if (item.isFile())
            out.push(full);
    }
    return out;
}
function walkDirs(root) {
    const out = [];
    for (const item of fs.readdirSync(root, { withFileTypes: true })) {
        const full = path.join(root, item.name);
        if (item.isDirectory())
            out.push(full, ...walkDirs(full));
    }
    return out;
}
function copyDir(source, target) {
    fs.mkdirSync(target, { recursive: true });
    for (const item of fs.readdirSync(source, { withFileTypes: true })) {
        const src = path.join(source, item.name);
        const dst = path.join(target, item.name);
        if (item.isDirectory())
            copyDir(src, dst);
        else if (item.isFile())
            fs.copyFileSync(src, dst);
    }
}
async function curlText(urls, timeout, extraArgs = []) {
    let lastErr;
    for (const url of urls) {
        try {
            const result = await runCommand("curl", ["-fsSL", "--retry", "2", "--connect-timeout", "8", "--max-time", String(timeout), "-H", "User-Agent: sillyGirl", ...extraArgs, url], timeout);
            return result.stdout;
        }
        catch (error) {
            lastErr = error;
        }
    }
    throw lastErr || new Error("curl 请求失败");
}
async function curlDownload(urls, target, timeout) {
    let lastErr;
    for (const url of urls) {
        try {
            await runCommand("curl", ["-fL", "--retry", "2", "--connect-timeout", "8", "--max-time", String(timeout), "-H", "User-Agent: sillyGirl", "-o", target, url], timeout);
            return;
        }
        catch (error) {
            lastErr = error;
        }
    }
    throw lastErr || new Error("curl 下载失败");
}
function releaseDownloadURLs(address) {
    const urls = [];
    for (const prefix of releaseGithubProxyPrefixes())
        urls.push(`${prefix.replace(/\/+$/, "")}/${address}`);
    urls.push(address);
    return [...new Set(urls)];
}
function releaseGithubProxyPrefixes() {
    const configured = String(process.env?.SILLYGIRL_GITHUB_PROXY || "").trim();
    const values = configured ? [configured] : [];
    return values.concat(["https://gh-proxy.org", "https://ghproxy.net", "https://cdn.gh-proxy.org"]);
}
function safeFileName(name) {
    return String(name || "release").replace(/[\\/:*?"<>|]/g, "_");
}
function normalizeVersionText(value) {
    return String(value || "").trim().replace(/^refs\/tags\//, "").replace(/^[vV]/, "");
}
function runCommand(command, args, timeoutSeconds) {
    return new Promise((resolve, reject) => {
        execFile(command, args, {
            timeout: Math.max(10, Number(timeoutSeconds || 120)) * 1000,
            windowsHide: true,
            maxBuffer: 1024 * 1024,
        }, (error, stdout, stderr) => {
            if (error) {
                error.stdout = stdout;
                error.stderr = stderr;
                reject(error);
                return;
            }
            resolve({ stdout: String(stdout || ""), stderr: String(stderr || "") });
        });
    });
}
function clampNumber(value, min, max) {
    value = Number(value || max);
    if (!Number.isFinite(value))
        return min;
    return Math.max(min, Math.min(max, Math.floor(value)));
}
class Console {
    error = (message, ...optionalParams) => { };
    info = (message, ...optionalParams) => { };
    log = (message, ...optionalParams) => { };
    debug = (message, ...optionalParams) => { };
}
let utils = {
    userList,
    sleep,
    version,
    restart,
    update,
    buildCQTag: (type, params, prefix = "CQ") => {
        const paramStrings = [];
        for (const key in params) {
            const value = params[key];
            const paramString = `${key}=${value}`;
            paramStrings.push(paramString);
        }
        const paramString = paramStrings.join(",");
        const cqString = `[${prefix}:${type}${paramString ? "," + paramString : ""}]`;
        return cqString;
    },
    parseCQText: (text, prefix = "CQ") => {
        const cqRegex = new RegExp(`\\[${prefix}:(\\w+)(.*?)\\]`, "g");
        const cqMatches = text.matchAll(cqRegex);
        const result = [];
        let lastIndex = 0;
        for (const match of cqMatches) {
            // 添加 CQ 码前的文本
            const matchIndex = text.indexOf(match[0], lastIndex);
            if (matchIndex > lastIndex) {
                result.push(text.slice(lastIndex, matchIndex));
            }
            // 解析 CQ 码
            const params = {};
            const paramRegex = /(\w+)=([^,]+)/g;
            const paramMatches = match[2].matchAll(paramRegex);
            for (const paramMatch of paramMatches) {
                params[paramMatch[1]] = paramMatch[2].trim();
            }
            result.push({
                type: match[1],
                params: params,
            });
            lastIndex = matchIndex + match[0].length;
        }
        if (lastIndex < text.length) {
            result.push(text.slice(lastIndex));
        }
        return result;
    },
    image: (url) => {
        return utils.buildCQTag("image", { url });
    },
    video: (url) => {
        return utils.buildCQTag("video", { url });
    },
};
exports.utils = utils;
let console = {
    log(...args) {
        client.Console(new srpc_1.srpc.ConsoleRequest({
            type: "log",
            content: (0, util_1.format)(...args),
            plugin_id,
        }), (err, resp) => { });
    },
    info(...args) {
        const content = args.reduce((acc, arg) => acc + " " + arg, "");
        client.Console(new srpc_1.srpc.ConsoleRequest({
            type: "info",
            content: (0, util_1.format)(...args),
            plugin_id,
        }), (err, resp) => { });
    },
    error(...args) {
        const content = args.reduce((acc, arg) => acc + " " + arg, "");
        client.Console(new srpc_1.srpc.ConsoleRequest({
            type: "error",
            content: (0, util_1.format)(...args),
            plugin_id,
        }), (err, resp) => { });
    },
    debug(...args) {
        const content = args.reduce((acc, arg) => acc + " " + arg, "");
        client.Console(new srpc_1.srpc.ConsoleRequest({
            type: "debug",
            content: (0, util_1.format)(...args),
            plugin_id,
        }), (err, resp) => { });
    },
};
exports.console = console;
const container = createContainerApi();
exports.container = container;
