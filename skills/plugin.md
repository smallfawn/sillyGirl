---
name: sillygirl-plugin-writer
description: Use when writing, migrating, reviewing, or fixing SillyGirl NodeJS or Python script plugins. Covers plugin metadata comments, message rules, cron/web/carry scripts, configuration schemas, sender/Bucket APIs, dependency declarations, and inline QingLong/SmallCat/DaiDai clients.
---

# SillyGirl Plugin Writer

Use this skill to write SillyGirl script plugins for this repository.

## Ground Rules

- Prefer CommonJS NodeJS plugins unless the user asks for Python.
- Python plugins must target Python 3.12 and await all SillyGirl SDK calls.
- Import NodeJS runtime APIs from `sillygirl`:

```js
const {
  sender: s,
  Bucket,
  container,
  form,
  utils,
} = require('sillygirl');

```

Use `new container.QingLong({ id })`, `new container.SmallCat({ id })`, `new container.DaiDai({ id })` for panel clients. Use `utils.userList()`, `utils.sleep()`, `utils.version()`, `utils.restart()`, `utils.update()` for system helpers.

- Import Python runtime APIs from `sillygirl`:

```python
from sillygirl import sender as s, Bucket, container, form, utils

```

Use `container.QingLong({"id": 1})`, `container.SmallCat({"id": 1})`, `container.DaiDai({"id": 1})`.

- Do not use Goja-only APIs or BNCR globals.
- Do not use `BncrDB`, `BncrCreateSchema`, or `BncrPluginConfig`.
- Do not invent wrappers that change third-party API response shapes. Return or reply with the original API meaning unless the user asks for formatting.
- Prefer `async function main() { ... }` and end with `main().catch(...)`.
- Always handle exceptions and reply with a useful error message.
- Never hard-code secrets in plugin code. Use `new form({...})`, `Bucket`, or environment variables.
- Declare third-party dependencies with new-style `[depe: ["axios"]]` / `[depe: ["requests"]]`; legacy `@depe ...` metadata is still detected for migration. NodeJS uses pnpm, Python uses pipx. 

> 迁移旧插件时不要依赖外部兼容脚本，也不要随插件安装额外文件；插件应单文件运行，只从 `sillygirl` 导入 `sender`、`Sender`、`Bucket`、`container`、`utils`、`form` 等现有能力。
Do not use `[param: {...}]`; plugin configuration must use top-level `new form({...})` in NodeJS or `form({...})` in Python.

## Metadata Header

New-style plugins should start with square-bracket metadata comments. NodeJS uses `// [key: value]`; Python uses `# [key: value]`. Spacing after `//` or `#` is optional.

```js
// [title: 插件标题]
// [name: pluginFileName]
// [description: 插件说明]
// [author: 作者]
// [version: v1.0.0]
// [rule: ^命令$]
// [public: true]
// [class: 工具]
// [depe: ["axios"]]
```

```python
# [title: Python插件标题]
# [name: pythonPluginFileName]
# [description: 插件说明]
# [author: 作者]
# [version: v1.0.0]
# [rule: raw ^命令$]
# [public: true]
# [class: 工具]
# [depe: ["requests"]]
```

Legacy `@title ...` comments are accepted for migration compatibility, but new plugins should use the `[title: ...]` style above.

```js
/**
 * @title Legacy Node
 * @desc Legacy description
 * @rule ^legacy$
 * @depe ["axios"]
 */
```

Do not write `[param: {...}]`; it is intentionally ignored. Use `form` for all plugin settings.

Supported metadata:

| Tag | Required | Meaning |
| --- | --- | --- |
| `[title: 标题]` | Recommended | Display name in Admin and plugin market; legacy `@title` metadata is still accepted. |
| `[name: pluginFileName]` | Required for local market creation | Script file name without suffix. |
| `[author: 作者]` | Optional | Plugin author. |
| `[version: v1.0.1]` | Optional | Plugin version. |
| `[description: 说明]` | Optional | Plugin description. `[desc: ...]` is accepted for compatibility. |
| `[icon: URL]` | Optional | Plugin icon URL. If omitted, SillyGirl uses the default apple icon. |
| `[rule: 规则]` | Required for message plugins | Message trigger. Can appear multiple times. |
| `[admin: true/false]` | Optional | Whether only admins can trigger it. |
| `[class: 分类]` | Optional | Plugin market category. |
| `[public: true/false]` | Optional | Whether it can be listed publicly. Local manual creation is forced to false. |
| `[cron: 表达式]` | Optional | Cron expression only. |
| `[web: true/false]` | Optional | Whether the plugin stays running. |
| `[carry: true]` | Optional | Makes the plugin selectable as a carry processing script. |
| `[module: true]` | Optional | Utility/module file, not a normal message handler. |
| `[on_start: true]` | Optional | Run once on startup. |
| `[depe: ["pkg"]]` | Optional | Runtime dependencies. |

