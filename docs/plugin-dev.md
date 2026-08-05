# 插件开发指南

SillyGirl 的插件系统基于外部脚本运行时。插件以 `.js` 或 `.py` 文件形式放在 `/data/plugins`，通过顶部注释声明元数据，由框架自动加载、匹配，并通过 gRPC 与 Go 主程序通信。

## 目录

- [插件结构](#插件结构)
- [Python 插件](#python-插件)
- [注释元数据](#注释元数据)
- [搬运处理脚本](#搬运处理脚本)
- [脚本定时任务](#脚本定时任务)
- [全局对象与 API](#全局对象与-api)
  - [sender (s)](#sender-s)
  - [Bucket(name)](#bucketname)
  - [QingLong 内联客户端](#qinglong-内联客户端)
  - [SmallCat 内联客户端](#smallcat-内联客户端)
  - [DaiDai 内联客户端](#daidai-内联客户端)
  - [Cron()](#cron)
  - [其他全局函数](#其他全局函数)
- [规则匹配语法](#规则匹配语法)
- [消息监听与会话](#消息监听与会话)
- [定时任务](#定时任务)
- [完整示例](#完整示例)
- [调试技巧](#调试技巧)

## 插件结构

一个最小插件由注释元数据和执行代码组成：

脚本插件默认使用扁平文件结构，容器内路径为：

```text
/data/plugins/
  smallcat.js
  hello.py
  package.json
  pnpm-lock.yaml
  node_modules/
```

NodeJS 依赖是插件目录共享的，所有 NodeJS 插件共用 `/data/plugins/package.json` 和 `/data/plugins/node_modules`。旧版
`/data/plugins/插件名/main.js` 仍会兼容加载，但新建和插件市场安装都会写入 `/data/plugins/插件名.js`。
Python 插件使用 Python 3.12，文件为 `/data/plugins/插件名.py`。Docker 镜像已内置 Python 3.12、pipx、`grpcio==1.83.0` 和 `protobuf==7.35.1`；本地运行时需要自行安装 Python 3.12 和 pipx。

```js
/**
 * @title HelloWorld
 * @rule raw ^你好$
 */

s.reply("Hello World!");
```

Python 最小插件：

```python
"""
* @title PythonHello
* @rule raw ^py你好$
"""

import asyncio
from sillygirl import sender as s


async def main():
    await s.reply("Hello World!")


asyncio.run(main())
```

## Python 插件

Python 插件和 NodeJS 插件共用同一套元数据、规则匹配、配置表单、定时任务和插件市场逻辑。差别主要在运行时和 SDK 调用方式。

插件市场支持直接新增本地非公开插件：点击插件市场工具栏“新增插件”会打开源码编辑器，保存时必须包含新式 `[title: xxx]`、`[name: 文件名]`、`[description: xxx]`、`[version: vx.y.z]`，并至少包含 `[rule: xxx]` 或 `[cron: xxx]`/`[on_start: true]`/`[web: true]` 之一；系统会强制写入/修正为 `[public: false]`。已安装插件卡片图标也可打开同款源码编辑器，支持格式化、保存和删除。旧式 `@title` 注释仍兼容。

### 运行环境

| 项目 | 说明 |
|------|------|
| Python 版本 | 固定使用 Python 3.12。Docker 镜像已内置；本地 `go run` 时需要机器上能执行 `python3.12`，或配置 `SILLYGIRL_PYTHON_BIN` |
| SDK 路径 | 程序会把 `sillygirl.py`、`srpc_pb2.py`、`srpc_pb2_grpc.py` 加入 `PYTHONPATH`，插件直接 `from sillygirl import ...` |
| gRPC 依赖 | 运行时自动准备 `grpcio==1.83.0`、`protobuf==7.35.1` |
| 第三方依赖 | 通过新式 `[depe: ["包名"]]` 声明；安装、卸载和查看在 Admin 面板「依赖管理」里选择 Python；旧式 `@depe` 仍兼容 |
| 依赖管理 | Python 依赖由 pipx 管理，统一安装到 `/data/plugins/python_packages`，所有 Python 插件共享 |
| 文件结构 | 推荐 `/data/plugins/插件名.py` 扁平文件；插件市场按索引 `path` 写入同名 `.py` 文件 |

本地运行时可以按需指定：

```bash
export SILLYGIRL_PYTHON_BIN=python3.12
export SILLYGIRL_PIPX=pipx
```

Windows PowerShell：

```powershell
$env:SILLYGIRL_PYTHON_BIN = "py -3.12"
$env:SILLYGIRL_PIPX = "pipx"
```

### 异步调用约定

Python SDK 方法都是异步方法，必须在 `async def main()` 中 `await`：

```python
content = await s.getContent()
user_id = await s.getUserId()
await s.reply("收到：" + content)
```

不要写成：

```python
content = s.getContent()
s.reply("收到")
```

上面的写法只会得到 coroutine 对象，或者导致脚本未真正发送回复。

### 常用 API

```python
from sillygirl import sender as s, Bucket, container, utils

content = await s.getContent()
user_id = await s.getUserId()
chat_id = await s.getChatId()
platform = await s.getPlatform()
is_admin = await s.isAdmin()
city = await s.param("城市")
await s.reply("文本")
await s.pushAdmin("管理员通知")
await utils.restart()
update_result = await utils.update({"restart": True})
```

### 普通用户列表

`utils.userList()` 提供普通用户及当前插件的授权和绑定状态。`authorized` 只表示当前插件的 `smallcat:read` 授权，运行时会从当前插件上下文判定，不接受插件 UUID 参数。

```js
const { container, utils } = require("sillygirl");

const users = await utils.userList();
const openids = users
  .filter((user) => !user.disabled && user.authorized)
  .flatMap((user) => user.bindings.smallcat_openids);

const accounts = await new container.SmallCat({ id: 1 }).userList();
```

```python
from sillygirl import container, utils

users = await utils.userList()
openids = [
    openid
    for user in users
    if not user["disabled"] and user["authorized"]
    for openid in user["bindings"]["smallcat_openids"]
]

accounts = await container.SmallCat({"id": 1}).userList()
```

`utils.userList()` 的每个用户包含：

```json
{
  "id": "USER_ID",
  "username": "ACCOUNT",
  "nickname": "昵称",
  "disabled": false,
  "authorized": true,
  "bindings": {
    "qq": "10001",
    "telegram": "20002",
    "smallcat_openids": ["OPENID"]
  }
}
```

未授权或已禁用用户仍会出现在列表中用于判断状态，但 `bindings.smallcat_openids` 会被运行时清空。插件关闭、禁用或未声明使用 SmallCat 时，所有用户的 `authorized` 都为 `false`；插件只能读取当前有效授权用户的 SmallCat openid。

### Bucket 存储

```python
from sillygirl import Bucket

db = Bucket("python-demo")

await db.set("count", 1)
count = await db.get("count", 0)
keys = await db.keys()
await db.delete("count")
```

不同插件建议使用不同 Bucket 名称，避免覆盖其他插件的数据。

### 配置表单

Python 插件支持和 NodeJS 一样的声明式配置。配置对象建议在文件顶层创建，这样插件安装后触发一次规则或设置 `@on_start true` 时，后台「插件配置」就能注册到表单。

```python
from sillygirl import sender as s, form

config = form({
    "apiBase": form.string()
        .title("接口地址")
        .default("http://127.0.0.1:8081"),
    "token": form.string()
        .title("Token")
        .format("password")
        .default(""),
    "enabled": form.boolean()
        .title("启用")
        .default(False),
})


async def main():
    values = await config.get()
    if not values.get("token"):
        await s.reply("请先到后台插件配置填写 Token")
        return
```

### 依赖声明

第三方 Python 包写在新式 `[depe: ...]`，值是 JSON 数组：

```python
"""
# [title: HTTP 示例]
# [rule: raw ^请求测试$]
# [depe: ["requests"]]
"""
```

支持版本约束：

```python
# [depe: ["requests==2.32.0", "beautifulsoup4"]]
```

注意：

- `os`、`sys`、`json`、`asyncio`、`time`、`pathlib` 等标准库不要写进 `[depe: ...]`。
- Python 依赖是共享安装，卸载依赖前确认没有其他插件还在使用。
- 插件仓库的索引 `dependencies` 字段由插件注释里的新式 `[depe: ...]` 生成；没有索引时，SillyGirl 会保底读取脚本注释，也兼容旧式 `@depe ...`。

### Python 定时任务

```python
"""
* @title Python定时提醒
* @cron 0 9 * * *
"""

import asyncio
from sillygirl import sender as s


async def main():
    await s.reply("早上 9 点提醒")


asyncio.run(main())
```

`@cron` 只写表达式，不要在后面追加平台。Admin 面板里手动创建 `python 插件名.py` 定时任务时，也会把表达式写回脚本注释。

### Python Web 常驻脚本

`@web true` 会让脚本作为常驻进程启动，端口和路由由脚本自己监听。下面示例只用标准库，不需要额外依赖：

```python
"""
* @title PythonWeb示例
* @web true
"""

from http.server import BaseHTTPRequestHandler, HTTPServer
import json


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/health":
            self.send_response(404)
            self.end_headers()
            return
        body = json.dumps({"status": True, "message": "ok", "data": None}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


HTTPServer(("0.0.0.0", 3002), Handler).serve_forever()
```

### 内联客户端

Python 插件可以引入内联客户端，构造方式和 JS 一样，按后台页面编号选择实例：

```python
ql = QingLong({"id": 1})
envs = await ql.getEnvs({"searchValue": "JD_COOKIE"})

sc = SmallCat({"id": 1})
code = await sc.getCode({"openid": "openid", "appid": "wx123"})

dd = DaiDai({"id": 1})
items = await dd.getEnvs({"keyword": "JD_COOKIE"})
```

所有客户端方法仍然需要 `await`。

插件文件可以包含多个 `@rule`，每条规则独立匹配：

```js
/**
 * @title 多功能助手
 * @rule raw ^你好$
 * @rule raw ^再见$
 * @rule 天气 [城市]
 */

const content = s.getContent();
if (content === "你好") {
  s.reply("Hello!");
} else if (content === "再见") {
  s.reply("Goodbye!");
} else {
  const city = s.param("城市");
  s.reply(`${city}今天天气晴朗！`);
}
```

## 注释元数据

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | 是 | 插件标题，显示在管理面板和插件市场 |
| `rule` | string | 否 | 消息匹配规则，支持多行。详见[规则匹配语法](#规则匹配语法) |
| `priority` | number | 否 | 匹配优先级，数字越大越优先，默认 0 |
| `cron` | string | 否 | 脚本定时任务表达式，例如 `@cron 0 * * * *`；只支持直接写 Cron 表达式 |
| `on_start` | boolean | 否 | `true` 时随系统启动执行一次，常用于初始化服务 |
| `web` | boolean | 否 | `true` 时作为 Web 常驻脚本启动；端口和路由由脚本内 Express 自己处理 |
| `module` | boolean | 否 | `true` 时表示为模块插件，不响应消息规则 |
| `carry` | boolean | 否 | 写 `@carry` 或 `@carry true` 时可作为 Admin 面板「搬运」里的处理脚本；`@carry false` 表示关闭 |
| `version` | string | 否 | 版本号，如 `v1.0.1` |
| `author` | string | 否 | 作者名 |
| `desc` | string | 否 | 插件描述 |
| `depe` | JSON array | 否 | 插件依赖声明，例如 `[depe: ["ipp"]]`；NodeJS 依赖由 pnpm 安装，Python 依赖由 pipx 安装；旧式 `@depe` 兼容 |
| `icon` | string | 否 | 插件图标 URL，未填写时使用默认苹果图标 |
| `public` | boolean | 否 | `true` 时允许发布到插件市场 |
| `disable` | boolean | 否 | `true` 时禁用插件 |
| `admin` | boolean | 否 | `true` 时仅管理员可触发 |


### 新式方括号注释

新式注释使用 `[key: value]` 元数据。NodeJS 用 `// [title: ...]`，Python 用 `# [title: ...]`；`//[title: ...]` / `#[title: ...]` 不带空格也能解析。

NodeJS 新式写法：

```js
// [title: 新式插件]
// [name: newPlugin]
// [description: 新式说明]
// [rule: ^新式命令$]
// [version: v1.0.0]
// [author: admin]
// [class: 工具]
// [depe: ["axios"]]
```

Python 新式写法：

```python
# [title: 新式Python插件]
# [name: newPythonPlugin]
# [description: 新式说明]
# [rule: ^新式命令$]
# [cron: 12 8 * * *]
# [version: v1.0.0]
# [depe: ["requests"]]
```

兼容字段包括：`title`、`name`、`description`（等同 `desc`）、`rule`、`cron`、`admin`、`priority`、`version`、`author`、`class`、`public`、`icon`、`module`、`carry`、`on_start`、`web`、`smallcat`、`depe` 等。新插件建议使用 `[title: ...]` / `[rule: ...]` / `[depe: ...]` 形式。

### 旧式 `@` 注释兼容

旧式 `@title ...` / `@rule ...` / `@depe ...` 仍兼容读取，但新插件不再推荐使用。

```js
/**
 * @title 旧式插件
 * @desc 旧式说明
 * @rule ^旧式命令$
 * @depe ["axios"]
 */
```

```python
r"""
@title 旧式Python插件
@desc 旧式说明
@rule raw ^旧式命令$
@depe ["requests"]
"""
```

`[param: {...}]` 已废弃且不再解析；插件配置必须使用 `form`：NodeJS 顶层 `new form({...})`，Python 顶层 `form({...})`。

### 元数据示例

```js
/**
 * @title 每日早报
 * @rule raw ^早报$
 * @priority 10
 * @version v1.2.1
 * @author cdle
 * @desc 每天早上9点推送新闻早报
 * @depe ["axios"]
 * @icon https://example.com/icon.png
 * @public true
 */
```

### Web 服务脚本

`[web: true]` / 旧式 `@web true` 只支持 `true` 或 `false`。写 `[web: true]` 后，SillyGirl 会在启动或脚本重载时把该脚本作为常驻脚本进程运行；HTTP 端口、路由前缀和监听逻辑全部由脚本自己决定。NodeJS 插件可以使用内置 `http` 模块或通过 `[depe: ...]` 自行声明 Web 框架；Python 插件可以使用 Python HTTP 框架或标准库自行监听。旧式 `@web` / `@depe` 仍兼容。

```js
/**
 * @title Web 示例
 * @web true
 * @class 工具
 */

const http = require("http");

http
  .createServer((req, res) => {
    if (req.url !== "/health") {
      res.writeHead(404);
      res.end();
      return;
    }
    res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
    res.end(JSON.stringify({ status: true, message: "ok", data: null }));
  })
  .listen(3001, () => console.log("web plugin listening on 3001"));
```

## 搬运处理脚本

Admin 面板「搬运」只负责按平台、群号和工作机器人匹配消息，然后把消息交给选中的处理脚本。脚本里自己决定是否过滤、回复或转发。

```js
/**
 * @title 搬运处理示例
 * @carry
 */

const { sender: s } = require("sillygirl");

const content = s.getContent();
if (content.includes("关键词")) {
  s.reply("处理后的内容");
}
```

## 脚本定时任务

脚本头部写了 `@cron` 后，会直接显示在 Admin 面板「定时任务」列表，并由框架自动注册定时执行。

```js
/**
 * @title 每小时执行
 * @cron 0 * * * *
 */

const { sender: s } = require("sillygirl");

console.log("当前触发平台：" + s.getPlatform());
```

如果在 Admin 面板「定时任务」里选择 `node 插件名.js` 或 `python 插件名.py` 创建任务，系统会自动把 Cron 表达式写回该脚本头部注释，避免生成重复的独立任务。

## 全局对象与 API

### sender (s)

当前消息的 Sender 对象，是插件中最核心的交互入口。

NodeJS 插件里可以直接调用同步方法；Python 插件里 SDK 方法是异步方法，需要在 `async def main()` 中 `await s.reply(...)`、`await s.getContent()`。

#### 用户信息

```js
s.getUserId()       // 获取用户ID（string）
s.getUserName()     // 获取用户昵称（string）
s.getChatId()       // 获取群聊ID，私聊时为空字符串（string）
s.getChatName()     // 获取群聊名称（string）
s.getMessageId()    // 获取消息ID（string）
s.getPlatform()     // 获取平台类型，如 "qq"、"web"（string）
s.getBotId()        // 获取当前机器人ID（string）
s.isAdmin()         // 判断用户是否为管理员（boolean）
```

#### 内容操作

```js
s.getContent()      // 获取消息原始内容（string）
s.setContent(text)  // 修改当前消息内容（影响后续插件匹配）
s.continue()        // 继续匹配后续规则（默认匹配成功即停止）
```

#### 回复消息

```js
s.reply("文本")     // 回复文本消息，返回 { message_id, error }
s.reply("文本1", "文本2")  // 多段回复
```

#### 参数捕获

```js
// 对于规则 "天气 [城市]"
s.param("城市")     // 通过名称获取捕获组
s.param(1)          // 通过索引获取第1个捕获组（从1开始）
s.get(1)            // param 的别名
s.getAllMatch()     // 获取所有匹配组（二维数组）
```

#### 群管功能

```js
s.kick(userId)          // 踢出群成员
s.unkick(userId)        // 取消踢出
s.ban(userId, duration) // 禁言，duration 为秒数
s.unban(userId)         // 解除禁言
s.recallMessage(messageId)  // 撤回消息
```

### Bucket(name)

持久化键值存储，数据自动持久化到 BoltDB 或 Redis。

```js
const bucket = Bucket("myapp");

// 基础读写
bucket.set("key", "value");
const value = bucket.get("key", "default_value");  // 支持默认值

// 类型自动转换
bucket.set("count", 100);        // 存储为数字
bucket.set("enabled", true);     // 存储为布尔值
bucket.set("data", { a: 1 });    // 存储为对象（自动 JSON 序列化）

// 监听变更
bucket.watch("key", (oldValue, newValue, key) => {
  console.log(`${key} changed: ${oldValue} -> ${newValue}`);
});

// 其他方法
bucket.keys();       // 获取所有键名（string[]）
bucket.getAll();     // 获取所有键值（object）
bucket.delete("key"); // 删除键
bucket.empty();      // 清空 bucket
bucket.len();        // 获取键数量（number）
```

**作用域说明**：每个 Bucket 是独立的命名空间，不同插件建议使用不同的 Bucket 名称，避免键冲突。

### form 配置表单

SillyGirl 支持声明式插件配置。Node 插件推荐直接用顶层 `form`：`new form({...})` 注册并读取配置，不需要再从 `utils` 取 schema。`form.string()` / `form.boolean()` / `form.select()` 等 helper 会生成配置表单结构；后台会在「插件设置」弹窗展示已注册的配置表单。

```js
/**
 * @title 配置示例
 * @rule raw ^配置测试$
 */

const { sender: s, form } = require("sillygirl");

const ConfigDB = new form({
  host: form.string()
    .title("服务地址")
    .description("例如 http://127.0.0.1:9090")
    .default("http://127.0.0.1:9090"),
  open: form.boolean()
    .title("启用开关")
    .default(false),
  delTime: form.number()
    .title("撤回时间")
    .description("0 表示不撤回")
    .default(0),
  mode: form.select([
    { label: "普通", value: "normal" },
    { label: "快速", value: "fast" },
  ]).title("模式"),
});

ConfigDB.get();
if (!Object.keys(ConfigDB.userConfig).length) {
  s.reply("请先到后台「插件配置」完成配置");
} else {
  s.reply("当前 host: " + ConfigDB.userConfig.host);
}
```

`new form({...})` 的对象参数固定使用字段名到 `form.*()` helper 的映射；不要传原生 JSON Schema。需要嵌套对象时只用 `form.object({...})`。

支持的 helper：

| helper | 说明 |
|------|------|
| `form.string()` | 字符串字段 |
| `form.number()` | 数字字段 |
| `form.integer()` | 整数字段 |
| `form.boolean()` | 布尔开关 |
| `form.array(item)` | 数组字段 |
| `form.object(props)` | 对象字段 |
| `form.select(options)` | 下拉/单选候选值，支持 `[{ label, value }]` 或 `{ value: label }` |
| `form.defaults(fields)` | 读取表单默认值 |

支持的链式方法包括：

| 方法 | 说明 |
|------|------|
| `title(text)` | 字段标题 |
| `description(text)` | 字段说明 |
| `default(value)` | 默认值 |
| `options(values)` | 可选值 |
| `format(value)` | 字段格式，例如 `password`、`textarea` |
| `widget(value)` | UI 组件提示，例如 `radio`、`password`、`textarea` |
| `setMin(value)` / `setMax(value)` | 数字范围 |
| `setMinLength(value)` / `setMaxLength(value)` | 字符串长度 |
| `setPattern(value)` | 字符串正则约束 |

`new form(fields)` 返回的配置实例属性和方法：

```js
ConfigDB.jsonSchema   // 当前插件配置 schema
ConfigDB.userConfig   // 当前用户配置对象
ConfigDB.get()        // 从存储重新读取配置
ConfigDB.set()        // 保存 ConfigDB.userConfig
ConfigDB.set(obj)     // 保存指定配置对象
```

注意：配置 schema 会在插件顶层执行到 `new form({...})` 时注册。新插件首次安装后，
如果后台「插件配置」里还看不到它，先触发一次插件规则或把插件声明为 `@on_start true`。

Python 插件不用 `new`，也可以直接使用 `form.string()` 这一套 helper：

```python
from sillygirl import form

config = form({
    "host": form.string()
        .title("服务地址")
        .default("http://127.0.0.1:9090"),
    "enabled": form.boolean()
        .title("启用")
        .default(False),
})


async def main():
    values = await config.get()
    print(values)
```

### container 容器入口

`container` 是容器面板统一入口：

```js
const { container } = require("sillygirl");

const all = await container.getList();       // { smallcat, qinglong, daidai }
const qlList = await container.getList("qinglong");
const ql1 = new container.QingLong({ id: 1 });
const envs = await ql1.getEnvs();
```

`container.getList()` 返回后台已绑定的 smallcat / 青龙 / 呆呆容器数量和只读列表；`container.QingLong`、`container.SmallCat`、`container.DaiDai` 负责继续调用对应面板 API。

### QingLong 内联客户端

`container` 顶层导出负责容器列表和面板客户端。`container.QingLong` 是青龙面板的脚本内联客户端。先在 Admin 面板左侧「青龙容器」中添加青龙面板，再在脚本里按页面表格编号创建实例。

```js
const ql = new container.QingLong({ id: 1 });
```

构造参数必须是对象：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | number/string | 是 | 青龙容器页面中的顺序编号，从 `1` 开始 |

实例基础属性：

```js
ql.id       // 当前编号
ql.uuid     // 面板内部 UUID
ql.name     // 面板名称
ql.address  // 青龙地址
```

常用环境变量 API：

| 方法 | 对应青龙 Open API | 参数 | 返回 |
|------|-------------------|------|------|
| `getEnvs(options)` | `GET /open/envs` | `{ searchValue?: string }` 或搜索字符串 | 青龙返回的 `data` |
| `getEnvById(id)` | `GET /open/envs/:id` | 环境变量 ID | 青龙返回的 `data` |
| `createEnv(env)` | `POST /open/envs` | 单个环境变量对象或数组 | 青龙返回的 `data` |
| `updateEnv(env)` | `PUT /open/envs` | 环境变量对象，需包含 `id` | 青龙返回的 `data` |
| `deleteEnvs(ids)` | `DELETE /open/envs` | ID、ID 数组或逗号分隔字符串 | 青龙返回的 `data` |
| `moveEnv(id, fromIndex, toIndex)` | `PUT /open/envs/:id/move` | 环境变量 ID、原位置、新位置 | 青龙返回的 `data` |
| `disableEnvs(ids)` | `PUT /open/envs/disable` | ID、ID 数组或逗号分隔字符串 | 青龙返回的 `data` |
| `enableEnvs(ids)` | `PUT /open/envs/enable` | ID、ID 数组或逗号分隔字符串 | 青龙返回的 `data` |
| `updateEnvNames(ids, name)` | `PUT /open/envs/name` | ID 集合、新变量名 | 青龙返回的 `data` |
| `systemNotify(title, content)` | `PUT /open/system/notify` | 标题、内容 | 青龙返回的 `data` |

通用调用：

```js
ql.request(method, path, body, query);
```

`request` 会自动加青龙 `Bearer` token，并返回青龙原始响应对象，适合调用上表之外的 Open API。

示例：

```js
const ql = new container.QingLong({ id: 1 });

const envs = ql.getEnvs({ searchValue: "JD_COOKIE" });
console.log("匹配数量", envs.length);

const created = ql.createEnv({
  name: "TEST_TOKEN",
  value: "123456",
  remarks: "脚本创建测试",
});

ql.disableEnvs([created[0].id]);
ql.enableEnvs([created[0].id]);
ql.deleteEnvs([created[0].id]);
```

注意：

- `new container.QingLong({ id: 1 })` 只接受对象参数，不支持 `new container.QingLong(1)`。
- 编号按「青龙容器」页面当前列表顺序，从 `1` 开始。
- 除 `request` 外，封装方法会在青龙业务 `code != 200` 或 HTTP 非 2xx 时抛出脚本异常。

### SmallCat 内联客户端

`container` 顶层导出负责容器列表和面板客户端。`container.SmallCat` 是 smallcat 面板的脚本内联客户端。先在 Admin 面板左侧「smallcat」中添加地址和 `api_auth`，再在脚本里按页面表格编号创建实例。

```js
const sc = new container.SmallCat({ id: 1 });
```

构造参数必须是对象：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | number/string | 是 | smallcat 页面中的顺序编号，从 `1` 开始 |

实例基础属性：

```js
sc.id       // 当前编号
sc.uuid     // 面板内部 UUID
sc.name     // 面板名称
sc.address  // smallcat 地址
```

接口方法：

| 方法 | 对应 smallcat API | 参数 | 返回 |
|------|-------------------|------|------|
| `createQr(type)` | `POST /api/qr/start` | 登录来源类型，如 `1` | 原始 API 响应 |
| `createQr(options)` | `POST /api/qr/start` | `{ type, openid?, proxyNodeId? }` 等对象 | 原始 API 响应 |
| `checkQr(uuid)` | `GET /api/qr/status?uuid=...` | 二维码 UUID | 原始 API 响应 |
| `addUser(options)` | `POST /api/accounts/add` | `{ code, displayName?, oauthState?, ... }` | 原始 API 响应 |
| `rescanUser(options)` | `POST /api/accounts/rescan` | `{ openid, code, displayName?, oauthState? }` | 原始 API 响应 |
| `userList()` | `GET /api/accounts` | 无 | 原始 API 响应 |
| `checkUsers(options)` | `POST /api/accounts/status` | `{ openid }`，`openid` 可为字符串或数组 | 原始 API 响应 |
| `setUserRemark(options)` | `POST /api/accounts/remark` | `{ openid, displayName }` | 原始 API 响应 |
| `setUserDisabled(options)` | `POST /api/accounts/disable` | `{ openid, disabled }` | 原始 API 响应 |
| `deleteUser(options)` | `POST /api/accounts/delete` | `{ openid }` | 原始 API 响应 |
| `proxyList()` | `GET /api/proxies` | 无 | 原始 API 响应 |
| `testProxy(options)` | `POST /api/proxies/test` | 代理节点对象 | 原始 API 响应 |
| `addProxy(options)` | `POST /api/proxies/add` | 代理节点对象 | 原始 API 响应 |
| `deleteProxy(options)` | `POST /api/proxies/delete` | `{ id }` | 原始 API 响应 |
| `creditBalance()` | `GET /credits/balance` | 无 | 原始 API 响应 |
| `creditLedger(query?)` | `GET /credits/ledger` | 查询对象或条数，如 `{ limit: 50 }` / `50` | 原始 API 响应 |
| `getCode(options)` | `POST /wx/code` | `{ openid, appid }` | 原始 API 响应 |
| `getSession(options)` | `POST /wx/getsession` | `{ openid, appid }` | 原始 API 响应 |
| `refreshSession(options)` | `POST /wx/refresh` | `{ openid, appid }` | 原始 API 响应 |
| `getUserInfo(options)` | `POST /wx/getuserinfo` | `{ openid, appid }` | 原始 API 响应 |
| `getEncryptKey(options)` | `POST /wx/encryptkey` | `{ openid, appid }` | 原始 API 响应 |
| `getPhoneNumber(options)` | `POST /wx/getphonenumber` | `{ openid, appid }` | 原始 API 响应 |
| `cloud(options)` | `POST /wx/cloud` | `{ openid, appid, function_name, data }` | 原始 API 响应 |
| `gateway(options)` | `POST /wx/gateway` | `{ openid, appid, action, env }` 或完整 `domain` | 原始 API 响应 |
| `qrCodeAuth(options)` | `POST /wx/qrcodeauth` | `{ openid, uuid }` | 原始 API 响应 |
| `oAuth(options)` | `POST /wx/oauth` | `{ openid, appid, redirect_uri, scope?, state?, component_appid? }` 等 | 原始 API 响应 |
| `translateLink(options)` | `POST /wx/translatelink` | `{ openid, link, scene? }` | 原始 API 响应 |
| `autoAuth(options)` | `POST /wx/autoauth` | `{ openid }` | 原始 API 响应 |
| `appMsgExt(options)` | `POST /wx/appmsgext` | `{ openid, article_url }` | 原始 API 响应 |
| `appMsgLike(options)` | `POST /wx/appmsglike` | `{ openid, article_url }` | 原始 API 响应 |
| `request(method, path, body, query)` | 任意 smallcat API | 自定义方法、路径、请求体、查询参数 | 原始 API 响应 |

smallcat 运行时不会改写 API 返回。脚本收到的就是 smallcat 原始 JSON，一般结构为：

```js
{
  status: true,
  message: "成功",
  data: {}
}
```

示例：

```js
const sc = new container.SmallCat({ id: 1 });

const qr = sc.createQr(1);
if (!qr.status) {
  s.reply("生成二维码失败：" + qr.message);
  return;
}

s.reply("扫码地址：" + qr.data.qrcodeUrl);

const checked = sc.checkQr(qr.data.uuid);
if (checked.data.state === "confirmed" && checked.data.wxCode) {
  const saved = sc.addUser({
    code: checked.data.wxCode,
    displayName: "备注",
  });
  s.reply(saved.message);
} else {
  s.reply("当前扫码状态：" + checked.data.state);
}

const users = sc.userList();
console.log(users.status, users.message, users.data && users.data.items);

const code = sc.getCode({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
console.log(code.status, code.message, code.data);

const session = sc.getSession({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
console.log(session.data && session.data.session);

const refreshed = sc.refreshSession({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
console.log(refreshed.data && refreshed.data.expireIn);

const userInfo = sc.getUserInfo({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
console.log(userInfo.status, userInfo.message, userInfo.data);

const phone = sc.getPhoneNumber({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
console.log(phone.status, phone.message, phone.data);

const oauth = sc.oAuth({
  openid: "用户 openid",
  appid: "wx2f5d8f9715c59d10",
  redirect_uri: "https://example.com/callback",
  scope: "snsapi_userinfo",
  state: "STATE",
});
console.log(oauth.status, oauth.message, oauth.data);

const qrOAuth = sc.qrCodeAuth({
  openid: "用户 openid",
  uuid: "二维码 UUID",
});
console.log(qrOAuth.status, qrOAuth.message, qrOAuth.data);
```

注意：

- `new container.SmallCat({ id: 1 })` 只接受对象参数，不支持 `new container.SmallCat(1)`。
- `addUser` 只接受对象参数，推荐写 `sc.addUser({ code: "xxxxx", displayName: "备注" })`；重扫已有账号使用 `rescanUser`。
- 只有网络失败、请求体编码失败、JSON 解析失败这类没有 smallcat 原始响应的情况，运行时才会返回 `{ status: false, message: "..." }`。

### DaiDai 内联客户端

`container` 顶层导出负责容器列表和面板客户端。`container.DaiDai` 是呆呆面板的脚本内联客户端。先在 Admin 面板左侧「呆呆面板」中添加地址、`app_key`、`app_secret`，再在脚本里按页面表格编号创建实例。

```js
const dd = new container.DaiDai({ id: 1 });
```

构造参数必须是对象：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | number/string | 是 | 呆呆面板页面中的顺序编号，从 `1` 开始 |

实例基础属性：

```js
dd.id       // 当前编号
dd.uuid     // 面板内部 UUID
dd.name     // 面板名称
dd.address  // 呆呆面板地址
```

环境变量方法：

| 方法 | 对应呆呆面板 API | 参数 | 返回 |
|------|------------------|------|------|
| `getEnvs(options)` | `GET /api/envs` | `{ keyword?: string, page?: number, page_size?: number }` 或搜索字符串 | 呆呆返回的 `data` |
| `getEnvById(id)` | `GET /api/envs/:id` | 环境变量 ID | 呆呆返回的 `data` |
| `createEnv(env)` | `POST /api/envs` | 单个环境变量对象或数组 | 呆呆返回的 `data` |
| `updateEnv(env)` | `PUT /api/envs/:id` | 环境变量对象，建议包含 `id` | 呆呆返回的 `data` |
| `deleteEnv(id)` | `DELETE /api/envs/:id` | 环境变量 ID | 原始 API 响应 |
| `deleteEnvs(ids)` | `DELETE /api/envs/batch` | ID、ID 数组或逗号分隔字符串 | 原始 API 响应 |
| `enableEnv(id)` / `disableEnv(id)` | `PUT /api/envs/:id/enable` / `disable` | 环境变量 ID | 呆呆返回的 `data` |
| `enableEnvs(ids)` / `disableEnvs(ids)` | `PUT /api/envs/batch/enable` / `disable` | ID 集合 | 原始 API 响应 |

任务方法：

| 方法 | 对应呆呆面板 API | 参数 | 返回 |
|------|------------------|------|------|
| `getTasks(options)` | `GET /api/tasks` | `{ keyword?: string, page?: number, page_size?: number }` 或搜索字符串 | 呆呆返回的 `data` |
| `getTaskById(id)` | `GET /api/tasks/:id` | 任务 ID | 呆呆返回的 `data` |
| `createTask(task)` | `POST /api/tasks` | 任务对象 | 呆呆返回的 `data` |
| `updateTask(task)` | `PUT /api/tasks/:id` | 任务对象，建议包含 `id` | 呆呆返回的 `data` |
| `deleteTask(id)` | `DELETE /api/tasks/:id` | 任务 ID | 原始 API 响应 |
| `runTask(id)` / `stopTask(id)` | `PUT /api/tasks/:id/run` / `stop` | 任务 ID | 原始 API 响应 |
| `enableTask(id)` / `disableTask(id)` | `PUT /api/tasks/:id/enable` / `disable` | 任务 ID | 原始 API 响应 |

通用调用：

```js
dd.request(method, path, body, query);
```

`request` 会自动通过 `/api/open-api/token` 获取 `Bearer` token，并返回呆呆面板原始响应对象，适合调用上表之外的 API。

示例：

```js
const dd = new container.DaiDai({ id: 1 });

const envs = dd.getEnvs({ keyword: "JD_COOKIE" });
console.log("匹配数量", envs.length);

const created = dd.createEnv({
  name: "TEST_TOKEN",
  value: "123456",
  remarks: "脚本创建测试",
});

dd.disableEnv(created.id);
dd.enableEnv(created.id);
dd.deleteEnv(created.id);
```

注意：

- `new container.DaiDai({ id: 1 })` 只接受对象参数，不支持 `new container.DaiDai(1)`。
- 编号按「呆呆面板」页面当前列表顺序，从 `1` 开始。
- 除 `request` 外，封装方法默认返回呆呆响应里的 `data`；HTTP 非 2xx 或 `success: false` 会抛出脚本异常。

### Cron()

定时任务调度器，支持标准 Cron 表达式。

```js
const task = Cron();

// 添加任务（6字段：秒 分 时 日 月 周）
const { id, error } = task.add("*/5 * * * * *", () => {
  console.log("每5秒执行一次");
});

// 添加任务（5字段：分 时 日 月 周，秒自动补0）
const { id, error } = task.add("0 9 * * *", () => {
  console.log("每天早上9点执行");
});

// 移除任务
task.remove(id);
```

> 注意：脚本头部写了 `@cron` 会自动同步到管理后台“定时任务”。

### 其他全局函数

```js
sleep(ms)           // 同步阻塞睡眠（毫秒）
md5(str)            // MD5 哈希
uuid()              // 生成 UUID
running()           // 判断当前插件是否仍在运行（boolean）
fmt.Sprintf(format, ...args)  // 格式化字符串
fmt.Printf(format, ...args)   // 格式化输出
time.Now()          // 获取当前时间对象
time.Sleep(ms)      // 睡眠（毫秒）
time.Unix(sec)      // Unix 时间戳转时间对象
time.Parse(str, layout, locale)  // 解析时间字符串
```

## 规则匹配语法

规则（`@rule`）支持多种语法，框架会自动转换为正则表达式进行匹配。

### 基础规则

```js
/**
 * @rule raw ^你好$      // 原始正则，完全匹配"你好"
 * @rule raw ^/help$     // 原始正则，匹配 "/help"
 * @rule 你好            // 自动转换为 ^你好$，完全匹配
 * @rule ^天气           // 以"天气"开头
 * @rule 帮助$           // 以"帮助"结尾
 */
```

### 参数捕获

```js
/**
 * @rule 天气 [城市]              // 匹配"天气 北京"，捕获"北京"
 * @rule [操作:登录,注册,退出]     // 匹配"登录"、"注册"或"退出"
 * @rule 查询 ?                   // ? 匹配任意非空白字符
 * @rule 搜索 *                   // * 匹配任意内容（包括空白）
 */
```

### 可选参数

```js
/**
 * @rule 天气 [城市?]             // ? 表示可选，匹配"天气"或"天气 北京"
 */
```

### 优先级与冲突

当多条规则可能同时匹配一条消息时，框架按以下顺序决定执行：

1. **监听规则**（`s.listen` 注册的）优先于普通规则
2. **高 priority** 优先于低 priority
3. 同一优先级下，先加载的插件优先

使用 `s.continue()` 可以让当前插件执行完毕后继续匹配后续规则：

```js
/**
 * @title 日志中间件
 * @rule *
 * @priority 999
 */

console.log("收到消息:", s.getContent());
s.continue();  // 继续让其他插件处理
```

## 消息监听与会话

`s.listen()` 是实现对话式交互的核心 API，支持等待用户后续输入并按规则匹配。

### 基础用法

```js
/**
 * @title 注册流程
 * @rule raw ^注册$
 */

s.reply("请输入你的用户名：");
const result = s.listen({
  rules: ["raw ^(.+)$"],  // 捕获任意输入
  timeout: 30000,          // 30秒超时
  handle: (s2) => {
    const username = s2.param(1);
    s2.reply(`用户名 "${username}" 注册成功！`);
  },
});

if (!result) {
  s.reply("注册超时，请重试。");
}
```

### 监听选项

```js
s.listen({
  rules: ["规则1", "规则2"],        // 匹配规则数组
  timeout: 10000,                   // 超时时间（毫秒）
  handle: (s2) => { ... },          // 匹配后的回调函数
  private: true,                    // 允许在私聊中触发
  group: true,                      // 允许在群聊中触发
  require_admin: false,             // 是否要求管理员权限
  allow_platforms: ["qq"],          // 限制平台
  prohibit_platforms: ["web"],      // 禁止平台
  allow_users: ["12345"],           // 仅允许指定用户
  allow_groups: ["67890"],          // 仅允许指定群组
  user_id: s.getUserId(),           // 仅监听当前用户
  chat_id: s.getChatId(),           // 仅监听当前群组
});
```

### 持久化监听

添加 `"persistent"` 参数可创建长期有效的监听器：

```js
/**
 * @title 关键词监控
 * @rule raw ^启动监控$
 */

s.listen({
  rules: ["raw ^(.+)$"],
  timeout: 0,           // 0 表示永不超时
  handle: (s2) => {
    console.log("监控到消息:", s2.getContent());
    // 返回空字符串或 undefined 会继续监听
  },
}, "persistent");

s.reply("监控已启动");
```

### HoldOn 与 GoAgain

```js
/**
 * @title 循环输入
 * @rule raw ^开始$
 */

function ask() {
  s.reply("请输入内容（输入'结束'停止）：");
  const r = s.listen({
    rules: ["raw ^(.+)$"],
    timeout: 30000,
    handle: (s2) => {
      if (s2.param(1) === "结束") {
        s2.reply("已结束");
        return;
      }
      s2.reply("你输入了：" + s2.param(1));
      return s2.holdOn("开始");  // 重新触发当前插件
    },
  });
}
ask();
```

## 定时任务

定时任务可以在脚本头部声明 `@cron`，也可以在管理后台的“定时任务”中创建。

```js
/**
 * @title 每日提醒
 * @cron 0 9 * * *
 */

s.reply("该起床啦！今天是 " + time.Now().Format("2006-01-02"));
```

Cron 表达式格式（5字段或6字段）：

```
秒(可选) 分 时 日 月 周
```

| 字段 | 范围 | 特殊字符 |
|------|------|----------|
| 秒 | 0-59 | `, - * /` |
| 分 | 0-59 | `, - * /` |
| 时 | 0-23 | `, - * /` |
| 日 | 1-31 | `, - * /` |
| 月 | 1-12 | `, - * /` |
| 周 | 0-6（0=周日）| `, - * /` |

示例：

| 表达式 | 说明 |
|--------|------|
| `*/5 * * * *` | 每5分钟 |
| `0 */1 * * *` | 每小时 |
| `0 9 * * 1-5` | 工作日早上9点 |
| `0 0 1 * *` | 每月1号零点 |
| `*/30 * * * * *` | 每30秒 |

## 完整示例

### 示例 1：记忆名字

```js
/**
 * @title 记忆名字
 * @rule raw ^我是谁$
 * @rule 我是[姓名]
 * @version v1.0.1
 * @author cdle
 */

const user = Bucket("user_names");
const name = s.param("姓名");

if (!name) {
  const stored = user.get(s.getUserId());
  if (stored) {
    s.reply(`你是 ${stored}`);
  } else {
    s.reply("我还不知道你是谁，告诉我吧：我是[你的名字]");
  }
} else {
  user.set(s.getUserId(), name);
  s.reply(`好的，我记住你了，${name}！`);
}
```

### 示例 2：倒计时提醒

```js
/**
 * @title 倒计时
 * @rule 倒计时 [分钟:1,2,3,5,10] 分钟
 * @version v1.0.1
 */

const minutes = parseInt(s.param(1));
s.reply(`好的，${minutes}分钟后提醒你。`);

// 使用 Cron 不太适合一次性延时，这里用 sleep
// 注意：sleep 会阻塞，长时间任务建议用 cron 或其他方式
go(() => {
  sleep(minutes * 60 * 1000);
  s.reply("⏰ 时间到了！");
});
```

> 注：实际开发中长时间后台任务建议使用管理后台的“定时任务”或外部调度。

## 调试技巧

### 1. 使用终端模式

```bash
./sillyGirl -t
```

终端模式是开发插件的最快方式，修改插件后立即生效，无需重启。

### 2. 开启调试模式

在 Admin 面板将 `sillyGirl.debug` 设为 `true`，或在插件中：

```js
console.log("调试信息:", someVariable);
console.debug("详细调试:", detailedInfo);
console.error("错误:", err);
```

### 3. 查看插件状态

在 Admin 面板的"插件"页面，可以查看：
- 插件加载状态
- 规则列表
- 错误日志
- 性能统计

### 4. 安全执行

插件运行异常时框架会自动捕获 panic，不会影响其他插件或核心服务。你可以在 Admin 面板查看具体的错误堆栈。

### 5. 模块复用

将公共逻辑抽取为 `@module true` 插件，其他插件通过 `require` 或全局变量复用：

```js
/**
 * @title 工具模块
 * @module true
 */

// 定义全局工具函数
RegistFuncs["utils"] = {
  formatTime: (t) => t.Format("2006-01-02"),
  isWeekend: (t) => t.Weekday() === 0 || t.Weekday() === 6,
};
```


迁移旧插件时不要依赖外部兼容脚本；插件应单文件运行，只从 `sillygirl` 导入 `sender`、`Sender`、`Bucket`、`container`、`utils`、`form` 等现有能力。
