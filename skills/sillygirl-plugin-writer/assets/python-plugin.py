# [title: AI Python 示例插件]
# [name: aiPythonExample]
# [description: 可直接修改的 SillyGirl Python 插件模板]
# [author: AI]
# [version: v1.0.0]
# [rule: raw ^Python示例$]
# [public: false]

import asyncio

from sillygirl import Bucket, plugin
from sillygirl import sender as s


Config = plugin.Form({
    "prefix": plugin.Form.string().title("回复前缀").default("SillyGirl"),
})
Store = Bucket("ai-python-example")


async def main():
    config = await Config.get()
    count = int(await Store.get("count", 0)) + 1
    await Store.set("count", count)
    await s.reply(f"{config.get('prefix') or 'SillyGirl'}：已执行 {count} 次")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except Exception as error:
        print(f"插件执行失败：{error}")