Do not use removed/legacy tags such as `[param: ...]`, `@name`, `@form`, `@encrypt`, `@http`, `@findall`, `@match`, `@regex`, `@pattern`.

## Rule Patterns

Use simple rules unless exact regex is needed:

```js
/**
 * @title 示例
 * @rule raw ^ping$
 * @rule 天气 [城市]
 */
```

- Use `raw ^...$` for exact regex.
- Use `[参数名]` for captured parameters.
- Read captured text from `s.param(index)` when needed.
- If the script has no `@rule`, it will not be triggered by normal messages.

## Sender API

Use `sender` as `s`:

```js
const content = await s.getContent();
const userId = await s.getUserId();
const chatId = await s.getChatId();
const isAdmin = await s.isAdmin();
await s.reply('回复内容');
```

For Python, always await SDK calls:

```python
import asyncio
from sillygirl import sender as s


async def main():
    content = await s.getContent()
    await s.reply('收到：' + content)


asyncio.run(main())
```

Common methods:

- `s.getContent()`
- `s.getUserId()`
- `s.getChatId()`
- `s.getPlatform()`
- `s.getBotId()`
- `s.isAdmin()`
- `s.param(index)`
- `s.reply(text)`

For admin-only commands, still check `await s.isAdmin()` in code when the action is sensitive, even if `@admin true` is present.

## Storage

Use `Bucket` for persistent plugin data:

```js
const db = new Bucket('my-plugin');
const oldValue = await db.get('key', '');
await db.set('key', 'value');
await db.delete('key');
```

Python:

```python
from sillygirl import Bucket

db = Bucket('my-plugin')
value = await db.get('key', '')
await db.set('key', 'value')
await db.delete('key')
```

Use one bucket per plugin or feature. Avoid writing to shared buckets like `sillyGirl` unless the user explicitly asks.

## Plugin Configuration

Use only the chain-style `new form({...})` at top level. Do not use raw JSON Schema, `utils.schema`, `form.schema`, `SillyGirlPluginConfig`, or `setTitle/setDefault/setEnum` aliases.

```js
const { form } = require('sillygirl');

const Config = new form({
  apiBase: form.string().title('接口地址').default(''),
  token: form.string().title('Token').format('password').default(''),
  open: form.boolean().title('是否启用').default(false),
  mode: form.select([{ label: '自动', value: 'auto' }, { label: '手动', value: 'manual' }])
    .title('模式')
    .default('auto'),
});
```

Read config values:

```js
const values = await Config.get();
const token = values.token || '';
```

Config registration must run at plugin load time, not inside a branch that may never execute.

## Python Plugins

Use Python only when requested or when the plugin naturally benefits from Python libraries.

Python plugin header:

```python
"""
"""
@title Python示例
@rule raw ^你好$
@version v1.0.1
@depe ["requests"]
"""
"""
```

Runtime facts:

- Python version is fixed to 3.12.
- Built-in runtime dependencies are `grpcio==1.83.0` and `protobuf==7.35.1`.
- Third-party dependencies are installed by pipx into `/data/plugins/python_packages`.
- Standard-library modules such as `os`, `sys`, `json`, `asyncio`, `time`, and `pathlib` should not be listed in `@depe ...`.
- All SDK calls are async and must be awaited.

Minimal Python plugin:

```python
"""
* @title Python你好
* @rule raw ^你好$
* @version v1.0.1
"""

import asyncio
from sillygirl import sender as s


async def main():
    await s.reply("你也好")


asyncio.run(main())
```

Python inline clients:

```python
ql = container.QingLong({"id": 1})
envs = await ql.getEnvs({"searchValue": "JD_COOKIE"})

sc = container.SmallCat({"id": 1})
code = await sc.getCode({"openid": "openid", "appid": "wx123"})

dd = container.DaiDai({"id": 1})
items = await dd.getEnvs({"keyword": "JD_COOKIE"})
```

## Inline Clients

Constructors use object parameters only:

```js
const ql = new container.QingLong({ id: 1 });
const sc = new container.SmallCat({ id: 1 });
const dd = new container.DaiDai({ id: 1 });
```

Do not write:

```js
new container.QingLong(1);
new container.SmallCat(1);
new container.DaiDai(1);
```

### QingLong

Common methods:

