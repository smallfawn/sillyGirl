# Plugin Recipes

Copy the nearest recipe, replace its metadata and business logic, then run the bundled validator.

## Message Command with Capture

```js
// [title: Echo]
// [name: echo]
// [description: Echo a captured value]
// [author: AI]
// [version: v1.0.0]
// [rule: raw ^echo [内容]$]

const { sender: s } = require("sillygirl");

async function main() {
  const value = await s.param("内容");
  await s.reply(String(value || ""));
}

main().catch(async (error) => {
  console.error(error);
  await s.reply(`执行失败：${error?.message || error}`);
});
```

## Configured HTTP Request

```js
// [title: API Query]
// [name: apiQuery]
// [description: Query a configured API]
// [author: AI]
// [version: v1.0.0]
// [rule: raw ^查询$]
// [depe: ["axios"]]

const axios = require("axios");
const { sender: s, plugin } = require("sillygirl");

const Config = new plugin.Form({
  endpoint: plugin.Form.string().title("接口地址").default(""),
  token: plugin.Form.string().title("Token").format("password").default(""),
});

async function main() {
  const { endpoint, token } = await Config.get();
  if (!endpoint) throw new Error("请先配置接口地址");
  const response = await axios.get(endpoint, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    timeout: 10000,
    validateStatus: (status) => status >= 200 && status < 300,
  });
  await s.reply(JSON.stringify(response.data));
}

main().catch(async (error) => {
  console.error(error?.message || error);
  await s.reply(`请求失败：${error?.message || error}`);
});
```

## Cron Job

```js
// [title: Daily Job]
// [name: dailyJob]
// [description: Run every day at 08:00]
// [author: AI]
// [version: v1.0.0]
// [cron: 0 0 8 * * *]

const { Bucket } = require("sillygirl");
const store = new Bucket("daily-job");

async function main() {
  await store.set("lastRun", new Date().toISOString());
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
```

## Carry Entry

```js
// [title: Carry Processor]
// [name: carryProcessor]
// [description: Process carry content]
// [author: AI]
// [version: v1.0.0]
// [rule: raw ^处理 [内容]$]
// [carry: true]

const { sender: s } = require("sillygirl");

async function main() {
  const content = await s.param("内容");
  await s.reply(`已处理：${content}`);
}

main().catch(async (error) => s.reply(`执行失败：${error?.message || error}`));
```

## Resident Web Process

```js
// [title: Health Server]
// [name: healthServer]
// [description: Expose a local health endpoint]
// [author: AI]
// [version: v1.0.0]
// [web: true]

const http = require("http");
const port = Number(process.env.PORT || 3000);

http.createServer((request, response) => {
  if (request.method === "GET" && request.url === "/health") {
    response.writeHead(200, { "content-type": "application/json; charset=utf-8" });
    response.end(JSON.stringify({ ok: true }));
    return;
  }
  response.writeHead(404);
  response.end();
}).listen(port, "0.0.0.0", () => console.log(`listening on ${port}`));
```

## Python Message Command

```python
# [title: Python Echo]
# [name: pythonEcho]
# [description: Echo a captured value]
# [author: AI]
# [version: v1.0.0]
# [rule: raw ^py [内容]$]

import asyncio
from sillygirl import sender as s


async def main():
    value = await s.param("内容")
    await s.reply(str(value or ""))


if __name__ == "__main__":
    asyncio.run(main())
```
