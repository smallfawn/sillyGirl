// [title: 中国移动 Token 获取]
// [name: chinaMobileToken]
// [language: nodejs]
// [class: 工具]
// [author: SillyGirl]
// [version: v1.0.0]
// [public: false]
// [status: true]
// [admin: false]
// [uses_smallcat: true]
// [rule: raw ^(中国移动token|移动token|10086token)$]
// [rule: raw ^(中国移动token|移动token|10086token)\s+(\S+)$]
// [icon: https://api.iconify.design/lucide:key-round.svg]
// [desc: 通过 smallcat OAuth 获取中国移动 10086 微信授权跳转 Cookie]

const { Bucket, container, plugin, sender, user } = require("sillygirl");

const DEFAULT_APPID = "wx43a850f87498127d";
const DEFAULT_SCOPE = "snsapi_base";
const DEFAULT_REDIRECT_URI =
  "https://wx.10086.cn/website/bind/bindAccount/new?redirectSource=SSO_YQS&redirectUrl=https%3A%2F%2Fwx.10086.cn%2Fqwhdsso%2Fredirect%3Fsid%3DQWHDSSOD20260810T185026921DU1021122301H4tsl8R474146&activityId=1021010101&activityName=%E4%B8%AD%E5%9B%BD%E7%A7%BB%E5%8A%A810086";
const WECHAT_UA =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541939) XWEB/19841 Flue";

const configForm = new plugin.Form({
  panel_id: plugin.Form.integer().title("smallcat 编号").default(1).min(1),
  appid: plugin.Form.string().title("AppID").default(DEFAULT_APPID).required(),
  scope: plugin.Form.string().title("Scope").default(DEFAULT_SCOPE).required(),
  redirect_uri: plugin.Form.string()
    .title("Redirect URI")
    .default(DEFAULT_REDIRECT_URI)
    .required(),
  state: plugin.Form.string().title("State").default(""),
  base_cookie: plugin.Form.string()
    .title("基础 Cookie")
    .description("可选。只填写你自己的 Cookie；不填则只使用服务端 Set-Cookie。")
    .default(""),
  user_agent: plugin.Form.string().title("User-Agent").default(WECHAT_UA),
  timeout_sec: plugin.Form.integer().title("请求超时秒数").default(30).min(5).max(120),
  save_cookie: plugin.Form.boolean().title("保存 Cookie").default(true),
});

const store = new Bucket("china_mobile_token");

function text(value) {
  return String(value ?? "").trim();
}

function pickData(value) {
  if (value && typeof value === "object" && "data" in value) {
    return value.data;
  }
  return value;
}

function firstString(value, keys) {
  if (!value || typeof value !== "object") {
    return "";
  }
  for (const key of keys) {
    const current = value[key];
    if (typeof current === "string" && current.trim()) {
      return current.trim();
    }
  }
  return "";
}

function extractFullURL(payload) {
  const data = pickData(payload);
  const direct = firstString(data, ["full_url", "fullUrl", "url", "redirect_url"]);
  if (direct) {
    return direct;
  }
  const nested = pickData(data);
  return firstString(nested, ["full_url", "fullUrl", "url", "redirect_url"]);
}

function collectOpenIDs(value, out = []) {
  if (!value) {
    return out;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      collectOpenIDs(item, out);
    }
    return out;
  }
  if (typeof value !== "object") {
    return out;
  }
  const openid = firstString(value, ["openid", "open_id", "wxOpenid", "userKey"]);
  if (openid) {
    out.push(openid);
  }
  for (const key of ["data", "items", "accounts", "list", "users", "records"]) {
    if (Object.prototype.hasOwnProperty.call(value, key)) {
      collectOpenIDs(value[key], out);
    }
  }
  return out;
}

async function openidsFromBoundUsers() {
  const rows = await user.getUserList({ withRecords: true }).catch(() => []);
  const openids = [];
  for (const row of Array.isArray(rows) ? rows : []) {
    const bindings = row && row.bindings;
    collectOpenIDs(bindings && bindings.smallcat_openids, openids);
    collectOpenIDs(row && row.smallcat_openids, openids);
  }
  return uniq(openids);
}

async function resolveOpenID(smallcat, requested) {
  const explicit = text(requested);
  if (explicit) {
    return explicit;
  }
  const bound = await openidsFromBoundUsers();
  if (bound.length) {
    return bound[0];
  }
  const accountList = await smallcat.userList();
  const openids = uniq(collectOpenIDs(accountList));
  if (!openids.length) {
    throw new Error("未找到 smallcat openid，请在命令后追加 openid，或先绑定 smallcat 账号");
  }
  return openids[0];
}

