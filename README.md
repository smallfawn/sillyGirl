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

## 插件编写

插件是普通 JavaScript 文件，通过头部注释声明名称、规则、版本等元数据。脚本插件可以在 Admin 面板「脚本插件」里编辑，也可以放到 `plugins/插件名/main.js` 使用 NodeJS 运行。

```js
/**
 * @title HelloWorld
 * @rule raw ^你好$
 * @version v1.0.0
 * @author custom
 */

s.reply("Hello World!");
```

常用元数据：

| 字段 | 说明 |
|------|------|
| `title` | 插件标题，显示在管理面板和插件市场 |
| `rule` | 消息匹配规则，可写多条 |
| `priority` | 匹配优先级，数字越大越优先 |
| `version` | 插件版本，例如 `v1.0.0` |
| `author` | 作者 |
| `description` | 插件说明 |
| `public` | 是否公开到插件市场 |
| `disable` | 是否禁用 |
| `admin` | 是否仅管理员可触发 |
| `platform` | 限制平台，例如 `qq`、`telegram`、`web` |

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

插件配置表单：

```js
const schema = SillyGirlCreateSchema.object({
  host: SillyGirlCreateSchema.string()
    .setTitle("服务地址")
    .setDefault("http://127.0.0.1:9090"),
  enabled: SillyGirlCreateSchema.boolean()
    .setTitle("启用")
    .setDefault(false),
});

const ConfigDB = new SillyGirlPluginConfig(schema);
ConfigDB.get();
s.reply("当前地址：" + ConfigDB.userConfig.host);
```

持久化存储：

```js
const db = Bucket("my-plugin");
db.set("count", 1);
db.get("count", 0);
db.delete("count");
db.keys();
```

## 内联函数说明

### qinglong

先在 Admin 面板「青龙容器」中添加青龙地址、`client_id`、`client_secret`。脚本里按页面编号创建实例：

```js
const ql = new qinglong({ id: 1 });
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
const ql = new qinglong({ id: 1 });
const envs = ql.getEnvs({ searchValue: "JD_COOKIE" });
s.reply("匹配到 " + envs.length + " 个变量");
```

注意：`new qinglong({ id: 1 })` 只接受对象参数，不支持 `new qinglong(1)`。

### smallcat

先在 Admin 面板「smallcat」中添加地址和 `api_auth`。脚本里按页面编号创建实例：

```js
const sc = new smallcat({ id: 1 });
```

常用方法：

| 方法 | 说明 |
|------|------|
| `createQr(type)` | 创建二维码 |
| `createQr(options)` | 创建二维码，支持对象参数 |
| `checkQr(uuid)` | 检查二维码状态 |
| `addUser(options)` | 添加用户，参数 `{ code, type, displayName? }` |
| `userList()` | 获取用户列表 |
| `request(method, path, body, query)` | 调用其他 smallcat API |

示例：

```js
const sc = new smallcat({ id: 1 });
const qr = sc.createQr(1);
if (!qr.status) {
  s.reply("生成二维码失败：" + qr.message);
  return;
}

const checked = sc.checkQr(qr.data.uuid);
s.reply("扫码状态：" + checked.data.state);
```

smallcat 返回值保持原始 API 响应，不额外改写。

### daidai

先在 Admin 面板「呆呆面板」中添加地址、`app_key`、`app_secret`。脚本里按页面编号创建实例：

```js
const dd = new daidai({ id: 1 });
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
const dd = new daidai({ id: 1 });
const envs = dd.getEnvs({ keyword: "JD_COOKIE" });
s.reply("呆呆面板变量数量：" + envs.length);
```

注意：`new daidai({ id: 1 })` 只接受对象参数，不支持 `new daidai(1)`。

### Cron

```js
const task = Cron();
const ret = task.add("*/5 * * * * *", () => {
  console.log("每 5 秒执行一次");
});

task.remove(ret.id);
```

定时执行推荐在 Admin 面板「定时任务」里配置。

### Express

```js
const app = Express();

app.get("/hello", (req, res) => {
  res.send("Hello World!");
});
```

Web 插件需要声明 `@web true`。

## 功能说明

