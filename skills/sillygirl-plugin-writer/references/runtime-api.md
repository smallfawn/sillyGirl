# SillyGirl Plugin Runtime API

Use this reference when generating a plugin. All runtime calls below are asynchronous unless explicitly shown otherwise, so JavaScript uses `await` and Python SDK code also uses `await`.

## Metadata

One metadata item per comment line:

```text
// [title: Display title]
// [name: filesystemSafeName]
// [desc: What the plugin does]
// [author: Author]
// [version: v1.0.0]
// [rule: raw ^command$]
// [cron: 0 0 8 * * *]
// [on_start: true]
// [web: true]
// [admin: true]
// [status: true]
// [public: false]
// [class: Utilities]
// [depe: ["axios"]]
```

Python changes the prefix to `#`. Repeat `rule` for multiple triggers. `title`, `name`, `desc`, and `version` are required. An installable file also needs at least one of `rule`, `cron`, `on_start`, `web`, or `module=true`.

`status` is the plugin-wide runtime switch. Missing `status` defaults to `true`.

## JavaScript Imports

```js
const {
  sender: s,
  Bucket,
  plugin,
  user,
  container,
  utils,
} = require("sillygirl");
```

Import only the symbols in use.

## Sender

Common methods:

```js
await s.getMsg();
await s.getMsgId();
await s.getUserId();
await s.getChatId();
await s.getPlatform();
await s.getUsername();
await s.param("name");
await s.isAdmin();
await s.reply("text");
await s.reply({ type: "image", url: "https://HOST/image.png" });
await s.listen({ timeout: 30000 });
await s.recallMessage(messageId);
await s.resume();
```

Use `getMsg()` to read the current message and `setMsg(content)` to replace it.

Do not assume a return shape that the target code has not established. Inspect `core/grpc_plugins.go` and existing plugins when using less common methods.

## Bucket

```js
const store = new Bucket("plugin-name");
await store.get("key", defaultValue);
await store.set("key", value);
await store.delete("key");
await store.keys();
```

Use a plugin-specific bucket name. Store structured values as JSON when the runtime call expects strings.

## Forms

Register forms at top level:

```js
const Config = new plugin.Form({
  endpoint: plugin.Form.string().title("接口地址").default(""),
  token: plugin.Form.string().title("Token").format("password").default(""),
  retries: plugin.Form.number().title("重试次数").default(2),
  featureEnabled: plugin.Form.boolean().title("启用功能").default(true),
});

const config = await Config.get();
```

For per-user data, use `new user.Form({...})`. Validation builders include `.required().err("message")`, `.match(pattern).err("message")`, and `.test(fn)`. A `.test` callback must be self-contained because the runtime may execute it in isolation.

## Containers

Construct integrations with object parameters:

```js
const qinglong = new container.QingLong({ id: 1 });
const smallCat = new container.SmallCat({ id: 1 });
const daiDai = new container.DaiDai({ id: 1 });
```

Await client methods and verify method names against `core/grpc_plugins.go` or a repository example before generating an uncommon call. Never log tokens, cookies, or complete credentials.

## Utilities and Dependencies

- Use `utils` only for methods verified in the current checkout.
- Declare npm or PyPI packages in JSON-array metadata such as `[depe: ["axios"]]`.
- Declare marketplace module plugins in the same array as `./name.js` for NodeJS or `./name.py` for Python. Module resolution uses only `depe`, never `require` or `import` scanning.
- Marketplace scripts are installed under `/data/plugins/<publisher>/`; locally created scripts use `/data/plugins/local/`. A `./name.js` or `./name.py` module dependency resolves only inside that same publisher directory.
- Marketplace installation recursively installs modules first and installs each script's package dependencies before loading that script. Shared package runtimes remain at `/data/plugins/node_modules` and `/data/plugins/python_packages`.
- The same publisher cannot contain both `name.js` and `name.py`; the runtime-independent identity is `<publisher>/name`.
- Do not declare Node built-ins or Python standard-library modules.
- Add explicit network timeouts and handle non-success responses.

## Python Runtime

Target Python 3.12:

```python
import asyncio
from sillygirl import Bucket, plugin
from sillygirl import sender as s

async def main():
    content = await s.getMsg()
    await s.reply(content)

if __name__ == "__main__":
    asyncio.run(main())
```

Calls on `sender`, `Bucket`, forms, users, and containers are awaitable. Keep form registration at module scope and execute work from `main()`.
