# API 参考

本文档提供 SillyGirl 暴露的所有编程接口的详细参考，包括 REST API、gRPC API 和 JavaScript 插件 API。

## 目录

- [REST API](#rest-api)
  - [认证](#认证)
  - [Bucket API](#bucket-api)
  - [Plugin API](#plugin-api)
  - [File API](#file-api)
- [WebSocket](#websocket)
- [gRPC API](#grpc-api)
  - [服务定义](#服务定义)
  - [消息类型](#消息类型)
- [JavaScript API](#javascript-api)
  - [全局函数](#全局函数)
  - [Sender 接口](#sender-接口)
  - [Bucket 接口](#bucket-接口)
  - [Cron 接口](#cron-接口)

## REST API

Base URL: `http://host:port/api`

### 约定

- 资源名使用小写复数名词和 kebab-case；资源标识放在路径参数中。
- API 业务路由只使用 `GET` 和 `POST`：`GET` 安全读取；`POST` 创建资源、提交资源表示或创建处理任务。
- 对现有资源的变更使用 `POST /resources/:id`；删除被建模为 deletion 子资源，使用 `POST /resources/:id/deletions`，不使用动作式 URL。
- 分页、筛选和可选关联数据使用查询参数；不使用 `/list`、动词路径、扩展名路由或 `?id=` 定位单资源。
- 被替换的旧路由不保留兼容别名。
- 创建成功返回 `201 Created` 和新资源的 `Location`；异步任务返回 `202 Accepted`；成功删除且不返回表示时使用 `204 No Content`。
- 不存在、冲突、语义校验失败和服务端异常分别返回 `404`、`409`、`422`、`500`，不再用 `200` 包装错误。

除文件下载等原始内容接口外，响应使用统一 envelope：

```json
{
  "status": true,
  "message": "成功",
  "data": {}
}
```

错误响应使用 `application/problem+json`：

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "字段校验失败",
  "instance": "/api/admin/tasks/task-1",
  "errors": { "schedule": "Cron 表达式格式错误" }
}
```

需要登录的 Admin 和 User 资源通过 `Authorization: Bearer <token>` 认证。

### Admin 认证与系统资源

| Method | Resource | 说明 |
|---|---|---|
| `GET`, `POST` | `/api/admin/setup` | 查询或完成首次初始化 |
| `POST` | `/api/admin/sessions` | 创建管理会话 |
| `GET` | `/api/admin/sessions/current` | 读取当前管理会话 |
| `POST` | `/api/admin/sessions/current/deletions` | 创建当前管理会话的删除记录并结束会话 |
| `GET`, `POST` | `/api/admin/settings` | 读取或提交系统设置单例 |
| `POST` | `/api/admin/system-update-jobs` | 创建更新任务 |
| `GET` | `/api/admin/system-update-jobs/:id` | 读取更新任务 |
| `POST` | `/api/admin/system-restart-jobs` | 创建重启任务 |
| `GET` | `/api/admin/system-restart-jobs/:id` | 读取重启任务 |
| `GET` | `/api/admin/system-backups/current` | 下载当前系统备份 |
| `POST` | `/api/admin/clawbot-login-sessions` | 创建 ClawBot 登录会话 |
| `GET` | `/api/admin/clawbot-login-sessions/:session` | 读取登录状态 |
| `POST` | `/api/admin/clawbot-login-sessions/:session/verification-attempts` | 提交验证码尝试 |

### Admin 业务资源

| Method | Resource |
|---|---|
| `GET`, `POST` | `/api/admin/users` |
| `POST` | `/api/admin/users/:username` |
| `POST` | `/api/admin/users/:username/deletions` |
| `GET`, `POST` | `/api/admin/tasks` |
| `POST` | `/api/admin/tasks/:task_id` |
| `POST` | `/api/admin/tasks/:task_id/deletions` |
| `GET` | `/api/admin/task-options?task_id=:task_id` |
| `POST` | `/api/admin/tasks/:task_id/executions` |
| `GET`, `POST` | `/api/admin/replies` |
| `POST` | `/api/admin/replies/:id` |
| `POST` | `/api/admin/replies/:id/deletions` |
| `GET`, `POST` | `/api/admin/carry-groups` |
| `POST` | `/api/admin/carry-groups/:chat_id` |
| `POST` | `/api/admin/carry-groups/:chat_id/deletions` |
| `GET` | `/api/admin/carry-group-options` |
| `GET`, `POST` | `/api/admin/masters` |
| `POST` | `/api/admin/masters/:platform/:number/deletions` |
| `GET`, `POST` | `/api/admin/panels` |
| `POST` | `/api/admin/panels/:id` |
| `POST` | `/api/admin/panels/:id/deletions` |
| `GET` | `/api/admin/panels/:id/accounts` |
| `POST` | `/api/admin/panel-connection-tests` |
| `POST` | `/api/admin/panel-status-checks` |
| `GET`, `POST` | `/api/admin/plugin-settings/:uuid` |
| `POST` | `/api/admin/plugin-settings/:uuid/deletions` |
| `GET` | `/api/admin/plugin-settings` |
| `POST` | `/api/admin/local-plugins` |
| `GET`, `POST` | `/api/admin/local-plugins/:id` |
| `POST` | `/api/admin/local-plugins/:id/deletions` |
| `POST` | `/api/admin/plugins/:uuid/access` |
| `GET` | `/api/admin/dependencies?runtime=:runtime&plugin=:plugin` |
| `POST` | `/api/admin/dependencies` |
| `POST` | `/api/admin/dependency-deletions/:runtime/:plugin/:package` |
| `GET`, `POST` | `/api/admin/dependency-registries/:runtime` |
| `POST` | `/api/admin/dependency-registries/:runtime/options` |
| `POST` | `/api/admin/dependency-registries/:runtime/option-deletions/:registry` |
| `POST` | `/api/admin/scripts` |
| `GET`, `POST` | `/api/admin/scripts/:id` |
| `POST` | `/api/admin/scripts/:id/deletions` |
| `GET`, `POST` | `/api/admin/plugin-market/sources` |
| `POST` | `/api/admin/plugin-market/source-deletions/:address` |
| `GET` | `/api/admin/plugin-market/plugins` |
| `GET`, `POST` | `/api/admin/plugin-market/github-proxy` |
| `POST` | `/api/admin/plugin-market/github-proxy-options` |
| `POST` | `/api/admin/plugin-market/github-proxy-option-deletions/:proxy` |
| `GET`, `POST` | `/api/admin/storage/values` |
| `GET` | `/api/admin/storage/entries` |
| `GET`, `POST` | `/api/admin/storage/buckets` |
| `POST` | `/api/admin/storage/buckets/:bucket/deletions` |
| `GET` | `/api/admin/message-rules/:kind` |
| `POST` | `/api/admin/message-rules/:kind/:key` |
| `POST` | `/api/admin/message-rules/:kind/:key/deletions` |
| `GET` | `/api/admin/bots` |

面板集合使用请求体字段 `type: "qinglong" | "daidai" | "smallcat"` 区分具体类型。`GET /api/admin/panels` 一次返回三类面板，不再并发请求 provider 专用接口。

### User 资源

| Method | Resource |
|---|---|
| `POST` | `/api/user/accounts` |
| `POST` | `/api/user/sessions` |
| `POST` | `/api/user/sessions/current/deletions` |
| `GET` | `/api/user/profile` |
| `POST` | `/api/user/bindings/:platform` |
| `POST` | `/api/user/bindings/:platform/deletions` |
| `GET` | `/api/user/plugins` |
| `POST` | `/api/user/plugins/:uuid/authorization` |
| `GET` | `/api/user/plugins/:uuid/smallcat-accounts` |
| `GET` | `/api/user/plugins/:uuid/form` |
| `POST` | `/api/user/plugins/:uuid/form-records` |
| `POST` | `/api/user/plugins/:uuid/form-records/:record_id` |
| `POST` | `/api/user/plugins/:uuid/form-records/:record_id/deletions` |
| `GET` | `/api/user/smallcat-panels` |
| `POST` | `/api/user/smallcat-login-sessions` |
| `GET` | `/api/user/smallcat-login-sessions/:panel/:uuid` |
| `POST` | `/api/user/smallcat-login-sessions/:panel/:uuid/confirmations` |
| `POST` | `/api/user/smallcat-accounts` |
| `POST` | `/api/user/smallcat-verification-codes` |

### Public 资源

| Method | Resource | 响应 |
|---|---|---|
| `GET` | `/api/health` | 健康状态 |
| `GET` | `/api/public/plugins` | 已开放插件 |
| `GET` | `/api/plugin-market/plugins` | 联邦插件市场数据 |
| `GET` | `/api/plugin-downloads/:uuid` | JavaScript 或 ZIP |
| `GET` | `/api/files/*filename` | 插件静态文件 |
| `GET` | `/api/binary-content/:token` | 临时二进制内容 |
| `GET` | `/api/web-chat/messages?rid=...` | 长轮询接收消息 |
| `POST` | `/api/web-chat/messages` | 创建聊天消息 |

Web Chat 发送示例：

```bash
curl -X POST http://localhost:8080/api/web-chat/messages \
  -H 'Content-Type: application/json' \
  -d '{"rid":"user123","ctt":"你好"}'
```

仅接收消息：

```bash
curl 'http://localhost:8080/api/web-chat/messages?rid=user123'
```

## gRPC API

### 服务定义

**Proto 文件**：`proto3/srpc.proto`

**Go 包**：`github.com/smallfawn/sillyGirl/proto3/srpc`

**服务名**：`SillyGirlService`

### 消息类型

#### Empty
```protobuf
message Empty {}
```

#### Default
```protobuf
message Default {
  string value = 1;
}
```

#### BucketSetRequest
```protobuf
message BucketSetRequest {
  string name = 1;
  string key = 2;
  string value = 3;
}
```

#### BucketSetResponse
```protobuf
message BucketSetResponse {
  bool changed = 1;
  string message = 2;
}
```

#### BucketKeyRequest
```protobuf
message BucketKeyRequest {
  string name = 1;
  string key = 2;
}
```

#### BucketRequest
```protobuf
message BucketRequest {
  string name = 1;
}
```

#### BucketKeysResponse
```protobuf
message BucketKeysResponse {
  repeated string keys = 1;
}
```

#### LenResponse
```protobuf
message LenResponse {
  int32 length = 1;
}
```

#### BoolResponse
```protobuf
message BoolResponse {
  bool value = 1;
}
```

#### BucketsResponse
```protobuf
message BucketsResponse {
  repeated string buckets = 1;
}
```

#### SenderRequest
```protobuf
message SenderRequest {
  string uuid = 1;
}
```

#### ReplyRequest
```protobuf
message ReplyRequest {
  string uuid = 1;
  string content = 2;
}
```

#### SenderContentRequest
```protobuf
message SenderContentRequest {
  string uuid = 1;
  string content = 2;
}
```

#### SenderListenRequest
```protobuf
message SenderListenRequest {
  string uuid = 1;
  repeated string rules = 2;
  int32 timeout = 3;
  bool listen_group = 4;
  bool listen_private = 5;
  bool require_admin = 6;
  repeated string allow_platforms = 7;
  repeated string prohibit_platforms = 8;
  repeated string allow_users = 9;
  repeated string prohibit_users = 10;
  repeated string allow_groups = 11;
  repeated string prohibit_groups = 12;
  bool persistent = 13;
  string value = 14;
  string plugin_id = 15;
}
```

#### SenderListenResponse
```protobuf
message SenderListenResponse {
  string echo = 1;
  string uuid = 2;
}
```

#### AdapterRegistRequest
```protobuf
message AdapterRegistRequest {
  string platform = 1;
  string bot_id = 2;
}
```

#### AdapterRequest
```protobuf
message AdapterRequest {
  string platform = 1;
  string bot_id = 2;
  string value = 3;
}
```

#### ConsoleRequest
```protobuf
message ConsoleRequest {
  string type = 1;
  string content = 2;
  string plugin_id = 3;
}
```

### RPC 方法列表

| 方法 | 请求 | 响应 | 说明 |
|------|------|------|------|
| `BucketGet` | `BucketKeyRequest` | `Default` | 获取 Bucket 键值 |
| `BucketSet` | `BucketSetRequest` | `BucketSetResponse` | 设置 Bucket 键值 |
| `BucketDelete` | `BucketRequest` | `Empty` | 删除 Bucket |
| `BucketKeys` | `BucketRequest` | `BucketKeysResponse` | 获取所有键名 |
| `BucketLen` | `BucketRequest` | `LenResponse` | 获取键数量 |
| `BucketGetAll` | `BucketRequest` | `Default` | 获取所有键值（JSON） |
| `BucketBuckets` | `Empty` | `BucketsResponse` | 获取所有 Bucket 名 |
| `BucketWatch` | `stream BucketWatchRequest` | `stream BucketWatchResponse` | 流式监听变更 |
| `SenderGetUserId` | `SenderRequest` | `Default` | 获取用户ID |
| `SenderGetUserName` | `SenderRequest` | `Default` | 获取用户名 |
| `SenderGetChatId` | `SenderRequest` | `Default` | 获取群聊ID |
| `SenderGetChatName` | `SenderRequest` | `Default` | 获取群聊名 |
| `SenderGetMessageId` | `SenderRequest` | `Default` | 获取消息ID |
| `SenderIsAdmin` | `SenderRequest` | `BoolResponse` | 是否管理员 |
| `SenderGetPlatform` | `SenderRequest` | `Default` | 获取平台 |
| `SenderGetBotId` | `SenderRequest` | `Default` | 获取机器人ID |
| `SenderGetContent` | `SenderRequest` | `Default` | 获取消息内容 |
| `SenderSetContent` | `SenderContentRequest` | `Empty` | 设置消息内容 |
| `SenderContinue` | `SenderRequest` | `Empty` | 继续匹配 |
| `SenderListen` | `stream SenderListenRequest` | `stream SenderListenResponse` | 流式消息监听 |
| `SenderEvent` | `SenderRequest` | `Default` | 获取事件数据 |
| `SenderReply` | `ReplyRequest` | `Default` | 发送回复 |
| `SenderParam` | `ReplyRequest` | `Default` | 获取参数 |
| `SenderAction` | `ReplyRequest` | `Default` | 执行动作 |
| `SenderDestroy` | `ReplyRequest` | `Empty` | 销毁 Sender |
| `AdapterRegist` | `stream AdapterRegistRequest` | `stream Default` | 注册适配器 |
| `AdapterReceive` | `AdapterRequest` | `Empty` | 接收消息 |
| `AdapterPush` | `AdapterRequest` | `Default` | 推送消息 |
| `AdapterDestroy` | `AdapterRequest` | `Empty` | 销毁适配器 |
| `AdapterSender` | `AdapterRequest` | `Default` | 获取 Sender |
| `Console` | `ConsoleRequest` | `Empty` | 控制台日志 |

### Python 插件示例

Python 插件运行在 Python 3.12 中，通过 `sillygirl` SDK 调用 Go 主程序能力：

```python
"""
* @title Python 示例
* @rule raw ^py$
"""

import asyncio
from sillygirl import sender as s, Bucket


async def main():
    db = Bucket("demo")
    count = await db.get("count", 0)
    await db.set("count", count + 1)
    await s.reply(f"count={count + 1}")


asyncio.run(main())
```

### Python gRPC 客户端示例

```python
import grpc
from proto3 import srpc_pb2, srpc_pb2_grpc

channel = grpc.insecure_channel('localhost:50051')
stub = srpc_pb2_grpc.SillyGirlServiceStub(channel)

# 获取 Bucket 值
req = srpc_pb2.BucketKeyRequest(name="sillyGirl", key="port")
resp = stub.BucketGet(req)
print(f"Port: {resp.value}")

# 设置 Bucket 值
req = srpc_pb2.BucketSetRequest(name="app", key="test", value="hello")
resp = stub.BucketSet(req)
print(f"Changed: {resp.changed}")
```

## JavaScript API

### 全局函数

```js
Bucket(name: string): Bucket
Cron(): Cron
sleep(ms: number): void
md5(str: string): string
uuid(): string
running(): boolean
```

### Sender 接口

`Sender` 对象通过全局变量 `s` 或 `sender` 访问。

#### 用户信息

```js
s.getUserId(): string
s.getUserName(): string
s.getChatId(): string
s.getChatName(): string
s.getMessageId(): string
s.getPlatform(): string
s.getBotId(): string
s.isAdmin(): boolean
s.getLevel(): number
s.setLevel(level: number): void
```

#### 内容操作

```js
s.getContent(): string
s.setContent(content: string): void
s.resume(): void
```

#### 回复与消息

```js
s.reply(...texts: string[]): { message_id: string, error: string }
s.recallMessage(messageId: string | string[] | string[][]): void
```

#### 参数捕获

```js
s.param(name: string): string
s.param(index: number): string
s.get(index: number): string
s.getAllMatch(): string[][]
```

#### 群管功能

```js
s.kick(userId: string): string | null
s.unkick(userId: string): string | null
s.ban(userId: string, duration: number): string | null
s.unban(userId: string): string | null
```

#### 监听

```js
s.listen(options: ListenOptions): Sender | undefined
```

`ListenOptions` 结构：

```js
{
  rules: string[],           // 匹配规则数组
  timeout?: number,          // 超时毫秒
  handle?: Function,         // 回调函数
  private?: boolean,         // 允许私聊
  group?: boolean,           // 允许群聊
  require_admin?: boolean,   // 需要管理员
  allow_platforms?: string[],
  prohibit_platforms?: string[],
  allow_users?: string[],
  allow_groups?: string[],
  prohibit_users?: string[],
  prohibit_groups?: string[],
  user_id?: string,
  chat_id?: string,
  platform?: string,
}
```

#### 其他

```js
s.holdOn(text?: string): string
s.action(options: object): { result: any, error: any }
s.doAction(options: object): { result: any, error: any }
s.getVar(key: string): any
s.setVar(key: string, value: any): void
s.setVars(kvs: object): void
s.getVars(): object
s.getReplyUserID(): number
s.isReply(): boolean
```

### Bucket 接口

```js
interface Bucket {
  get(key: string, defaultValue?: any): any
  set(key: string, value: any): Error | null
  set2(key: string, value: any): Error | null
  delete(key: string): Error | null
  keys(): string[]
  watch(key: string, callback: (old: any, new_: any, key: string) => void): void
  getAll(): Record<string, any>
  empty(): Error | undefined
  count(): number
  buckets(): string[]
}
```

### Cron 接口

```js
interface Cron {
  add(crontab: string, callback: Function): { id: number, error: string }
  remove(id: number): void
}
```
