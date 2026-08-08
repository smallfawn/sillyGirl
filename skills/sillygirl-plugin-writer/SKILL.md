---
name: sillygirl-plugin-writer
description: Write, migrate, review, debug, and validate complete SillyGirl JavaScript/NodeJS or Python plugins. Use for message-rule plugins, cron jobs, startup jobs, web services, carry handlers, modules, plugin.Form/user.Form schemas, Bucket persistence, sender APIs, dependencies, and QingLong/SmallCat/DaiDai integrations. Trigger on requests to create or fix SillyGirl scripts, plugin metadata, plugin-market files, or files under plugins/ or /data/plugins.
---

# SillyGirl Plugin Writer

Produce a complete runnable plugin, not a fragment or plan. Prefer editing or creating the plugin file directly when a workspace is available.

## Workflow

1. Inspect the target repository and existing plugin before writing. In this repository, treat `docs/plugin-development.md`, `core/grpc_plugins.go`, and current plugin tests as authoritative.
2. Determine language and execution kind. Default to JavaScript/NodeJS unless the user requests Python or a Python library is materially better.
3. Choose exactly one primary activation: message `rule`, `cron`, `on_start`, or `web`. Add `carry` or `module` only when required.
4. Write current square-bracket metadata, a single-file implementation, configuration, error handling, and user-facing replies.
5. Save source-repository files in its existing `plugins/` layout. Runtime marketplace installs use `/data/plugins/<publisher>/<name>.<ext>`; Admin-created local plugins use `/data/plugins/local/<name>.<ext>`. Shared package runtimes remain at `/data/plugins/node_modules` and `/data/plugins/python_packages`.
6. Run `python skills/sillygirl-plugin-writer/scripts/validate_plugin.py <file>`. Fix every error. Run `node --check <file>` for JavaScript or `python -m py_compile <file>` for Python when available.
7. For repository edits, run the smallest relevant tests and inspect the final diff. Return the file path, trigger/configuration, dependencies, and validation results.

## Select the Plugin Kind

| Request | Metadata | Runtime behavior |
|---|---|---|
| Chat command | `[rule: ...]` | Runs for each matching message |
| Scheduled job | `[cron: ...]` | Runs on the cron expression |
| Startup task | `[on_start: true]` | Runs once when SillyGirl starts |
| Long-running HTTP service | `[web: true]` | Starts as a resident process; the script owns its port |
| Carry processor | `[carry: true]` plus a normal activation accepted by the install path | Appears in carry-script selection |
| Shared library | `[module: true]` | Does not handle normal messages |

The Admin local-plugin editor requires `title`, `name`, `desc`, `version`, and at least one of `rule`, `cron`, `on_start`, `web`, or `module=true`.

Normal plugins and `[module: true]` files share their publisher directory. Declare a same-publisher module with `[depe: ["./sharedTools.js"]]` or `[depe: ["./shared_tools.py"]]`. Resolution uses only `depe`, does not inspect `require`/`import`, and rejects `../` or cross-publisher paths.

Marketplace installation recursively installs declared modules first. Before each module or target script is written and loaded, its own npm/PyPI dependencies are installed into the shared runtime. Keep `package.json`, `node_modules`, and `python_packages` at `/data/plugins`, never inside a publisher directory. The same publisher cannot own both `name.js` and `name.py` because they share one plugin identity.

## Write Current Metadata

Use square-bracket comments for new plugins. Keep `name` filesystem-safe and omit the extension.

```js
// [title: 插件标题]
// [name: pluginName]
// [desc: 插件说明]
// [author: 作者]
// [version: v1.0.0]
// [rule: raw ^命令$]
// [status: true]
// [public: false]
// [class: 工具]
// [depe: ["axios"]]
```

Python uses `# [key: value]`. Legacy `@title`, `@rule`, and `@depe` are migration inputs only; preserve them only when maintaining a legacy file. Never use `[param: ...]`, `BncrDB`, `BncrCreateSchema`, or `BncrPluginConfig`.

