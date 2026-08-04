# SillyGirl

[![Go Version](https://img.shields.io/badge/Go-1.18+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

DOCKER镜像加速地址查看
https://status.1panel.top/
## Docker 快速部署

```bash
mkdir -p data
docker run -d \
  --name sillygirl \
  --restart unless-stopped \
  -p 8080:8080 \
  -e SILLYGIRL_DATA_PATH=/data \
  -v $(pwd)/data:/data \
  smallfawn/sillygirl:latest
```

访问 `http://localhost:8080/admin` 打开管理面板。`./data` 会映射到容器内 `/data`，用于持久化 BoltDB、插件和配置。
机器人里发送 `更新` 会通过 GitHub 加速地址下载最新 Release 包并替换程序文件，不需要挂载 Docker socket。

## 升级前数据迁移

主程序启动前会自动执行一次旧字段迁移，把旧的容器面板、BOT 配置、插件配置、用户绑定和插件授权字段写入新版存储字段。

也可以在启动前单独运行迁移小程序检查或手动迁移：

```bash
# 只检查不写入
go run ./cmd/storage-migrate -dry-run

# 手动迁移
go run ./cmd/storage-migrate
```

Release 包内会附带同平台的 `storage-migrate_*` 小程序；二进制部署时也可以直接运行该文件。

## Docker Compose

创建 `docker-compose.yml`：

```yaml
services:
  sillygirl:
    image: smallfawn/sillygirl:latest
    container_name: sillygirl
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      SILLYGIRL_DATA_PATH: /data
    volumes:
      - ./data:/data
```

启动：

```bash
mkdir -p data
docker compose up -d
docker compose logs -f
```

## 界面预览

### 后台管理

![SillyGirl 后台管理](docs/images/admin-page.png)

## 插件编写

插件是普通 JavaScript 或 Python 文件，通过头部注释声明名称、规则、版本等元数据。脚本插件可以在 Admin 面板「脚本插件」里编辑，也可以放到 `plugins/插件名.js` 或 `plugins/插件名.py` 运行。容器内对应路径是 `/data/plugins/插件名.js` 或 `/data/plugins/插件名.py`。

```js
/**
 * @title HelloWorld
 * @rule raw ^你好$
 * @version v1.0.1
 * @author custom
 */

s.reply("Hello World!");
```

Python 插件使用 Python 3.12 运行，SDK 通过 `from sillygirl import ...` 引入。异步 API 需要 `await`：

```python
"""
* @title PythonHello
* @rule raw ^py你好$
* @version v1.0.1
* @author custom
"""

import asyncio
from sillygirl import sender as s


async def main():
    await s.reply("Hello from Python!")


asyncio.run(main())
```

Python 插件要点：

| 项目 | 说明 |
|------|------|
| 运行版本 | 只使用 Python 3.12；Docker 镜像已内置，本地运行需要安装 Python 3.12 |
| 运行路径 | 推荐扁平文件 `/data/plugins/插件名.py`，不要再套 `插件名/main.py` |
| SDK 引入 | 使用 `from sillygirl import sender as s, Bucket, container, form, utils` |
| 异步调用 | Python SDK 方法都是异步方法，`s.reply()`、`Bucket.get()` 等都要 `await` |
| 运行时依赖 | 内置 `grpcio==1.83.0`、`protobuf==7.35.1`，用于和 Go 主程序 gRPC 通信 |
| 第三方依赖 | 在注释中写 `@depe ["requests"]`，后台「依赖管理」选择 Python 后可安装/卸载 |
| 依赖位置 | Python 依赖由 pipx 管理，统一放在 `/data/plugins/python_packages` 下面供插件共享 |

Python 使用存储和配置：

```python
"""
* @title Python配置示例
* @rule raw ^py配置$
* @depe ["requests"]
"""

import asyncio
from sillygirl import sender as s, Bucket, form

config = form({
    "token": form.string()
        .title("Token")
        .format("password")
        .default(""),
})
db = Bucket("python-demo")


async def main():
    values = await config.get()
    await db.set("last_user", await s.getUserId())
    await s.reply("Token 是否已配置：" + ("是" if values.get("token") else "否"))


asyncio.run(main())
```

元数据必填规则：

| 使用场景 | 必填参数 | 说明 |
|------|------|------|
| 普通消息插件 | `@title`、`@rule` | `@rule` 用来匹配消息，不写规则就不会被普通消息触发 |
| 搬运处理脚本 | `@title`、`@carry` | 搬运页的“处理脚本”只展示带 `@carry` 或 `@carry true` 的插件 |
| 启动脚本 | `@title`、`@on_start true` | 程序启动时执行一次 |
| Web 服务脚本 | `@title`、`@web true` | 程序启动时常驻运行，脚本自己监听端口 |
| 脚本定时任务 | `@title`、`@cron 表达式` | 写了 `@cron` 的脚本会直接显示在 Admin 面板「定时任务」 |
| 纯模块/工具脚本 | `@title`、`@module true` | 只作为模块或工具文件，不参与普通消息匹配 |

元数据参数说明：

| 参数 | 是否必填 | 说明 |
|------|------|------|
| `@title 名称` | 建议必填 | 插件标题，显示在管理面板和插件市场 |
| `@rule 规则` | 普通消息插件必填 | 消息匹配规则，可写多条；支持 `raw ^正则$` 和占位参数 `[名称]` |
| `@priority 数字` | 非必填 | 匹配优先级，数字越大越优先，默认 `0` |
| `@admin true/false` | 非必填 | 是否仅管理员可触发，默认 `false` |
| `@version 版本号` | 非必填 | 插件版本，默认 `v1.0.1` |
| `@author 作者` | 非必填 | 作者名 |
| `@desc 描述` | 非必填 | 插件说明，显示在后台或插件市场 |
| `@depe ["依赖名"]` | 非必填 | 插件依赖声明；NodeJS 依赖由 pnpm 安装，Python 依赖由 pipx 安装 |
| `@icon URL` | 非必填 | 插件图标 URL，未填写时使用默认苹果图标 |
| `@public true/false` | 非必填 | 是否允许公开到插件市场，默认 `false` |
| `@origin 来源` | 非必填 | 插件来源标记，默认 `自定义` |
| `@class 标签` | 非必填 | 插件分类标签，可写多个 |
| `@module true/false` | 非必填 | 是否作为模块插件；为 `true` 时不参与普通消息匹配 |
| `@carry` 或 `@carry true/false` | 搬运脚本必填 | 是否可作为搬运处理脚本；写 `@carry` 等同于 `@carry true`，默认 `false` |
| `@cron 表达式` | 脚本定时任务必填 | 声明脚本定时任务，例如 `@cron 0 * * * *`；只支持直接写 Cron 表达式 |
| `@on_start true/false` | 启动脚本必填 | 是否在程序启动时执行一次 |
| `@web true/false` | Web 服务脚本必填 | 是否作为 Web 常驻脚本启动；端口和路由由脚本自己处理 |

如果脚本已经写了 `@cron`，它会自动展示到「定时任务」列表；如果在「定时任务」里选择 `node 插件名.js` 或 `python 插件名.py` 创建任务，系统会把 Cron 表达式写回该脚本头部注释，而不是额外创建一份重复任务。

规则支持占位捕获：

```js
/**
 * @title 天气示例
 * @rule 天气 [城市]
 */

const city = s.param("城市");
s.reply(city + " 天气晴");
```

常用 `sender` 方法：

```js
s.getUserId();       // 用户 ID
s.getUserName();     // 用户昵称
s.getChatId();       // 群聊 ID
s.getPlatform();     // 平台
s.getContent();      // 消息内容
s.param("城市");     // 获取规则捕获参数
s.reply("文本");     // 回复消息
s.continue();        // 继续匹配后续插件
```

推送管理员：

```js
const { sender: s } = require("sillygirl");

await s.pushAdmin("任务执行完成");
await s.pushAdmin("只推送 QQ 管理员", { platform: "qq" });
await s.pushAdmin("指定机器人推送", { platform: "qq", botId: "10001" });
```

`s.pushAdmin(content, options?)` 会读取对应平台存储桶里的 `masters` 管理员列表；不传 `platform` 时会遍历所有带 `masters` 的平台。

插件配置表单：

```js
const { form } = require("sillygirl");

const ConfigDB = new form({
  host: form.string()
    .title("服务地址")
    .default("http://127.0.0.1:9090"),
  enabled: form.boolean()
    .title("启用")
    .default(false),
});

ConfigDB.get();
s.reply("当前地址：" + ConfigDB.userConfig.host);
```

Web 服务脚本：

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

Python Web 服务脚本：

```python
"""
* @title PythonWeb示例
* @web true
* @class 工具
"""

from http.server import BaseHTTPRequestHandler, HTTPServer
import json


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/health":
            self.send_response(404)
            self.end_headers()
            return

        body = json.dumps(
            {"status": True, "message": "ok", "data": None},
            ensure_ascii=False,
        ).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


HTTPServer(("0.0.0.0", 3002), Handler).serve_forever()
```

持久化存储：

```js
const db = new Bucket("my-plugin");
db.set("count", 1);
db.get("count", 0);
db.delete("count");
db.keys();
```

## 内联函数说明

### QingLong

先在 Admin 面板「青龙容器」中添加青龙地址、`client_id`、`client_secret`。脚本里按页面编号创建实例：

```js
const ql = new container.QingLong({ id: 1 });
```

常用方法：

| 方法 | 说明 |
|------|------|
| `getEnvs(options)` | 获取环境变量，支持 `{ searchValue }` |
| `getEnvById(id)` | 获取单个环境变量 |
| `createEnv(env)` | 新增环境变量，支持对象或数组 |
| `updateEnv(env)` | 更新环境变量 |
| `deleteEnvs(ids)` | 删除环境变量 |
| `disableEnvs(ids)` | 禁用环境变量 |
| `enableEnvs(ids)` | 启用环境变量 |
| `systemNotify(title, content)` | 调用青龙系统通知 |
| `request(method, path, body, query)` | 调用其他青龙 Open API |

示例：

```js
const ql = new container.QingLong({ id: 1 });
const envs = ql.getEnvs({ searchValue: "JD_COOKIE" });
s.reply("匹配到 " + envs.length + " 个变量");
```

注意：`new container.QingLong({ id: 1 })` 只接受对象参数，不支持 `new container.QingLong(1)`。

### 普通用户列表

`utils.userList()` 返回普通用户、绑定信息以及当前插件的 SmallCat 读取授权状态；授权状态由运行时自动绑定到当前插件，脚本不需要也不能传入其他插件 UUID。

```js
const { container, utils } = require("sillygirl");

const users = await utils.userList();
const authorizedOpenids = users
  .filter((user) => !user.disabled && user.authorized)
  .flatMap((user) => user.bindings.smallcat_openids);

// SmallCat.userList() 保持为 SmallCat 账号列表接口。
const accounts = await new container.SmallCat({ id: 1 }).userList();
```

每项包含 `id`、`username`、`nickname`、`disabled`、`authorized`，以及 `bindings.qq`、`bindings.telegram`、`bindings.smallcat_openids`。未授权、用户已禁用、插件已关闭或插件已禁用时，`authorized` 为 `false`，`smallcat_openids` 固定为空数组。

### SmallCat

先在 Admin 面板「smallcat」中添加地址和 `api_auth`。脚本里按页面编号创建实例：

```js
const sc = new container.SmallCat({ id: 1 });
```

常用方法：

| 方法 | 说明 |
|------|------|
| `createQr(type)` | 创建二维码 |
| `createQr(options)` | 创建二维码，支持对象参数 |
| `checkQr(uuid)` | 检查二维码状态 |
| `addUser(options)` / `rescanUser(options)` | 新增 / 重扫更新账号 |
| `userList()` | 获取用户列表 |
| `checkUsers(options)` | 检查一个或多个账号状态 |
| `setUserRemark(options)` / `setUserDisabled(options)` / `deleteUser(options)` | 修改备注 / 启停 / 删除账号 |
| `proxyList()` / `testProxy(options)` / `addProxy(options)` / `deleteProxy(options)` | 代理节点管理 |
| `creditBalance()` / `creditLedger(query?)` | 积分余额 / 流水 |
| `getCode(options)` | 获取小程序 code，参数 `{ openid, appid }`，返回 smallcat API 原始 JSON |
| `getSession(options)` / `refreshSession(options)` | 获取 / 刷新小程序运行时 session |
| `getUserInfo(options)` | 获取小程序用户信息，参数 `{ openid, appid }`，返回 smallcat API 原始 JSON |
| `getEncryptKey(options)` | 获取用户加密 key |
| `getPhoneNumber(options)` | 获取手机号 code，调用 `POST /wx/getphonenumber`，返回 smallcat API 原始 JSON |
| `cloud(options)` / `gateway(options)` | 云函数 / V3 云网关凭证 |
| `qrCodeAuth(options)` | 二维码 OAuth 授权，调用 `POST /wx/qrcodeauth`，返回 smallcat API 原始 JSON |
| `oAuth(options)` | OAuth 授权，调用 `POST /wx/oauth`，返回 smallcat API 原始 JSON |
| `translateLink(options)` / `autoAuth(options)` | 解析小程序口令 / 刷新 APP SESSION |
| `appMsgExt(options)` / `appMsgLike(options)` | 阅读扩展 / 公众号文章点赞 |
| `request(method, path, body, query)` | 调用其他 smallcat API |

示例：

```js
const sc = new container.SmallCat({ id: 1 });
const qr = sc.createQr(1);
if (!qr.status) {
  s.reply("生成二维码失败：" + qr.message);
  return;
}

const checked = sc.checkQr(qr.data.uuid);
s.reply("扫码状态：" + checked.data.state);

const code = sc.getCode({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
s.reply(JSON.stringify(code));

const session = sc.getSession({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
s.reply(JSON.stringify(session));

const userInfo = sc.getUserInfo({
  openid: "用户 openid",
  appid: "wx1234567890abcdef",
});
s.reply(JSON.stringify(userInfo));

const oauth = sc.oAuth({
  openid: "用户 openid",
  appid: "wx2f5d8f9715c59d10",
  redirect_uri: "https://example.com/callback",
  scope: "snsapi_userinfo",
  state: "STATE",
});
s.reply(JSON.stringify(oauth));

const qrOAuth = sc.qrCodeAuth({
  openid: "用户 openid",
  uuid: "二维码 UUID",
});
s.reply(JSON.stringify(qrOAuth));
```

smallcat 返回值保持原始 API 响应，不额外改写。

### DaiDai

先在 Admin 面板「呆呆面板」中添加地址、`app_key`、`app_secret`。脚本里按页面编号创建实例：

```js
const dd = new container.DaiDai({ id: 1 });
```

常用方法：

| 方法 | 说明 |
|------|------|
| `getEnvs(options)` | 获取环境变量，支持 `{ keyword }` |
| `getEnvById(id)` | 获取单个环境变量 |
| `createEnv(env)` | 新增环境变量 |
| `updateEnv(env)` | 更新环境变量，建议包含 `id` |
| `deleteEnv(id)` / `deleteEnvs(ids)` | 删除单个或批量删除环境变量 |
| `enableEnv(id)` / `disableEnv(id)` | 启用或禁用单个环境变量 |
| `getTasks(options)` | 获取任务列表，支持 `{ keyword }` |
| `runTask(id)` / `stopTask(id)` | 运行或停止任务 |
| `request(method, path, body, query)` | 调用其他呆呆面板 API |

示例：

```js
const dd = new container.DaiDai({ id: 1 });
const envs = dd.getEnvs({ keyword: "JD_COOKIE" });
s.reply("呆呆面板变量数量：" + envs.length);
```

注意：`new container.DaiDai({ id: 1 })` 只接受对象参数，不支持 `new container.DaiDai(1)`。

### Cron

```js
const task = Cron();
const ret = task.add("*/5 * * * * *", () => {
  console.log("每 5 秒执行一次");
});

task.remove(ret.id);
```

定时执行推荐在 Admin 面板「定时任务」里配置。

## 功能说明

| 功能 | 说明 |
|------|------|
| 管理面板 | Vue 管理后台，支持脚本、插件市场、配置、存储、任务等管理 |
| 脚本插件 | 支持 JS/Python 代码高亮、文件管理和在线编辑；JS 支持格式化 |
| 插件市场 | 支持管理插件源，从 GitHub 仓库 `plugins/` 目录导入插件 |
| 插件配置 | 支持 `new form({ key: form.string().title("标题").default("") })` 链式配置表单 |
| 依赖管理 | 支持按 NodeJS/Python 筛选；NodeJS 使用 pnpm 管理共享依赖，Python 使用 pipx 管理共享依赖；依赖从插件注释 `@depe ["包名"]` 读取 |
| NodeJS 运行 | `/data/plugins/插件名.js` 走 NodeJS 运行时，兼容旧版 `plugins/插件名/main.js` |
| Python 运行 | `/data/plugins/插件名.py` 走 Python 3.12 运行时，Docker 镜像已内置 Python、pipx、`grpcio==1.83.0` 和 `protobuf==7.35.1` |
| 存储 | 支持 BoltDB 和 Redis，Admin 面板可切换存储桶查询 |
| 搬运 | 可按平台和群号把消息交给指定插件脚本处理，业务过滤和转发由脚本自行实现 |
| 青龙容器 | 可添加多个青龙面板，并在脚本中通过 `new container.QingLong({ id })` 调用 |
| smallcat | 可添加多个 smallcat 面板，并在脚本中通过 `new container.SmallCat({ id })` 调用 |
| 呆呆面板 | 可添加多个呆呆面板，并在脚本中通过 `new container.DaiDai({ id })` 调用 |
| 适配器 | 内置微信 ClawBot、QQ/OneBot、Telegram Bot、钉钉 Stream、QQ 官方频道 Webhook、Web、Pagermaid 适配器 |
| 定时任务 | 支持 Cron 表达式、`node 插件名.js` 和 `python 插件名.py` 脚本触发 |
| Docker 发布 | GitHub Actions 打包 Releases，并推送 Docker Hub 镜像 |

后台首次访问规则：

- 未设置 `sillyGirl.password` 时，首次打开后台会强制创建管理员账号和密码。
- 初始化成功后才会进入管理页面，并写入登录 Cookie。
- 初始化后可在 Admin 面板「基础设置」中修改后台账号名和密码。

## 接入适配器

适配器配置都可以在 Admin 面板「存储」里添加或修改。选择对应存储桶后新增键值，保存后相关适配器会自动重载；也可以重启程序确认连接状态。

### 微信 ClawBot

ClawBot 接入使用腾讯 OpenClaw 微信通道同款 iLink HTTP API：`ilink/bot/getupdates` 长轮询收消息，`ilink/bot/sendmessage` 发送回复。当前实现聚焦文本私聊对话：用户给 ClawBot 发消息后，SillyGirl 匹配脚本规则并用同一条消息的 `context_token` 回复。

SillyGirl 侧配置：

| 存储桶 | 键 | 说明 |
|------|----|------|
| `clawbot` | `token` | ClawBot / OpenClaw 微信通道的 iLink bot token，可在 Admin 面板「BOT」里扫码获取 |
| `clawbot` | `enable` | 可选，设为 `false` 时禁用 |
| `clawbot` | `api_base` | 可选，默认 `https://ilinkai.weixin.qq.com` |
| `clawbot` | `debug` | 可选，设为 `true` 时输出 ClawBot 收发消息调试日志 |

注意：

- Admin 面板「BOT」中点击 `扫码获取` 会调用微信 ClawBot 登录接口，前端把返回的二维码链接渲染成二维码图片；扫码确认后会自动写入 `clawbot.token` 并启用 ClawBot。
- ClawBot 的 `sendmessage` 依赖上游消息里的 `context_token`，因此不要把它当成无上下文主动推送通道使用。
- 连接成功后，Admin 面板「BOT」和「概览」里会看到 `微信 ClawBot` 在线。

### QQ

QQ 使用 OneBot 反向 WebSocket 接入，适用于 NapCat、Lagrange.OneBot、go-cqhttp 类兼容端。

SillyGirl 监听地址：

```text
ws://<SillyGirl地址>:8080/qq/receive
```

如果前面套了 HTTPS 反向代理，则使用：

```text
wss://<域名>/qq/receive
```

NapCat 示例配置：

```json
{
  "enable": true,
  "url": "ws://127.0.0.1:8080/qq/receive",
  "accessToken": "你的QQ连接密钥"
}
```

SillyGirl 侧配置：

| 存储桶 | 键 | 说明 |
|------|----|------|
| `qq` | `token` | OneBot 反向 WebSocket 的访问密钥，需和 NapCat 的 `accessToken` 一致 |
| `qq` | `debug` | 可选，设为 `true` 时输出 QQ 收发消息调试日志 |

注意：

- Docker 部署时，如果 NapCat 在宿主机或其他机器上，`url` 不能写容器内部的 `localhost`，要写宿主机 IP、局域网 IP 或域名。
- 不设置 `qq.token` 也能连接，但不安全，公网部署必须设置。
- 连接成功后，Admin 面板适配器状态里会看到 `QQ` 在线和当前 bot id。

### Telegram Bot

| 存储桶 | 键 | 说明 |
|------|----|------|
| `telegram` | `token` | BotFather 提供的 Bot Token |
| `telegram` | `enable` | 可选，设为 `false` 时禁用 |
| `telegram` | `api_base` | 可选，默认 `https://api.telegram.org` |
| `telegram` | `debug` | 可选，设为 `true` 时输出 Telegram 调试日志 |

接入步骤：

1. 在 Telegram 找 `@BotFather` 创建 Bot，拿到 Bot Token。
2. 在 Admin 面板「BOT」填写 Telegram Bot 的 `Token`。
3. 如果服务器访问 Telegram 官方 API 不通，在「BOT」把「代理 API」设置为 `https://api.telegram.org` 的兼容反代地址。
4. 保存后适配器会自动重启；日志出现 `telegram机器人(...)轮询已启动` 即表示接入成功。

Telegram 当前使用 Bot API 长轮询模式，启动时会调用 `deleteWebhook`。如果这个 Bot 之前设置过 webhook，程序会自动清理后再开始轮询。

### 钉钉机器人

钉钉适配器基于官方 [dingtalk-stream-sdk-go](https://github.com/open-dingtalk/dingtalk-stream-sdk-go) 使用 Stream 模式收消息，不需要暴露公网回调地址；文本回复通过消息携带的 `sessionWebhook` 发回原会话。

| 存储桶 | 键 | 说明 |
|------|----|------|
| `dingtalk` | `client_id` | 钉钉开放平台应用的 Client ID（原 AppKey） |
| `dingtalk` | `client_secret` | 钉钉开放平台应用的 Client Secret（原 AppSecret） |
| `dingtalk` | `enable` | 可选，设为 `false` 时禁用 |
| `dingtalk` | `debug` | 可选，设为 `true` 时输出收发消息调试日志 |

接入步骤：

1. 在钉钉开放平台创建机器人应用，启用机器人能力并选择 Stream 模式。
2. 在 Admin 面板「BOT」填写 `Client ID` 与 `Client Secret`。
3. 保存后日志出现 `dingtalk机器人(...) Stream 已连接` 即表示接入成功。

`sessionWebhook` 有时效；普通规则回复会直接复用当前消息上下文，主动推送只会使用该用户或群最近一次仍有效的会话地址。

### QQ 官方频道机器人

该适配器和现有 `QQ / OneBot` 相互独立，平台名为 `qqguild`。它基于腾讯官方 [BotGo](https://github.com/tencent-connect/botgo) 的 HTTP Webhook 事件回调，支持频道消息、频道 `@机器人` 消息和频道私信，并通过官方 OpenAPI 回复。

Webhook 地址：

```text
https://<SillyGirl域名>/qqguild/webhook
```

| 存储桶 | 键 | 说明 |
|------|----|------|
| `qqguild` | `app_id` | QQ 开放平台机器人的 AppID |
| `qqguild` | `app_secret` | QQ 开放平台机器人的 AppSecret |
| `qqguild` | `enable` | 可选，设为 `false` 时禁用 |
| `qqguild` | `sandbox` | 可选，设为 `true` 时使用 BotGo 沙箱 OpenAPI |
| `qqguild` | `debug` | 可选，设为 `true` 时输出收发消息调试日志 |

接入步骤：

1. 在 QQ 开放平台创建频道机器人，配置需要的消息事件权限。
2. 给 SillyGirl 配置可公网访问的 HTTPS 域名，把上面的 Webhook 地址填入机器人事件回调并完成平台校验。
3. 在 Admin 面板「BOT」填写 `AppID` 与 `AppSecret`，保存后日志出现 `qqguild机器人(...) Webhook 已就绪`。

### Pagermaid

Pagermaid 由 SillyGirl 内置 Go WebSocket 适配器接入，Pagermaid 端只需要仓库内的轻量桥接插件：

```text
adapters/pagermaid/sillyplus.py
```

接入步骤：

1. 将 [sillyplus.py](adapters/pagermaid/sillyplus.py) 放到 Pagermaid 的插件目录。
2. 在 Admin 面板「BOT」开启 Pagermaid，可选填写「连接密钥」，然后复制页面显示的 WebSocket 地址。
3. 把文件里的 `uri = "${rws()}"` 改成复制的 WebSocket 地址，或设置环境变量 `SILLYGIRL_PAGERMAID_WS`。
4. 重启 Pagermaid，或在 Pagermaid 中重新加载插件。
5. 在 Telegram 里发送 Pagermaid 命令 `sillyGirl`，返回 `傻+ 已连接` 表示桥接在线。

WebSocket 地址格式：

```text
ws://<SillyGirl地址>:8080/pagermaid/receive?token=<连接密钥>
```

如果使用 HTTPS 反向代理：

```text
wss://<域名>/pagermaid/receive?token=<连接密钥>
```

SillyGirl 侧配置：

| 存储桶 | 键 | 说明 |
|------|----|------|
| `pagermaid` | `enable` | 可选，设为 `false` 时禁用 |
| `pagermaid` | `token` | 可选，Pagermaid WebSocket 连接密钥，公网部署建议填写 |
| `pagermaid` | `debug` | 可选，设为 `true` 时输出 Pagermaid 收发消息调试日志 |

连接成功后，Admin 面板「BOT」和「概览」里会显示 Pagermaid 在线。群监听、禁言、屏蔽用户和插件规则匹配都由 SillyGirl Go 核心处理，Pagermaid 桥接脚本只负责转发消息和执行发消息动作。

更多细节见 `docs/` 目录。

## 致谢

本项目基于并延续了前作者 cdle 的 SillyGirl 项目思想与历史代码积累，感谢原项目作者及社区贡献者的长期工作。

- 原项目：https://github.com/cdle/sillyGirl

## 许可

[MIT](LICENSE)
