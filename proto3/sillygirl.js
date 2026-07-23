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
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.console = exports.utils = exports.sleep = exports.sender = exports.daidai = exports.smallcat = exports.qinglong = exports.Bucket = exports.Adapter = void 0;
const srpc_1 = require("./srpc");
const grpc_1 = __importStar(require("@grpc/grpc-js"));
const util_1 = require("util");
grpc_1.setLogVerbosity(grpc_1.logVerbosity.NONE);
let client = new srpc_1.srpc.SillyGirlServiceClient("localhost:50051", grpc_1.credentials.createInsecure());
let senders = [];
let plugin_id = process.env?.PLUGIN_ID ?? "";
const metadata = new grpc_1.Metadata();
metadata.add("RUNTIME_ID", process.env?.RUNTIME_ID ?? "");
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
            client.BucketGet(new srpc_1.srpc.BucketKeyRequest({ name: this.name, key }), (err, resp) => {
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
            }), (err, resp) => {
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
            client.BucketGetAll(new srpc_1.srpc.BucketRequest({ name: this.name }), (err, resp) => {
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
            client.BucketDelete(new srpc_1.srpc.BucketRequest({ name: this.name }), (err, resp) => {
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
            client.BucketKeys(new srpc_1.srpc.BucketRequest({ name: this.name }), (err, resp) => {
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
            client.BucketLen(new srpc_1.srpc.BucketRequest({ name: this.name }), (err, resp) => {
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
            client.BucketBuckets(new srpc_1.srpc.Empty(), (err, resp) => {
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
        const call = client.BucketWatch();
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
        catch (e) {
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
function queryString(query) {
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
class qinglong {
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
        const response = await fetch(`${this.address}${normalizeRuntimePath(path, "/open")}${queryString(query || {})}`, {
            method: String(method || "GET").toUpperCase(),
            headers: Object.assign({ Authorization: `Bearer ${this.token}` }, body === undefined || body === null ? {} : { "Content-Type": "application/json" }),
            body: body === undefined || body === null ? undefined : JSON.stringify(body),
        });
        const text = await response.text();
        const result = text ? JSON.parse(text) : {};
        if (!response.ok) {
            throw new Error(result.message || `青龙接口 HTTP ${response.status}`);
        }
        if (result.code !== undefined && result.code !== 200) {
            throw new Error(result.message || "青龙接口请求失败");
        }
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
exports.qinglong = qinglong;
class smallcat {
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
        const response = await fetch(`${this.address}${normalizeRuntimePath(path, "")}${queryString(query || {})}`, {
            method: String(method || "GET").toUpperCase(),
            headers: Object.assign({ auth: this.panel.api_auth || "" }, body === undefined || body === null ? {} : { "Content-Type": "application/json" }),
            body: body === undefined || body === null ? undefined : JSON.stringify(body),
        });
        const text = await response.text();
        if (!text)
            return {};
        return JSON.parse(text);
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
}
exports.smallcat = smallcat;
class daidai {
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
        const response = await fetch(`${this.address}${normalizeRuntimePath(path, "/api")}${queryString(query || {})}`, {
            method: String(method || "GET").toUpperCase(),
            headers: Object.assign({ Authorization: `Bearer ${this.token}` }, body === undefined || body === null ? {} : { "Content-Type": "application/json" }),
            body: body === undefined || body === null ? undefined : JSON.stringify(body),
        });
        const text = await response.text();
        const result = text ? JSON.parse(text) : {};
        if (!response.ok) {
            throw new Error(result.message || result.error || `呆呆面板接口 HTTP ${response.status}`);
        }
        if (result.success === false) {
            throw new Error(result.message || result.error || "呆呆面板接口请求失败");
        }
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
exports.daidai = daidai;
globalThis.qinglong = qinglong;
globalThis.smallcat = smallcat;
globalThis.daidai = daidai;
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
exports.sleep = sleep;
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