- `ql.getEnvs(search?)`
- `ql.getEnvById(id)`
- `ql.createEnv({ name, value, remarks })`
- `ql.updateEnv({ id, name, value, remarks })`
- `ql.deleteEnvs(ids)`
- `ql.enableEnvs(ids)`
- `ql.disableEnvs(ids)`
- `ql.systemNotify(title, content)`

### SmallCat

Common methods:

- `sc.createQr(type)`
- `sc.checkQr(uuid)`
- `sc.addUser(payload)` / `sc.rescanUser(payload)`
- `sc.userList()`
- `sc.checkUsers(payload)`
- `sc.setUserRemark(payload)` / `sc.setUserDisabled(payload)` / `sc.deleteUser(payload)`
- `sc.proxyList()` / `sc.testProxy(payload)` / `sc.addProxy(payload)` / `sc.deleteProxy(payload)`
- `sc.creditBalance()` / `sc.creditLedger(query)`
- `sc.getCode({ openid, appid })`
- `sc.getSession({ openid, appid })` / `sc.refreshSession({ openid, appid })`
- `sc.getUserInfo({ openid, appid })`
- `sc.getEncryptKey({ openid, appid })`
- `sc.getPhoneNumber({ openid, appid })`
- `sc.cloud(payload)` / `sc.gateway(payload)`
- `sc.qrCodeAuth(payload)`
- `sc.oAuth(payload)`
- `sc.translateLink(payload)` / `sc.autoAuth(payload)`
- `sc.appMsgExt(payload)` / `sc.appMsgLike(payload)`

Use camelCase exactly. Do not write `chechQr`.

### DaiDai

Common methods should follow the project inline client implementation. If unsure, inspect `core/node_runtime_preload.go` before using a method.

## Web Plugins

For HTTP plugins:

```js
/**
 * @title Web 示例
 * @web true
 * @version v1.0.1
 */

const http = require('http');

http
  .createServer((req, res) => {
    if (req.url !== '/health') {
      res.writeHead(404);
      res.end();
      return;
    }
    res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' });
    res.end(JSON.stringify({ status: true, message: 'ok', data: null }));
  })
  .listen(3001, () => {
    console.log('web plugin listening on 3001');
  });
```

Python HTTP plugin:

```python
"""
* @title PythonWeb示例
* @web true
* @version v1.0.1
"""

from http.server import BaseHTTPRequestHandler, HTTPServer
import json


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != '/health':
            self.send_response(404)
            self.end_headers()
            return

        body = json.dumps(
            {'status': True, 'message': 'ok', 'data': None},
            ensure_ascii=False,
        ).encode('utf-8')
        self.send_response(200)
        self.send_header('Content-Type', 'application/json; charset=utf-8')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)


HTTPServer(('0.0.0.0', 3002), Handler).serve_forever()
```

`@web` only accepts `true` or `false`. Do not put a port in the metadata.

## Carry Plugins

Carry handlers must include `@carry true` so Admin can select them:

```js
/**
 * @title 搬运处理
 * @carry true
 * @version v1.0.1
 */

const { sender: s } = require('sillygirl');

async function main() {
  const content = await s.getContent();
  await s.reply(content);
}

main().catch(err => s.reply(`搬运处理失败：${err.message || err}`));
```

## Cron Plugins

Cron plugins should include `@cron`:

```js
/**
 * @title 每日提醒
 * @cron 0 9 * * *
 * @version v1.0.1
 */

const { sender: s } = require('sillygirl');

async function main() {
  await s.reply('每日提醒');
}

main().catch(err => console.error(err));
```

## Response Style

- For chat commands, reply in concise Chinese.
- For API helper plugins, preserve original JSON response when the user needs raw results.
- For status summaries, include enough fields to debug: status, message, id/openid/uuid when relevant.
- Mask secrets before replying.

## Final Checklist

Before finishing a plugin:

- Header uses new-style `[title: ...]` and `[name: pluginFileName]`; legacy `@title ...` is accepted only for migration.
- Description uses `[description: ...]`; `[desc: ...]` is accepted for compatibility.
- No BNCR names remain.
- Constructors are `new container.QingLong({ id })`, `new container.SmallCat({ id })`, `new container.DaiDai({ id })`.
- `SmallCat.checkQr` is spelled correctly.
- Sensitive commands check admin permission.
- External requests have timeouts or are wrapped in try/catch.
- Config schema is registered at top level if configuration is needed.
- Code is valid CommonJS and can run under NodeJS.
- Python code uses Python 3.12 syntax and awaits all SillyGirl SDK calls.
