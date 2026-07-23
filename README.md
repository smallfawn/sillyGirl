# SillyGirl

[![Go Version](https://img.shields.io/badge/Go-1.18+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

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

访问 `http://localhost:8080/admin` 打开管理面板。`./data` 会映射到容器内 `/data`，用于持久化 BoltDB、插件、配置和 NodeJS 运行文件。

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
| 适配器 | 内置 QQ、Telegram Bot、Web、Pagermaid 等适配器 |
| 定时任务 | 支持 Cron 表达式和脚本触发 |
| Docker 发布 | GitHub Actions 打包 Releases，并推送 Docker Hub / GHCR 镜像 |

后台首次访问规则：

- 未设置 `sillyGirl.password` 时，首次打开后台会强制创建管理员账号和密码。
- 初始化成功后才会进入管理页面，并写入登录 Cookie。
- 初始化后可在 Admin 面板「基础设置」中修改后台账号名和密码。

Telegram Bot 配置：

| 存储桶 | 键 | 说明 |
|------|----|------|
| `telegram` | `token` | BotFather 提供的 Bot Token |
| `telegram` | `enable` | 可选，设为 `false` 时禁用 |
| `telegram` | `api_base` | 可选，默认 `https://api.telegram.org` |
| `telegram` | `drop_pending_updates` | 可选，默认 `true` |

更多细节见 `docs/` 目录。

## 致谢

本项目基于并延续了前作者 cdle 的 SillyGirl 项目思想与历史代码积累，感谢原项目作者及社区贡献者的长期工作。

- 原项目：https://github.com/cdle/sillyGirl

## 许可

[MIT](LICENSE)
