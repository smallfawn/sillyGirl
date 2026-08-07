// [title: AI 示例插件]
// [name: aiExample]
// [description: 可直接修改的 SillyGirl JavaScript 插件模板]
// [author: AI]
// [version: v1.0.0]
// [rule: raw ^示例$]
// [public: false]

const { sender: s, Bucket, plugin } = require("sillygirl");

const Config = new plugin.Form({
  prefix: plugin.Form.string().title("回复前缀").default("SillyGirl"),
});
const Store = new Bucket("ai-example");

async function main() {
  const config = await Config.get();
  const count = Number(await Store.get("count", 0)) + 1;
  await Store.set("count", count);
  await s.reply(`${config.prefix || "SillyGirl"}：已执行 ${count} 次`);
}

main().catch(async (error) => {
  console.error(error);
  await s.reply(`执行失败：${error?.message || error}`);
});