function uniq(values) {
  return [...new Set((values || []).map(text).filter(Boolean))];
}

function withTimeout(promise, timeoutMs, label) {
  return Promise.race([
    promise,
    new Promise((_, reject) => {
      setTimeout(() => reject(new Error(`${label}超时`)), timeoutMs);
    }),
  ]);
}

function mergeCookieHeader(setCookieHeaders, baseCookie) {
  const pairs = [];
  if (baseCookie) {
    pairs.push(...String(baseCookie).split(";").map(text).filter(Boolean));
  }
  for (const line of setCookieHeaders || []) {
    const pair = String(line || "").split(";")[0].trim();
    if (pair) {
      pairs.push(pair);
    }
  }
  const merged = new Map();
  for (const pair of pairs) {
    const index = pair.indexOf("=");
    if (index > 0) {
      merged.set(pair.slice(0, index).trim(), pair.slice(index + 1).trim());
    }
  }
  return [...merged.entries()].map(([key, value]) => `${key}=${value}`).join("; ");
}

function responseSetCookies(response) {
  if (typeof response.headers.getSetCookie === "function") {
    return response.headers.getSetCookie();
  }
  const value = response.headers.get("set-cookie");
  return value ? [value] : [];
}

async function fetchChinaMobileCookie(fullURL, config) {
  const response = await fetch(fullURL, {
    method: "GET",
    redirect: "manual",
    headers: {
      Host: "wx.10086.cn",
      Connection: "keep-alive",
      "Upgrade-Insecure-Requests": "1",
      "User-Agent": text(config.user_agent) || WECHAT_UA,
      Accept:
        "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/wxpic,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
      "Sec-Fetch-Site": "none",
      "Sec-Fetch-Mode": "navigate",
      "Sec-Fetch-Dest": "document",
      "Accept-Language": "zh-CN,zh;q=0.9",
      Cookie: text(config.base_cookie),
    },
  });
  const setCookies = responseSetCookies(response);
  const cookie = mergeCookieHeader(setCookies, text(config.base_cookie));
  const location = response.headers.get("location") || "";
  const body = await response.text().catch(() => "");
  return {
    http_status: response.status,
    location,
    cookie,
    set_cookie: setCookies,
    body_preview: body.slice(0, 300),
  };
}

async function main() {
  if (sender.isAdmin && !(await sender.isAdmin())) {
    await sender.reply("无权限：请使用傻妞管理员/master 账号执行 10086token");
    return;
  }
  await sender.reply("开始获取中国移动 Cookie...");
  const config = await configForm.get();
  const timeoutMs = Math.max(5, Math.min(Number(config.timeout_sec) || 30, 120)) * 1000;
  const matched = sender.param ? await sender.param(2) : "";
  const smallcat = new container.SmallCat({ id: config.panel_id || 1 });
  const openid = await withTimeout(resolveOpenID(smallcat, matched), timeoutMs, "获取 openid ");
  const oauth = await withTimeout(
    smallcat.oauth({
      appid: text(config.appid) || DEFAULT_APPID,
      scope: text(config.scope) || DEFAULT_SCOPE,
      redirect_uri: text(config.redirect_uri) || DEFAULT_REDIRECT_URI,
      openid,
      state: text(config.state),
    }),
    timeoutMs,
    "smallcat OAuth ",
  );
  const fullURL = extractFullURL(oauth);
  if (!fullURL) {
    throw new Error("smallcat OAuth 未返回 data.full_url");
  }
  const result = await withTimeout(fetchChinaMobileCookie(fullURL, config), timeoutMs, "中国移动请求 ");
  if (!result.cookie) {
    throw new Error(`中国移动未返回 Cookie，HTTP ${result.http_status}`);
  }
  if (config.save_cookie !== false) {
    await store.set(openid, {
      openid,
      cookie: result.cookie,
      full_url: fullURL,
      http_status: result.http_status,
      location: result.location,
      updated_at: Date.now(),
    });
  }
  await sender.reply(
    [
      "中国移动 Cookie 获取成功",
      `openid: ${openid}`,
      `HTTP: ${result.http_status}`,
      result.location ? `Location: ${result.location}` : "",
      `Cookie: ${result.cookie}`,
    ].filter(Boolean).join("\n"),
  );
}

main().catch((error) => sender.reply(`中国移动 Cookie 获取失败：${error.message}`));
