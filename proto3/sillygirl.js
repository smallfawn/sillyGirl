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
exports.express = exports.console = exports.utils = exports.sender = exports.SillyGirlPluginConfig = exports.sillyGirlCreateSchema = exports.DaiDai = exports.SmallCat = exports.QingLong = exports.Bucket = exports.Adapter = void 0;
exports.form = form;
exports.pluginConfigDefaults = pluginConfigDefaults;
exports.sleep = sleep;
exports.restart = restart;
exports.update = update;
const srpc_1 = require("./srpc");
const grpc_1 = __importStar(require("@grpc/grpc-js"));
const util_1 = require("util");
const { execFile } = require("child_process");
const path = require("path");
grpc_1.setLogVerbosity(grpc_1.logVerbosity.NONE);
let client = new srpc_1.srpc.SillyGirlServiceClient(process.env?.SILLYGIRL_GRPC_ADDR || "127.0.0.1:50051", grpc_1.credentials.createInsecure());
let senders = [];
let plugin_id = process.env?.PLUGIN_ID ?? "";
const metadata = new grpc_1.Metadata();
metadata.add("RUNTIME_ID", process.env?.RUNTIME_ID ?? "");
metadata.add("sillygirl-runtime-token", process.env?.SILLYGIRL_GRPC_TOKEN ?? "");
const express = new Proxy(function () { }, {
    apply(_target, thisArg, args) {
        return require("express").apply(thisArg, args);
    },
    get(_target, prop) {
        return require("express")[prop];
    },
});
exports.express = express;
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
function normalizeSchema(value) {
    if (value && value.__schemaNode && value.schema)
        return value.schema;
    if (value && typeof value.toJSON === "function")
        return value.toJSON();
    if (Array.isArray(value))
        return value.map((item) => normalizeSchema(item));
    if (value && typeof value === "object") {
        const result = {};
        for (const key of Object.keys(value)) {
            if (key.startsWith("_") || key === "__schemaNode")
                continue;
            result[key] = normalizeSchema(value[key]);
        }
        return result;
    }
    return value;
}
function pluginConfigDefaults(schema) {
    schema = normalizeSchema(schema) || {};
    if (Object.prototype.hasOwnProperty.call(schema, "default"))
        return schema.default;
    if (schema.type === "object" || schema.properties) {
        const values = {};
        for (const key of Object.keys(schema.properties || {})) {
            const value = pluginConfigDefaults(schema.properties[key]);
            if (value !== undefined)
                values[key] = value;
        }
        return values;
    }
    if (schema.type === "array")
        return [];
    return undefined;
}
class SchemaNode {
    __schemaNode = true;
    schema;
    constructor(type, extra = {}) {
        this.schema = Object.assign({ type }, extra);
    }
    setTitle(value) { this.schema.title = value; return this; }
    setDescription(value) { this.schema.description = value; return this; }
    setDefault(value) { this.schema.default = value; return this; }
    setEnum(value) { this.schema.enum = value; return this; }
    setEnumNames(value) { this.schema.enumNames = value; return this; }
    setRequired(value) { this.schema.required = value; return this; }
    setFormat(value) { this.schema.format = value; return this; }
    setMin(value) { this.schema.minimum = value; return this; }
    setMax(value) { this.schema.maximum = value; return this; }
    setMinLength(value) { this.schema.minLength = value; return this; }
    setMaxLength(value) { this.schema.maxLength = value; return this; }
    setPattern(value) { this.schema.pattern = value; return this; }
    setWidget(value) { this.schema["ui:widget"] = value; return this; }
    toJSON() { return this.schema; }
}
const sillyGirlCreateSchema = {
    string: () => new SchemaNode("string"),
    number: () => new SchemaNode("number"),
    integer: () => new SchemaNode("integer"),
    boolean: () => new SchemaNode("boolean"),
    array: (item) => new SchemaNode("array", { items: normalizeSchema(item) || {} }),
    object: (props) => {
        const properties = {};
        for (const key of Object.keys(props || {})) {
            properties[key] = normalizeSchema(props?.[key]);
        }
        return new SchemaNode("object", { properties });
    },
};
exports.sillyGirlCreateSchema = sillyGirlCreateSchema;
class SillyGirlPluginConfig {
    uuid = plugin_id;
    jsonSchema;
    userConfig = {};
    ready;
    constructor(schema) {
        this.jsonSchema = normalizeSchema(schema) || {};
        if (!this.jsonSchema.type)
            this.jsonSchema.type = "object";
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
exports.SillyGirlPluginConfig = SillyGirlPluginConfig;
function form(schema) {
    return new SillyGirlPluginConfig(schema);
}
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
exports.QingLong = QingLong;
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
    createQr(type) {
        const body = typeof type === "object" && type !== null ? type : { type };
        return this.request("POST", "/api/qr/start", body);
    }
    checkQr(uuid) {
        return this.request("GET", "/api/qr/status", undefined, { uuid });
    }
    addUser(options) {
        return this.request("POST", "/api/accounts/add", options || {});
    }
    userList() {
        return this.request("GET", "/api/accounts");
    }
    getCode(options) {
        const body = Object.assign({}, options || {});
        return this.request("POST", "/wx/code", body);
    }
    getUserInfo(options) {
        const body = Object.assign({}, options || {});
        return this.request("POST", "/wx/getuserinfo", body);
    }
    getPhoneNumber(options) {
        const body = Object.assign({}, options || {});
        return this.request("POST", "/wx/getphonenumber", body);
    }
    qrCodeAuth(options) {
        const body = Object.assign({}, options || {});
        return this.request("POST", "/wx/qrcodeauth", body);
    }
    oAuth(options) {
        const body = Object.assign({}, options || {});
        return this.request("POST", "/wx/oauth", body);
    }
}
exports.SmallCat = SmallCat;
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
exports.DaiDai = DaiDai;
globalThis.QingLong = QingLong;
globalThis.SmallCat = SmallCat;
globalThis.DaiDai = DaiDai;
globalThis.sillyGirlCreateSchema = sillyGirlCreateSchema;
globalThis.form = form;
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
async function update(options = {}) {
    const timeout = clampNumber(options.timeout || 120, 10, 600);
    const repo = await resolveSillyGirlRepo(options.appDir, timeout);
    const remote = String(options.gitRemote || "origin").trim() || "origin";
    const before = (await git(repo, ["rev-parse", "--short", "HEAD"], timeout)).stdout.trim();
    await git(repo, ["fetch", remote, "--prune"], timeout);
    const pullArgs = await pullCommand(repo, remote, options.gitBranch, timeout);
    const pulled = await git(repo, pullArgs, timeout);
    const after = (await git(repo, ["rev-parse", "--short", "HEAD"], timeout)).stdout.trim();
    const restarted = Boolean(options.restart);
    if (restarted)
        await restart();
    return {
        repo,
        before,
        after,
        changed: before !== after,
        output: compactRuntimeOutput(pulled.stdout || pulled.stderr),
        restarted,
    };
}
async function pullCommand(repo, remote, configuredBranch, timeout) {
    const branch = String(configuredBranch || "").trim() || (await currentBranch(repo, timeout)) || "main";
    if (configuredBranch)
        return ["pull", "--ff-only", remote, branch];
    try {
        const upstream = await git(repo, ["rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"], timeout);
        if (upstream.stdout.trim())
            return ["pull", "--ff-only"];
    }
    catch (_) { }
    return ["pull", "--ff-only", remote, branch];
}
async function currentBranch(repo, timeout) {
    try {
        const result = await git(repo, ["rev-parse", "--abbrev-ref", "HEAD"], timeout);
        const branch = result.stdout.trim();
        return branch && branch !== "HEAD" ? branch : "";
    }
    catch (_) {
        return "";
    }
}
async function resolveSillyGirlRepo(configured, timeout) {
    const candidates = [];
    addRepoCandidate(candidates, configured);
    addRepoCandidate(candidates, process.env?.SILLYGIRL_APP_DIR);
    addRepoCandidate(candidates, process.env?.SILLYGIRL_HOME);
    addRepoCandidate(candidates, process.env?.APP_HOME);
    addRepoCandidate(candidates, process.cwd());
    addRepoCandidate(candidates, path.resolve(process.cwd(), ".."));
    addRepoCandidate(candidates, path.resolve(process.cwd(), "../.."));
    addRepoCandidate(candidates, "/app");
    addRepoCandidate(candidates, "/data/sillyGirl");
    for (const candidate of candidates) {
        try {
            const result = await git(candidate, ["rev-parse", "--show-toplevel"], timeout);
            const repo = result.stdout.trim();
            if (repo && await isSillyGirlRepo(repo, timeout))
                return repo;
        }
        catch (_) { }
    }
    throw new Error("未找到可更新的 SillyGirl Git 仓库；Docker/Release 部署请使用镜像或 Release 包更新");
}
async function isSillyGirlRepo(repo, timeout) {
    try {
        const result = await git(repo, ["config", "--get", "remote.origin.url"], timeout);
        const remote = result.stdout.trim().toLowerCase();
        return remote.includes("sillygirl") && !remote.includes("sillygirl_plugins");
    }
    catch (_) {
        return false;
    }
}
function addRepoCandidate(list, value) {
    value = String(value || "").trim();
    if (!value)
        return;
    const normalized = path.resolve(value);
    if (!list.includes(normalized))
        list.push(normalized);
}
function git(cwd, args, timeoutSeconds) {
    return new Promise((resolve, reject) => {
        execFile("git", args, {
            cwd,
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
function compactRuntimeOutput(value) {
    const text = String(value || "").trim();
    if (!text)
        return "";
    return text.split(/\r?\n/).map((line) => line.trim()).filter(Boolean).slice(-8).join("\n").slice(0, 1000);
}
class Console {
    error = (message, ...optionalParams) => { };
    info = (message, ...optionalParams) => { };
    log = (message, ...optionalParams) => { };
    debug = (message, ...optionalParams) => { };
}
let utils = {
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
