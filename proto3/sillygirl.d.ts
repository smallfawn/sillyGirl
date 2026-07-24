declare const express: () => void;
declare class Sender {
    private uuid;
    private destoried;
    constructor(uuid: string);
    destroy(): void;
    getUserId(): Promise<string>;
    getUserName(): Promise<string>;
    getChatId(): Promise<string>;
    getChatName(): Promise<string>;
    getMessageId(): Promise<string>;
    getPlatform(): Promise<string>;
    getBotId(): Promise<string>;
    getContent(): Promise<string>;
    isAdmin(): Promise<boolean>;
    param(key: number | string): Promise<string>;
    setContent(content: string): Promise<undefined>;
    continue(): Promise<undefined>;
    getAdapter(): Promise<Adapter>;
    listen(options?: {
        rules?: string[];
        timeout?: number;
        handle?: (s: Sender) => Promise<string | void> | string | void;
        listen_private?: boolean;
        listen_group?: boolean;
        allow_platforms?: string[];
        prohibit_platforms?: string[];
        allow_groups?: string[];
        prohibit_groups?: string[];
        allow_users?: string[];
        prohibit_users?: string[];
    }): Promise<Sender | undefined>;
    holdOn(str: string): string;
    reply(content: string): Promise<string>;
    doAction(options: Record<string, any>): Promise<any>;
    getEvent(): Promise<Record<string, any>>;
}
declare class Bucket {
    private name;
    constructor(name: string);
    transform(v: string | undefined): string | number | boolean | undefined;
    reverseTransform(value: any): string;
    get(key: string, defaultValue?: any): Promise<any>;
    set(key: string, value: any): Promise<{
        message?: string;
        changed?: boolean;
    }>;
    getAll(): Promise<Record<string, any>>;
    delete(key: string): Promise<{
        message?: string;
        changed?: boolean;
    }>;
    deleteAll(): Promise<undefined>;
    keys(): Promise<string[]>;
    len(): Promise<number>;
    buckets(): Promise<string[]>;
    watch(key: string, handle: (old: any, now: any, key: string) => StorageModifier | void): void;
    getName(): Promise<string>;
}
interface SillyGirlSchemaNode {
    __schemaNode: boolean;
    schema: Record<string, any>;
    setTitle(value: string): SillyGirlSchemaNode;
    setDescription(value: string): SillyGirlSchemaNode;
    setDefault(value: any): SillyGirlSchemaNode;
    setEnum(value: any[]): SillyGirlSchemaNode;
    setEnumNames(value: string[]): SillyGirlSchemaNode;
    setRequired(value: string[] | boolean): SillyGirlSchemaNode;
    setFormat(value: string): SillyGirlSchemaNode;
    setMin(value: number): SillyGirlSchemaNode;
    setMax(value: number): SillyGirlSchemaNode;
    setMinLength(value: number): SillyGirlSchemaNode;
    setMaxLength(value: number): SillyGirlSchemaNode;
    setPattern(value: string): SillyGirlSchemaNode;
    setWidget(value: string): SillyGirlSchemaNode;
    toJSON(): Record<string, any>;
}
declare function pluginConfigDefaults(schema: any): any;
declare class SchemaNode implements SillyGirlSchemaNode {
    __schemaNode: boolean;
    schema: Record<string, any>;
    constructor(type: string, extra?: Record<string, any>);
    setTitle(value: string): this;
    setDescription(value: string): this;
    setDefault(value: any): this;
    setEnum(value: any[]): this;
    setEnumNames(value: string[]): this;
    setRequired(value: string[] | boolean): this;
    setFormat(value: string): this;
    setMin(value: number): this;
    setMax(value: number): this;
    setMinLength(value: number): this;
    setMaxLength(value: number): this;
    setPattern(value: string): this;
    setWidget(value: string): this;
    toJSON(): Record<string, any>;
}
declare const sillyGirlCreateSchema: {
    string: () => SchemaNode;
    number: () => SchemaNode;
    integer: () => SchemaNode;
    boolean: () => SchemaNode;
    array: (item?: any) => SchemaNode;
    object: (props?: Record<string, any>) => SchemaNode;
};
declare class SillyGirlPluginConfig {
    uuid: string;
    jsonSchema: Record<string, any>;
    userConfig: Record<string, any>;
    ready: Promise<Record<string, any>>;
    constructor(schema: any);
    init(): Promise<Record<string, any>>;
    get(): Promise<Record<string, any>>;
    Get(): Promise<Record<string, any>>;
    set(values?: Record<string, any>): Promise<{
        error: string;
    }>;
    Set(values?: Record<string, any>): Promise<{
        error: string;
    }>;
}
declare function form(schema: any): SillyGirlPluginConfig;
declare class QingLong {
    id: number;
    uuid: string;
    name: string;
    address: string;
    private panel;
    private token;
    private expiration;
    private ready;
    constructor(options: {
        id: number | string;
    });
    private init;
    private ensureToken;
    request(method: string, path: string, body?: any, query?: Record<string, any>): Promise<any>;
    getEnvs(options?: Record<string, any> | string): Promise<any>;
    getEnvById(id: number | string): Promise<any>;
    createEnv(env: any): Promise<any>;
    updateEnv(env: any): Promise<any>;
    deleteEnvs(ids: any): Promise<any>;
    moveEnv(id: number | string, arg1: any, arg2?: any): Promise<any>;
    disableEnvs(ids: any): Promise<any>;
    enableEnvs(ids: any): Promise<any>;
    updateEnvNames(arg1: any, arg2?: any): Promise<any>;
    systemNotify(title: string, content: string): Promise<any>;
}
declare class SmallCat {
    id: number;
    uuid: string;
    name: string;
    address: string;
    private panel;
    private ready;
    constructor(options: {
        id: number | string;
    });
    private init;
    request(method: string, path: string, body?: any, query?: Record<string, any>): Promise<any>;
    createQr(type: any): Promise<any>;
    checkQr(uuid: string): Promise<any>;
    addUser(options: {
        code: string;
        type: number | string;
        displayName?: string;
    }): Promise<any>;
    userList(): Promise<any>;
    getCode(options: {
        openid: string;
        appid: string;
    }): Promise<any>;
    getUserInfo(options: {
        openid: string;
        appid: string;
    }): Promise<any>;
    getPhoneNumber(options: {
        openid: string;
        appid: string;
    }): Promise<any>;
    qrCodeAuth(options: Record<string, any>): Promise<any>;
    oAuth(options: Record<string, any>): Promise<any>;
}
declare class DaiDai {
    id: number;
    uuid: string;
    name: string;
    address: string;
    private panel;
    private token;
    private expiration;
    private ready;
    constructor(options: {
        id: number | string;
    });
    private init;
    private ensureToken;
    request(method: string, path: string, body?: any, query?: Record<string, any>): Promise<any>;
    getEnvs(options?: Record<string, any> | string): Promise<any>;
    getEnvById(id: number | string): Promise<any>;
    createEnv(env: any): Promise<any>;
    updateEnv(env: any): Promise<any>;
    deleteEnv(id: number | string): Promise<any>;
    deleteEnvs(ids: any): Promise<any>;
    enableEnv(id: number | string): Promise<any>;
    disableEnv(id: number | string): Promise<any>;
    enableEnvs(ids: any): Promise<any>;
    disableEnvs(ids: any): Promise<any>;
    getTasks(options?: Record<string, any> | string): Promise<any>;
    getTaskById(id: number | string): Promise<any>;
    createTask(task: any): Promise<any>;
    updateTask(task: any): Promise<any>;
    deleteTask(id: number | string): Promise<any>;
    runTask(id: number | string): Promise<any>;
    stopTask(id: number | string): Promise<any>;
    enableTask(id: number | string): Promise<any>;
    disableTask(id: number | string): Promise<any>;
    systemNotify(title: string, content: string): Promise<any>;
}
interface StorageModifier {
    echo?: string;
    now?: any;
    message?: string;
    error?: string;
}
interface Message {
    message_id?: string;
    user_id: string;
    chat_id?: string;
    content: string;
    user_name?: string;
    chat_name?: string;
}
declare class Adapter {
    platform: string;
    bot_id: string;
    call: any;
    constructor(options: {
        platform: string;
        bot_id: string;
        replyHandler?: (message: Message) => Promise<string | undefined>;
        actionHandler?: (message: Message) => Promise<string | undefined>;
    });
    receive(message: Message): Promise<undefined>;
    push(message: Message): Promise<string>;
    destroy(): Promise<void>;
    sender(options: any): Promise<Sender>;
}
declare let sender: Sender;
declare function sleep(ms?: number): Promise<unknown>;
interface UpdateOptions {
    mode?: string;
    appDir?: string;
    gitRemote?: string;
    gitBranch?: string;
    dockerSocket?: string;
    dockerWatchtowerImage?: string;
    timeout?: number;
    restart?: boolean;
}
interface UpdateResult {
    mode?: string;
    repo: string;
    before: string;
    after: string;
    changed: boolean;
    output: string;
    restarted: boolean;
}
declare function restart(): Promise<{
    message?: string;
    changed?: boolean;
}>;
declare function update(options?: UpdateOptions): Promise<UpdateResult>;
interface CQItem {
    type: string;
    params: Record<string, string>;
}
interface CQParams {
    [key: string]: string | number | boolean;
}
declare let utils: {
    buildCQTag: (type: string, params: CQParams, prefix?: string) => string;
    parseCQText: (text: string, prefix?: string) => (string | CQItem)[];
    image: (url: string) => string;
    video: (url: string) => string;
};
declare let console: {
    log(...args: any[]): void;
    info(...args: any[]): void;
    error(...args: any[]): void;
    debug(...args: any[]): void;
};
export { Adapter, Bucket, QingLong, SmallCat, DaiDai, sillyGirlCreateSchema, SillyGirlPluginConfig, form, pluginConfigDefaults, sender, sleep, restart, update, utils, console, express, };