| 功能 | 说明 |
|------|------|
| 管理面板 | Vue 管理后台，支持脚本、插件市场、配置、存储、任务等管理 |
| 脚本插件 | 支持 JS 代码高亮、格式化、文件管理和在线编辑 |
| 插件市场 | 支持管理插件源，从 GitHub 仓库 `plugins/` 目录导入插件 |
| 插件配置 | 支持 `SillyGirlCreateSchema` / `SillyGirlPluginConfig` 声明式配置表单 |
| 依赖管理 | 使用 pnpm 管理 NodeJS 插件依赖，支持安装和卸载 |
| NodeJS 运行 | `plugins/插件名/main.js` 走 NodeJS 运行时 |
| 存储 | 支持 BoltDB 和 Redis，Admin 面板可切换存储桶查询 |
| 青龙容器 | 可添加多个青龙面板，并在脚本中通过 `new qinglong({ id })` 调用 |
| smallcat | 可添加多个 smallcat 面板，并在脚本中通过 `new smallcat({ id })` 调用 |
| 呆呆面板 | 可添加多个呆呆面板，并在脚本中通过 `new daidai({ id })` 调用 |
| 适配器 | 内置 QQ、Telegram Bot、Web 适配器，并提供 Pagermaid 桥接脚本 |
| 定时任务 | 支持 Cron 表达式和脚本触发 |
| Docker 发布 | GitHub Actions 打包 Releases，并推送 Docker Hub 镜像 |

后台首次访问规则：

- 未设置 `sillyGirl.password` 时，首次打开后台会强制创建管理员账号和密码。
- 初始化成功后才会进入管理页面，并写入登录 Cookie。
- 初始化后可在 Admin 面板「基础设置」中修改后台账号名和密码。

## 接入适配器

适配器配置都可以在 Admin 面板「存储」里添加或修改。选择对应存储桶后新增键值，保存后相关适配器会自动重载；也可以重启程序确认连接状态。

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
| `qq` | `access_token` | OneBot 反向 WebSocket 的访问密钥，需和 NapCat 的 `accessToken` 一致 |
| `qq` | `token` | 兼容旧写法；未设置 `access_token` 时读取 |
| `qq` | `debug` | 可选，设为 `true` 时输出 QQ 收发消息调试日志 |

注意：

- Docker 部署时，如果 NapCat 在宿主机或其他机器上，`url` 不能写容器内部的 `localhost`，要写宿主机 IP、局域网 IP 或域名。
- 不设置 `qq.access_token` 也能连接，但不安全，公网部署必须设置。
- 连接成功后，Admin 面板适配器状态里会看到 `QQ` 在线和当前 bot id。

### Telegram Bot

| 存储桶 | 键 | 说明 |
|------|----|------|
| `telegram` | `token` | BotFather 提供的 Bot Token |
| `telegram` | `bot_token` | 兼容写法；未设置 `token` 时读取 |
| `telegram` | `enable` | 可选，设为 `false` 时禁用 |
| `telegram` | `api_base` | 可选，默认 `https://api.telegram.org` |
| `telegram` | `drop_pending_updates` | 可选，默认 `true` |
| `telegram` | `debug` | 可选，设为 `true` 时输出 Telegram 调试日志 |

接入步骤：

1. 在 Telegram 找 `@BotFather` 创建 Bot，拿到 Bot Token。
2. 在 Admin 面板「存储」选择 `telegram` 存储桶，新增 `token`。
3. 如果服务器访问 Telegram 官方 API 不通，可以把 `api_base` 设置为反代地址，例如 `https://api.telegram.org` 的兼容代理。
4. 保存后适配器会自动重启；日志出现 `telegram机器人(...)轮询已启动` 即表示接入成功。

Telegram 当前使用 Bot API 长轮询模式，启动时会调用 `deleteWebhook`。如果这个 Bot 之前设置过 webhook，程序会自动清理后再开始轮询。

### Pagermaid

Pagermaid 通过仓库内的桥接插件接入：

```text
adapters/pagermaid/sillyplus.py
```

接入步骤：

1. 将 [sillyplus.py](adapters/pagermaid/sillyplus.py) 放到 Pagermaid 的插件目录。
2. 把文件里的 `uri = "${rws()}"` 改成 SillyGirl 提供的 WebSocket 地址。
3. 重启 Pagermaid，或在 Pagermaid 中重新加载插件。
4. 在 Telegram 里发送 Pagermaid 命令 `sillyGirl`，返回 `傻+ 已连接` 表示桥接在线。

WebSocket 地址格式：

```text
ws://<SillyGirl地址>:8080/<你的WebSocket路径>
```

如果使用 HTTPS 反向代理：

```text
wss://<域名>/<你的WebSocket路径>
```

当前仓库提供的是 Pagermaid 端桥接脚本；SillyGirl 侧需要有对应的 WebSocket 插件或接口来处理这个路径。没有配置服务端 WebSocket 路由时，Pagermaid 端会一直离线或重连。

更多细节见 `docs/` 目录。

## 致谢

本项目基于并延续了前作者 cdle 的 SillyGirl 项目思想与历史代码积累，感谢原项目作者及社区贡献者的长期工作。

- 原项目：https://github.com/cdle/sillyGirl

## 许可

[MIT](LICENSE)