For complete metadata and method signatures, read [references/runtime-api.md](references/runtime-api.md).

## JavaScript Pattern

Use CommonJS and await Promise-returning runtime APIs.
Use `s.getMsg()` to read the current message and `s.setMsg(content)` to replace it.

```js
const {
  sender: s,
  Bucket,
  container,
  plugin,
  user,
  utils,
} = require("sillygirl");

async function main() {
  const content = await s.getMsg();
  await s.reply(`收到：${content}`);
}

main().catch(async (error) => {
  console.error(error);
  await s.reply(`执行失败：${error?.message || error}`);
});
```

Use [assets/node-plugin.js](assets/node-plugin.js) as the starting file. Do not import BNCR or Goja globals.

## Python Pattern

Target Python 3.12. Await every SillyGirl SDK call.

```python
import asyncio
from sillygirl import sender as s, Bucket, container, plugin, user, utils


async def main():
    content = await s.getMsg()
    await s.reply(f"收到：{content}")


if __name__ == "__main__":
    asyncio.run(main())
```

Use [assets/python-plugin.py](assets/python-plugin.py) as the starting file. Declare non-standard packages in `[depe: [...]]`; do not list Python standard-library modules.

## Rules and Parameters

- Use `raw ^...$` for an exact regular expression.
- Use placeholders such as `[城市]` for named captures.
- Read captures with `await s.param("城市")` or a numeric index.
- Add multiple `[rule: ...]` lines when one plugin handles multiple commands.
- Use `[admin: true]` and also check `await s.isAdmin()` before destructive or privileged work.
- Call `await s.resume()` only when later plugins should continue matching.

## Configuration and Persistence

Register forms at file top level so Admin can discover them without executing business logic.

```js
const Config = new plugin.Form({
  apiBase: plugin.Form.string().title("接口地址").default(""),
  token: plugin.Form.string().title("Token").format("password").default(""),
  featureEnabled: plugin.Form.boolean().title("启用功能").default(true),
});
```

Read values inside `main` with `await Config.get()`. Use `new Bucket("plugin-name")` for persistent state and await `get`, `set`, `delete`, and `keys`. Do not write into the shared `sillyGirl` Bucket unless explicitly requested.

Use `new user.Form({...})` for end-user records. Use `.required().err(...)`, `.match(...).err(...)`, and `.test(...)` for validation; `.test` runs in an isolated process and must be self-contained.

## Containers and External APIs

Construct panel clients with object parameters only:

```js
const ql = new container.QingLong({ id: 1 });
const sc = new container.SmallCat({ id: 1 });
const dd = new container.DaiDai({ id: 1 });
```

Await every client call. Preserve upstream response meaning instead of inventing incompatible wrappers. Add timeouts, validate response status, catch network errors, and mask credentials in logs and replies.

Read [references/runtime-api.md](references/runtime-api.md) before using forms, listeners, user records, utilities, or container methods. Read [references/recipes.md](references/recipes.md) for message, cron, web, carry, configuration, and Python patterns.

## Quality Gates

- Deliver the whole plugin file with no TODO placeholders.
- Keep secrets in `plugin.Form`, Bucket, or environment variables.
- Keep configuration registration at top level and business execution inside `main`.
- Use the exact runtime casing: `QingLong`, `SmallCat`, `DaiDai`, `checkQr`.
- Use `[status: true|false]` metadata for the plugin-wide runtime switch; use distinct form keys for feature-specific toggles.
- Avoid hidden compatibility files or extra installers; prefer one portable plugin file.
- Run the bundled validator and language parser.
- Explain the trigger, configuration fields, dependency declarations, install path, and test command in the final response.

## Output Contract

When no workspace exists, output one fenced code block containing the complete plugin, then list the intended filename and validation command. When a workspace exists, create the file, validate it, and report the exact path and results.
