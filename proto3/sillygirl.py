import asyncio
import base64
import hashlib
import inspect
import json
import os
import pickle
import platform
import re
import shutil
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile

import grpc

import srpc_pb2
import srpc_pb2_grpc


plugin_id = os.environ.get("PLUGIN_ID", "")
runtime_id = os.environ.get("RUNTIME_ID", "")
grpc_addr = os.environ.get("SILLYGIRL_GRPC_ADDR", "127.0.0.1:50051")
grpc_token = os.environ.get("SILLYGIRL_GRPC_TOKEN", "")
metadata = (("runtime_id", runtime_id), ("sillygirl-runtime-token", grpc_token))

_channel = None
_stub = None
_async_channel = None
_async_stub = None


def get_stub():
    global _channel, _stub
    if _stub is None:
        _channel = grpc.insecure_channel(grpc_addr)
        _stub = srpc_pb2_grpc.SillyGirlServiceStub(_channel)
    return _stub


def get_async_stub():
    global _async_channel, _async_stub
    if _async_stub is None:
        _async_channel = grpc.aio.insecure_channel(grpc_addr)
        _async_stub = srpc_pb2_grpc.SillyGirlServiceStub(_async_channel)
    return _async_stub


def transform(value):
    if not value:
        return None
    if value.startswith("f:"):
        return float(value[2:])
    if value.startswith("d:") or value.startswith("i:"):
        return int(value[2:])
    if value.startswith("b:"):
        return value[2:] == "true"
    if value.startswith("o:"):
        return json.loads(value[2:])
    if value.startswith("p:"):
        return pickle.loads(base64.b64decode(value[2:]))
    return value


def reverse_transform(value):
    try:
        if isinstance(value, bool):
            return "b:true" if value else "b:false"
        if isinstance(value, int):
            return f"d:{value}"
        if isinstance(value, float):
            return f"f:{value}"
        if isinstance(value, str):
            return value
        if value is None:
            return ""
        return "o:" + json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    except Exception:
        return "p:%s" % base64.b64encode(pickle.dumps(value)).decode("utf-8")


reverseTransform = reverse_transform


class Bucket:
    def __init__(self, name):
        self.__name = str(name)

    def __getitem__(self, key):
        return self.__get(key)

    def __getattr__(self, name):
        return self.__get(name)

    def __setitem__(self, key, value):
        self.__set(key, value)

    def __setattr__(self, name, value):
        if name == "_Bucket__name":
            object.__setattr__(self, name, value)
        else:
            self.__set(name, value)

    async def get(self, key, defaultValue=None):
        response = await get_async_stub().BucketGet(
            srpc_pb2.BucketKeyRequest(name=self.__name, key=str(key)),
            metadata=metadata,
        )
        value = transform(response.value)
        return defaultValue if value is None else value

    def __get(self, key, defaultValue=None):
        response = get_stub().BucketGet(
            srpc_pb2.BucketKeyRequest(name=self.__name, key=str(key)),
            metadata=metadata,
        )
        value = transform(response.value)
        return defaultValue if value is None else value

    async def set(self, key, value):
        response = await get_async_stub().BucketSet(
            srpc_pb2.BucketSetRequest(name=self.__name, key=str(key), value=reverse_transform(value)),
            metadata=metadata,
        )
        return {"message": response.message, "changed": response.changed}

    def __set(self, key, value):
        response = get_stub().BucketSet(
            srpc_pb2.BucketSetRequest(name=self.__name, key=str(key), value=reverse_transform(value)),
            metadata=metadata,
        )
        return {"message": response.message, "changed": response.changed}

    async def getAll(self):
        response = await get_async_stub().BucketGetAll(
            srpc_pb2.BucketRequest(name=self.__name),
            metadata=metadata,
        )
        raw = json.loads(response.value or "{}")
        return {key: transform(value) for key, value in raw.items()}

    async def delete(self, key):
        return await self.set(key, None)

    async def deleteAll(self):
        await get_async_stub().BucketDelete(
            srpc_pb2.BucketRequest(name=self.__name),
            metadata=metadata,
        )

    async def keys(self):
        response = await get_async_stub().BucketKeys(
            srpc_pb2.BucketRequest(name=self.__name),
            metadata=metadata,
        )
        return list(response.keys)

    async def len(self):
        response = await get_async_stub().BucketLen(
            srpc_pb2.BucketRequest(name=self.__name),
            metadata=metadata,
        )
        return response.length

    async def buckets(self):
        response = await get_async_stub().BucketBuckets(
            srpc_pb2.Empty(),
            metadata=metadata,
        )
        return list(response.buckets)

    async def getName(self):
        return self.__name

    def watch(self, key, handle):
        async def watch_loop():
            queue = asyncio.Queue()

            async def request_iterator():
                yield srpc_pb2.BucketWatchRequest(
                    name=self.__name,
                    key=str(key),
                    plugin_id=plugin_id,
                )
                while True:
                    item = await queue.get()
                    if item is None:
                        return
                    yield item

            async for response in get_async_stub().BucketWatch(request_iterator(), metadata=metadata):
                try:
                    result = handle(transform(response.old), transform(response.now), response.key)
                    if inspect.isawaitable(result):
                        result = await result
                except Exception as exc:
                    result = {"error": str(exc)}

                payload = {"echo": response.echo}
                if not result:
                    payload["error"] = "VOID"
                else:
                    if "now" in result:
                        payload["now"] = reverse_transform(result["now"])
                    if "message" in result:
                        payload["message"] = str(result["message"])
                    if "error" in result:
                        payload["error"] = str(result["error"])
                await queue.put(srpc_pb2.BucketWatchRequest(**payload))

        try:
            asyncio.get_running_loop().create_task(watch_loop())
        except RuntimeError:
            asyncio.get_event_loop().create_task(watch_loop())


async def _userList():
    """Read ordinary users and this plugin's authorization state."""
    return await Bucket("__plugin_users__").get("list", [])


def _is_schema_node(value):
    return isinstance(value, SchemaNode)


def _normalize_form_field(value, path="field"):
    if _is_schema_node(value):
        return value.toJSON()
    raise RuntimeError(f"form schema {path} must use form.string()/form.boolean()/form.select() helpers")


def normalize_config_schema(fields):
    if not isinstance(fields, dict):
        raise RuntimeError('form(...) only accepts an object like {"token": form.string().title("Token")}')
    result = {"type": "object", "properties": {}}
    for key, value in fields.items():
        if str(key).startswith("_"):
            continue
        result["properties"][key] = _normalize_form_field(value, key)
    return result


def _pluginConfigDefaults(schema):
    schema = schema.toJSON() if _is_schema_node(schema) else (schema or {})
    if not isinstance(schema, dict):
        return None
    if "default" in schema:
        return schema["default"]
    if schema.get("type") == "object" or schema.get("properties"):
        result = {}
        for key, value in (schema.get("properties") or {}).items():
            default_value = _pluginConfigDefaults(value)
            if default_value is not None:
                result[key] = default_value
        return result
    if schema.get("type") == "array":
        return []
    return None


class SchemaNode:
    def __init__(self, schema_type, extra=None):
        self.schema = {"type": schema_type}
        if extra:
            self.schema.update(extra)

    def title(self, value):
        self.schema["title"] = value
        return self

    def description(self, value):
        self.schema["description"] = value
        return self

    def default(self, value):
        self.schema["default"] = value
        return self

    def options(self, value):
        return _apply_schema_options(self, value)

    def required(self, value):
        self.schema["required"] = value
        return self

    def format(self, value):
        self.schema["format"] = value
        return self

    def min(self, value):
        self.schema["minimum"] = value
        return self

    def max(self, value):
        self.schema["maximum"] = value
        return self

    def minLength(self, value):
        self.schema["minLength"] = value
        return self

    def maxLength(self, value):
        self.schema["maxLength"] = value
        return self

    def pattern(self, value):
        self.schema["pattern"] = value
        return self

    def widget(self, value):
        self.schema["ui:widget"] = value
        return self

    def toJSON(self):
        return self.schema


def _apply_schema_options(node, options):
    if isinstance(options, list):
        values = []
        names = []
        for item in options:
            if isinstance(item, dict):
                value = item["value"] if "value" in item else (item.get("id") or item.get("key") or item.get("name") or item.get("label"))
                values.append(value)
                names.append(str(item.get("label") or item.get("name") or value))
            else:
                values.append(item)
                names.append(str(item))
        node.schema["enum"] = values
        if any(name != str(values[index]) for index, name in enumerate(names)):
            node.schema["enumNames"] = names
        return node
    if isinstance(options, dict):
        values = list(options.keys())
        node.schema["enum"] = values
        node.schema["enumNames"] = [str(options[key]) for key in values]
    return node


class _FormHelpers:
    def string(self):
        return SchemaNode("string")

    def number(self):
        return SchemaNode("number")

    def integer(self):
        return SchemaNode("integer")

    def boolean(self):
        return SchemaNode("boolean")

    def array(self, item=None):
        return SchemaNode("array", {} if item is None else {"items": _normalize_form_field(item, "array item")})

    def object(self, props=None):
        return SchemaNode(
            "object",
            {"properties": {key: _normalize_form_field(value, key) for key, value in (props or {}).items()}},
        )

    def select(self, options):
        return _apply_schema_options(SchemaNode("string"), options)


_formHelpers = _FormHelpers()

class _PluginConfigFormInstance:
    def __init__(self, schema):
        self.uuid = plugin_id
        self.jsonSchema = normalize_config_schema(schema)
        self.userConfig = {}
        if os.environ.get("PLUGIN_CONFIG_JSON"):
            try:
                value = json.loads(os.environ["PLUGIN_CONFIG_JSON"])
                if isinstance(value, dict):
                    self.userConfig = value
            except Exception:
                pass
        if os.environ.get("SILLYGIRL_CONFIG_REGISTER_ONLY") == "true":
            target = os.environ.get("SILLYGIRL_CONFIG_SCHEMA_FILE", "")
            if target:
                with open(target, "w", encoding="utf-8") as fp:
                    json.dump(self.jsonSchema, fp, ensure_ascii=False, separators=(",", ":"))
            else:
                print("__SILLYGIRL_CONFIG_SCHEMA__" + json.dumps(self.jsonSchema, ensure_ascii=False))
            os._exit(0)

    async def init(self):
        if not self.uuid:
            return self.userConfig
        await Bucket("plugin_config_schemas").set(self.uuid, self.jsonSchema)
        self.userConfig = await Bucket("plugin_config_values").get(self.uuid, {})
        return self.userConfig

    async def get(self):
        if self.uuid:
            self.userConfig = await Bucket("plugin_config_values").get(self.uuid, {})
        return self.userConfig

    async def Get(self):
        return await self.get()

    async def set(self, values=None):
        if isinstance(values, dict):
            self.userConfig = values
        await Bucket("plugin_config_values").set(self.uuid, self.userConfig or {})
        return {"error": ""}

    async def Set(self, values=None):
        return await self.set(values)


def form(schema):
    return _PluginConfigFormInstance(schema)


for _name in ("string", "number", "integer", "boolean", "array", "object", "select"):
    setattr(form, _name, getattr(_formHelpers, _name))
form.defaults = lambda fields: _pluginConfigDefaults(normalize_config_schema(fields))


async def _read_runtime_panels(key):
    raw = await Bucket("sillyGirl").get(key, [])
    if isinstance(raw, list):
        return raw
    if isinstance(raw, str) and raw.strip():
        text = raw[2:] if raw.startswith("o:") else raw
        try:
            value = json.loads(text)
            return value if isinstance(value, list) else []
        except Exception:
            return []
    return []


def _runtime_panel_index(ref):
    if isinstance(ref, dict):
        ref = ref.get("id") or ref.get("ID")
    try:
        return int(ref)
    except Exception:
        return 0




_CONTAINER_DEFINITIONS = {
    "smallcat": {"key": "smallcat_panels", "label": "smallcat"},
    "qinglong": {"key": "qinglong_panels", "label": "青龙"},
    "daidai": {"key": "daidai_panels", "label": "呆呆"},
}


def _normalize_container_kind(kind=None):
    value = str(kind or "").strip().lower()
    if not value:
        return ""
    if value in ("smallcat", "small_cat", "sc"):
        return "smallcat"
    if value in ("qinglong", "qing_long", "ql", "青龙"):
        return "qinglong"
    if value in ("daidai", "dai_dai", "dd", "呆呆"):
        return "daidai"
    raise RuntimeError(f"未知容器类型：{kind}")


def _public_container_panel(panel, index):
    panel = panel or {}
    return {
        "index": index,
        "id": str(panel.get("id") or ""),
        "name": str(panel.get("name") or ""),
        "address": str(panel.get("address") or ""),
        "status": str(panel.get("status") or ""),
        "message": str(panel.get("message") or ""),
    }


class _Container:
    def __init__(self, options=None):
        self.options = options or {}
        self.QingLong = _QingLong
        self.SmallCat = _SmallCat
        self.DaiDai = _DaiDai

    async def getList(self, kind=None):
        wanted = _normalize_container_kind(kind)
        kinds = [wanted] if wanted else ["smallcat", "qinglong", "daidai"]
        result = {}
        for item in kinds:
            definition = _CONTAINER_DEFINITIONS[item]
            panels = await _read_runtime_panels(definition["key"])
            result[item] = {
                "type": item,
                "key": item,
                "label": definition["label"],
                "total": len(panels),
                "list": [_public_container_panel(panel, index + 1) for index, panel in enumerate(panels)],
            }
        return result[wanted] if wanted else result

    async def count(self, kind):
        info = await self.getList(kind)
        return int(info.get("total") or 0)

    async def get(self, kind, panel_id):
        info = await self.getList(kind)
        index = _runtime_panel_index(panel_id)
        target_id = str(panel_id)
        for item in info.get("list", []):
            if item.get("index") == index or item.get("id") == target_id:
                return item
        return None



def _normalize_path(value, prefix):
    value = str(value or "").strip()
    if not value:
        value = prefix
    if not value.startswith("/"):
        value = "/" + value
    if prefix and value != prefix and not value.startswith(prefix + "/"):
        value = prefix + value
    return value


def _query_string(query=None):
    query = query or {}
    values = urllib.parse.urlencode({key: value for key, value in query.items() if value is not None})
    return "?" + values if values else ""


def _normalize_ids(ids):
    if isinstance(ids, list):
        return ids
    if isinstance(ids, str):
        values = []
        for item in re.split(r"[,\s]+", ids):
            item = item.strip()
            if not item:
                continue
            values.append(int(item) if re.fullmatch(r"-?\d+", item) else item)
        return values
    return [ids]


def _http_json_sync(method, url, headers=None, body=None):
    headers = dict(headers or {})
    data = None
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        headers.setdefault("Content-Type", "application/json")
    request = urllib.request.Request(url, data=data, headers=headers, method=str(method or "GET").upper())
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            raw = response.read().decode("utf-8", "replace")
            status = getattr(response, "status", 200)
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", "replace")
        status = exc.code
    payload = json.loads(raw) if raw.strip() else {}
    if status < 200 or status >= 300:
        raise RuntimeError(payload.get("message") or payload.get("error") or f"HTTP {status}")
    return payload


async def _http_json(method, url, headers=None, body=None):
    return await asyncio.to_thread(_http_json_sync, method, url, headers, body)


class _QingLong:
    def __init__(self, options):
        self.id = _runtime_panel_index(options)
        self.uuid = ""
        self.name = ""
        self.address = ""
        self.panel = None
        self.token = ""
        self.expiration = 0

    async def _ready(self):
        if self.panel is not None:
            return
        panels = await _read_runtime_panels("qinglong_panels")
        if self.id < 1 or self.id > len(panels):
            raise RuntimeError(f"青龙编号 {self.id or ''} 不存在")
        self.panel = panels[self.id - 1]
        self.uuid = self.panel.get("id", "")
        self.name = self.panel.get("name", "")
        self.address = str(self.panel.get("address", "")).rstrip("/")

    async def _ensure_token(self):
        await self._ready()
        now = int(time.time())
        if self.token and self.expiration > now + 60:
            return
        result = await _http_json(
            "GET",
            self.address
            + "/open/auth/token"
            + _query_string({"client_id": self.panel.get("client_id"), "client_secret": self.panel.get("client_secret")}),
        )
        data = result.get("data") or {}
        if result.get("code") != 200 or not data.get("token"):
            raise RuntimeError(result.get("message") or "青龙认证失败")
        self.token = data["token"]
        self.expiration = int(data.get("expiration") or 0)

    async def request(self, method, path, body=None, query=None):
        await self._ensure_token()
        result = await _http_json(
            method,
            self.address + _normalize_path(path, "/open") + _query_string(query),
            {"Authorization": f"Bearer {self.token}"},
            body,
        )
        if "code" in result and result.get("code") != 200:
            raise RuntimeError(result.get("message") or "青龙接口请求失败")
        return result

    async def getEnvs(self, options=None):
        query = {"searchValue": options} if isinstance(options, str) else (options or {})
        result = await self.request("GET", "/envs", None, query)
        return result.get("data", result)

    async def getEnvById(self, env_id):
        result = await self.request("GET", f"/envs/{env_id}")
        return result.get("data", result)

    async def createEnv(self, env):
        result = await self.request("POST", "/envs", env if isinstance(env, list) else [env])
        return result.get("data", result)

    async def updateEnv(self, env):
        result = await self.request("PUT", "/envs", env)
        return result.get("data", result)

    async def deleteEnvs(self, ids):
        result = await self.request("DELETE", "/envs", _normalize_ids(ids))
        return result.get("data", result)

    async def disableEnvs(self, ids):
        result = await self.request("PUT", "/envs/disable", _normalize_ids(ids))
        return result.get("data", result)

    async def enableEnvs(self, ids):
        result = await self.request("PUT", "/envs/enable", _normalize_ids(ids))
        return result.get("data", result)

    async def systemNotify(self, title, content):
        result = await self.request("PUT", "/system/notify", {"title": title, "content": content})
        return result.get("data", result)


def _smallcat_account_openid(value):
    if not isinstance(value, dict):
        return ""
    return str(value.get("openid") or value.get("openId") or value.get("open_id") or "").strip()


def _filter_smallcat_account_payload(value, allowed):
    if isinstance(value, list):
        result = []
        for item in value:
            filtered = _filter_smallcat_account_payload(item, allowed)
            if filtered is not None:
                result.append(filtered)
        return result
    if not isinstance(value, dict):
        return value
    openid = _smallcat_account_openid(value)
    if openid and openid not in allowed:
        return None
    result = {}
    for key, item in value.items():
        filtered = _filter_smallcat_account_payload(item, allowed)
        if filtered is not None:
            result[key] = filtered
    return result


class _SmallCat:
    def __init__(self, options):
        self.id = _runtime_panel_index(options)
        self.uuid = ""
        self.name = ""
        self.address = ""
        self.panel = None

    async def _ready(self):
        if self.panel is not None:
            return
        panels = await _read_runtime_panels("smallcat_panels")
        if self.id < 1 or self.id > len(panels):
            raise RuntimeError(f"smallcat 编号 {self.id or ''} 不存在")
        self.panel = panels[self.id - 1]
        self.uuid = self.panel.get("id", "")
        self.name = self.panel.get("name", "")
        self.address = str(self.panel.get("address", "")).rstrip("/")

    async def request(self, method, path, body=None, query=None):
        await self._ready()
        return await _http_json(
            method,
            self.address + _normalize_path(path, "") + _query_string(query),
            {"auth": str(self.panel.get("api_auth") or "")},
            body,
        )

    async def _post(self, path, options=None):
        return await self.request("POST", path, dict(options or {}))

    async def createQr(self, qr_type):
        return await self.request("POST", "/api/qr/start", qr_type if isinstance(qr_type, dict) else {"type": qr_type})

    async def checkQr(self, uuid):
        return await self.request("GET", "/api/qr/status", None, {"uuid": uuid})

    async def addUser(self, options):
        return await self._post("/api/accounts/add", options)

    async def rescanUser(self, options):
        return await self._post("/api/accounts/rescan", options)

    async def authorizedUsers(self):
        return await Bucket("__plugin_smallcat_authorized__").get(
            "records",
            {"enforced": False, "scope": "smallcat:read", "openids": [], "users": []},
        )

    async def userList(self):
        authorization = await self.authorizedUsers()
        payload = await self.request("GET", "/api/accounts")
        if not authorization or not authorization.get("enforced"):
            return payload
        allowed = {str(item or "").strip() for item in authorization.get("openids", []) if str(item or "").strip()}
        return _filter_smallcat_account_payload(payload, allowed)

    async def checkUsers(self, options):
        return await self._post("/api/accounts/status", options)

    async def setUserRemark(self, options):
        return await self._post("/api/accounts/remark", options)

    async def setUserDisabled(self, options):
        return await self._post("/api/accounts/disable", options)

    async def deleteUser(self, options):
        return await self._post("/api/accounts/delete", options)

    async def proxyList(self):
        return await self.request("GET", "/api/proxies")

    async def testProxy(self, options):
        return await self._post("/api/proxies/test", options)

    async def addProxy(self, options):
        return await self._post("/api/proxies/add", options)

    async def deleteProxy(self, options):
        return await self._post("/api/proxies/delete", options)

    async def creditBalance(self):
        return await self.request("GET", "/credits/balance")

    async def creditLedger(self, query=None):
        params = {"limit": 50} if query is None else ({"limit": query} if isinstance(query, (int, float)) else query)
        return await self.request("GET", "/credits/ledger", None, params)

    async def getCode(self, options):
        return await self._post("/wx/code", options)

    async def getSession(self, options):
        return await self._post("/wx/getsession", options)

    async def refreshSession(self, options):
        return await self._post("/wx/refresh", options)

    async def getUserInfo(self, options):
        return await self._post("/wx/getuserinfo", options)

    async def getEncryptKey(self, options):
        return await self._post("/wx/encryptkey", options)

    async def getPhoneNumber(self, options):
        return await self._post("/wx/getphonenumber", options)

    async def cloud(self, options):
        return await self._post("/wx/cloud", options)

    async def gateway(self, options):
        return await self._post("/wx/gateway", options)

    async def qrCodeAuth(self, options):
        return await self._post("/wx/qrcodeauth", options)

    async def oAuth(self, options):
        return await self._post("/wx/oauth", options)

    async def translateLink(self, options):
        return await self._post("/wx/translatelink", options)

    async def autoAuth(self, options):
        return await self._post("/wx/autoauth", options)

    async def appMsgExt(self, options):
        return await self._post("/wx/appmsgext", options)

    async def appMsgLike(self, options):
        return await self._post("/wx/appmsglike", options)


class _DaiDai:
    def __init__(self, options):
        self.id = _runtime_panel_index(options)
        self.uuid = ""
        self.name = ""
        self.address = ""
        self.panel = None
        self.token = ""
        self.expiration = 0

    async def _ready(self):
        if self.panel is not None:
            return
        panels = await _read_runtime_panels("daidai_panels")
        if self.id < 1 or self.id > len(panels):
            raise RuntimeError(f"呆呆面板编号 {self.id or ''} 不存在")
        self.panel = panels[self.id - 1]
        self.uuid = self.panel.get("id", "")
        self.name = self.panel.get("name", "")
        self.address = str(self.panel.get("address", "")).rstrip("/")

    async def _ensure_token(self):
        await self._ready()
        now = int(time.time())
        if self.token and self.expiration > now + 60:
            return
        result = await _http_json(
            "POST",
            self.address + "/api/open-api/token",
            None,
            {"app_key": self.panel.get("app_key"), "app_secret": self.panel.get("app_secret")},
        )
        data = result.get("data") or {}
        if not data.get("access_token"):
            raise RuntimeError(result.get("message") or result.get("error") or "呆呆面板认证失败")
        self.token = data["access_token"]
        self.expiration = now + int(data.get("expires_in") or 86400)

    async def request(self, method, path, body=None, query=None):
        await self._ensure_token()
        result = await _http_json(
            method,
            self.address + _normalize_path(path, "/api") + _query_string(query),
            {"Authorization": f"Bearer {self.token}"},
            body,
        )
        if result.get("success") is False:
            raise RuntimeError(result.get("message") or result.get("error") or "呆呆面板接口请求失败")
        return result

    async def getEnvs(self, options=None):
        query = {"keyword": options} if isinstance(options, str) else (options or {})
        result = await self.request("GET", "/envs", None, query)
        return result.get("data", result)

    async def getEnvById(self, env_id):
        result = await self.request("GET", f"/envs/{env_id}")
        return result.get("data", result)

    async def createEnv(self, env):
        result = await self.request("POST", "/envs", env)
        return result.get("data", result)

    async def updateEnv(self, env):
        body = dict(env or {})
        env_id = body.pop("id", body.pop("ID", ""))
        result = await self.request("PUT", f"/envs/{env_id}" if env_id else "/envs", body)
        return result.get("data", result)

    async def deleteEnv(self, env_id):
        return await self.request("DELETE", f"/envs/{env_id}")

    async def deleteEnvs(self, ids):
        return await self.request("DELETE", "/envs/batch", {"ids": _normalize_ids(ids)})

    async def enableEnv(self, env_id):
        result = await self.request("PUT", f"/envs/{env_id}/enable")
        return result.get("data", result)

    async def disableEnv(self, env_id):
        result = await self.request("PUT", f"/envs/{env_id}/disable")
        return result.get("data", result)

    async def enableEnvs(self, ids):
        return await self.request("PUT", "/envs/batch/enable", {"ids": _normalize_ids(ids)})

    async def disableEnvs(self, ids):
        return await self.request("PUT", "/envs/batch/disable", {"ids": _normalize_ids(ids)})

    async def getTasks(self, options=None):
        query = {"keyword": options} if isinstance(options, str) else (options or {})
        result = await self.request("GET", "/tasks", None, query)
        return result.get("data", result)

    async def getTaskById(self, task_id):
        result = await self.request("GET", f"/tasks/{task_id}")
        return result.get("data", result)

    async def createTask(self, task):
        result = await self.request("POST", "/tasks", task)
        return result.get("data", result)

    async def updateTask(self, task):
        body = dict(task or {})
        task_id = body.pop("id", body.pop("ID", ""))
        result = await self.request("PUT", f"/tasks/{task_id}" if task_id else "/tasks", body)
        return result.get("data", result)

    async def deleteTask(self, task_id):
        return await self.request("DELETE", f"/tasks/{task_id}")

    async def runTask(self, task_id):
        return await self.request("PUT", f"/tasks/{task_id}/run")

    async def stopTask(self, task_id):
        return await self.request("PUT", f"/tasks/{task_id}/stop")

    async def enableTask(self, task_id):
        return await self.request("PUT", f"/tasks/{task_id}/enable")

    async def disableTask(self, task_id):
        return await self.request("PUT", f"/tasks/{task_id}/disable")

    async def systemNotify(self, title, content):
        return await self.request("POST", "/notifications/send", {"title": title, "content": content})


class Sender:
    def __init__(self, uuid):
        self.__uuid = uuid
        self.destroyed = False

    async def destroy(self):
        if self.destroyed:
            return
        self.destroyed = True
        await get_async_stub().SenderDestroy(
            srpc_pb2.ReplyRequest(uuid=self.__uuid),
            metadata=metadata,
        )

    async def getUserId(self):
        response = await get_async_stub().SenderGetUserId(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def getUserName(self):
        response = await get_async_stub().SenderGetUserName(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def getChatId(self):
        response = await get_async_stub().SenderGetChatId(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def getChatName(self):
        response = await get_async_stub().SenderGetChatName(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def getMessageId(self):
        response = await get_async_stub().SenderGetMessageId(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def getPlatform(self):
        response = await get_async_stub().SenderGetPlatform(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def getBotId(self):
        response = await get_async_stub().SenderGetBotId(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def getContent(self):
        response = await get_async_stub().SenderGetContent(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def isAdmin(self):
        response = await get_async_stub().SenderIsAdmin(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return response.value

    async def param(self, key):
        response = await get_async_stub().SenderParam(
            srpc_pb2.ReplyRequest(uuid=self.__uuid, content=str(key)),
            metadata=metadata,
        )
        return response.value

    async def setContent(self, content):
        await get_async_stub().SenderSetContent(
            srpc_pb2.SenderContentRequest(uuid=self.__uuid, content=str(content)),
            metadata=metadata,
        )

    async def continue_(self):
        await get_async_stub().SenderContinue(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)

    async def getEvent(self):
        response = await get_async_stub().SenderEvent(srpc_pb2.SenderRequest(uuid=self.__uuid), metadata=metadata)
        return json.loads(response.value or "{}")

    async def getAdapter(self):
        return Adapter(await self.getPlatform(), await self.getBotId())

    async def listen(
        self,
        options=None,
        timeout=0,
        rules=None,
        handle=None,
        listen_private=False,
        listen_group=False,
        allow_platforms=None,
        prohibit_platforms=None,
        allow_groups=None,
        prohibit_groups=None,
        allow_users=None,
        prohibit_users=None,
    ):
        if isinstance(options, dict):
            timeout = options.get("timeout", timeout)
            rules = options.get("rules", rules)
            handle = options.get("handle", handle)
            listen_private = options.get("listen_private", listen_private)
            listen_group = options.get("listen_group", listen_group)
            allow_platforms = options.get("allow_platforms", allow_platforms)
            prohibit_platforms = options.get("prohibit_platforms", prohibit_platforms)
            allow_groups = options.get("allow_groups", allow_groups)
            prohibit_groups = options.get("prohibit_groups", prohibit_groups)
            allow_users = options.get("allow_users", allow_users)
            prohibit_users = options.get("prohibit_users", prohibit_users)

        queue = asyncio.Queue()

        async def request_iterator():
            yield srpc_pb2.SenderListenRequest(
                uuid=self.__uuid,
                timeout=int(timeout or 0),
                rules=list(rules or []),
                listen_private=bool(listen_private),
                listen_group=bool(listen_group),
                allow_platforms=list(allow_platforms or []),
                prohibit_platforms=list(prohibit_platforms or []),
                allow_groups=list(allow_groups or []),
                prohibit_groups=list(prohibit_groups or []),
                allow_users=list(allow_users or []),
                prohibit_users=list(prohibit_users or []),
                persistent=self.__uuid == "",
                plugin_id=plugin_id,
            )
            while True:
                item = await queue.get()
                if item is None:
                    return
                yield item

        result_sender = None
        async for response in get_async_stub().SenderListen(request_iterator(), metadata=metadata):
            if response.echo == "END":
                break
            result_sender = Sender(response.uuid) if response.uuid else None
            value = ""
            if handle is not None and result_sender is not None:
                value = handle(result_sender)
                if inspect.isawaitable(value):
                    value = await value
                value = "" if value is None else str(value)
            await queue.put(srpc_pb2.SenderListenRequest(uuid=response.echo, value=value))
        await queue.put(None)
        return result_sender

    def holdOn(self, value):
        return "go_again_" + str(value)

    async def reply(self, content):
        response = await get_async_stub().SenderReply(
            srpc_pb2.ReplyRequest(uuid=self.__uuid, content=str(content)),
            metadata=metadata,
        )
        return response.value

    async def doAction(self, properties):
        response = await get_async_stub().SenderAction(
            srpc_pb2.ReplyRequest(uuid=self.__uuid, content=json.dumps(properties or {}, ensure_ascii=False)),
            metadata=metadata,
        )
        return json.loads(response.value or "null")


    async def pushAdmin(self, content, options=None):
        return await _pushAdmin(content, options or {})


setattr(Sender, "continue", Sender.continue_)
sender = Sender(os.environ.get("SENDER_ID", ""))
s = sender


class Adapter:
    def __init__(self, platform=None, bot_id="", replyHandler=None, actionHandler=None, **kwargs):
        if isinstance(platform, dict):
            options = platform
            platform = options.get("platform", "")
            bot_id = options.get("bot_id", options.get("botId", ""))
            replyHandler = options.get("replyHandler", replyHandler)
            actionHandler = options.get("actionHandler", actionHandler)
        self.platform = str(platform or "")
        self.bot_id = str(bot_id or "")
        self.queue = None
        self.task = None
        if replyHandler is not None or actionHandler is not None:
            try:
                self.task = asyncio.get_running_loop().create_task(self.__run(replyHandler, actionHandler))
            except RuntimeError:
                self.task = asyncio.get_event_loop().create_task(self.__run(replyHandler, actionHandler))

    async def __run(self, replyHandler, actionHandler):
        self.queue = asyncio.Queue()

        async def request_iterator():
            yield srpc_pb2.AdapterRegistRequest(bot_id=self.bot_id, platform=self.platform)
            while True:
                item = await self.queue.get()
                if item is None:
                    return
                yield item

        async for response in get_async_stub().AdapterRegist(request_iterator(), metadata=metadata):
            message = json.loads(response.value or "{}")
            echo = message.pop("echo", "")
            message_type = message.pop("__type__", "")
            handler = replyHandler if message_type == "reply" else actionHandler
            value = ""
            if handler is not None:
                value = handler(message)
                if inspect.isawaitable(value):
                    value = await value
                value = "" if value is None else str(value)
            await self.queue.put(srpc_pb2.AdapterRegistRequest(bot_id=echo, platform=value))

    async def receive(self, message):
        await get_async_stub().AdapterReceive(
            srpc_pb2.AdapterRequest(
                platform=self.platform,
                bot_id=self.bot_id,
                value=json.dumps(message or {}, ensure_ascii=False),
            ),
            metadata=metadata,
        )

    async def push(self, message):
        response = await get_async_stub().AdapterPush(
            srpc_pb2.AdapterRequest(
                platform=self.platform,
                bot_id=self.bot_id,
                value=json.dumps(message or {}, ensure_ascii=False),
            ),
            metadata=metadata,
        )
        return response.value or ""

    async def destroy(self):
        if self.queue is not None:
            await self.queue.put(None)

    async def sender(self, options=None):
        response = await get_async_stub().AdapterSender(
            srpc_pb2.AdapterRequest(
                platform=self.platform,
                bot_id=self.bot_id,
                value=json.dumps(options or {}, ensure_ascii=False),
            ),
            metadata=metadata,
        )
        return Sender(response.value) if response.value else None


class Utils:
    async def userList(self):
        return await _userList()

    async def sleep(self, ms=1000):
        return await _sleep(ms)

    async def restart(self):
        return await _restart()

    async def version(self):
        return {
            "current": str(os.environ.get("SILLYGIRL_VERSION") or "unknown"),
            "remote": str(os.environ.get("SILLYGIRL_REMOTE_VERSION") or os.environ.get("SILLYGIRL_VERSION") or "unknown"),
            "source": str(os.environ.get("SILLYGIRL_VERSION_SOURCE") or ""),
            "repository": str(os.environ.get("SILLYGIRL_REPOSITORY") or ""),
        }

    async def update(self, options=None):
        return await _update(options or {})

    def buildCQTag(self, cq_type, params=None, prefix="CQ"):
        params = params or {}
        values = ",".join(f"{key}={value}" for key, value in params.items())
        return f"[{prefix}:{cq_type}{',' + values if values else ''}]"

    def parseCQText(self, text, prefix="CQ"):
        result = []
        last = 0
        pattern = re.compile(rf"\[{re.escape(prefix)}:(\w+)(.*?)\]", re.S)
        for match in pattern.finditer(str(text or "")):
            if match.start() > last:
                result.append(text[last : match.start()])
            params = {}
            for key, value in re.findall(r"(\w+)=([^,]+)", match.group(2)):
                params[key] = value.strip()
            result.append({"type": match.group(1), "params": params})
            last = match.end()
        if last < len(str(text or "")):
            result.append(str(text or "")[last:])
        return result

    def image(self, url):
        return self.buildCQTag("image", {"url": url})

    def video(self, url):
        return self.buildCQTag("video", {"url": url})


container = _Container()


utils = Utils()


def _normalize_list(value):
    if value is None:
        return []
    if isinstance(value, list):
        return [str(item).strip() for item in value if str(item).strip()]
    return [item.strip() for item in re.split(r"[,&\s]+", str(value)) if item.strip()]


async def _pushAdmin(content, options=None):
    options = options or {}
    result = []
    platforms = _normalize_list(options.get("platform")) + _normalize_list(options.get("platforms"))
    platforms = list(dict.fromkeys(platforms or await Bucket("sillyGirl").buckets()))
    bot_id = str(options.get("botId") or options.get("bot_id") or "")
    explicit_users = _normalize_list(options.get("userIds")) + _normalize_list(options.get("users"))
    explicit_users = list(dict.fromkeys(explicit_users))
    for platform in platforms:
        users = explicit_users or _normalize_list(await Bucket(platform).get("masters", ""))
        adapter = Adapter(platform, bot_id)
        for user_id in users:
            try:
                message_id = await adapter.push({"user_id": user_id, "content": content})
                result.append({"platform": platform, "bot_id": bot_id, "user_id": user_id, "message_id": message_id})
            except Exception as exc:
                result.append({"platform": platform, "bot_id": bot_id, "user_id": user_id, "error": str(exc)})
    return result


async def _sleep(ms=1000):
    await asyncio.sleep(float(ms or 0) / 1000)


async def _restart():
    return await Bucket("sillyGirl").set("started_at", time.strftime("%Y-%m-%d %H:%M:%S"))


def _run_process(cwd, args, timeout=120):
    proc = subprocess.run(
        args,
        cwd=cwd,
        text=True,
        capture_output=True,
        timeout=max(10, min(int(timeout or 120), 600)),
        check=False,
    )
    if proc.returncode != 0:
        message = _compact_runtime_output(proc.stderr or proc.stdout)
        raise RuntimeError(message or f"{args[0]} 执行失败：exit {proc.returncode}")
    return {
        "stdout": proc.stdout or "",
        "stderr": proc.stderr or "",
    }


def _curl_text(urls, timeout=120, extra_args=None):
    last_error = None
    for url in urls:
        try:
            result = _run_process(
                None,
                [
                    "curl",
                    "-fsSL",
                    "--retry",
                    "2",
                    "--connect-timeout",
                    "8",
                    "--max-time",
                    str(timeout),
                    "-H",
                    "User-Agent: sillyGirl",
                    *(extra_args or []),
                    url,
                ],
                timeout,
            )
            return result["stdout"]
        except Exception as exc:
            last_error = exc
    raise RuntimeError(str(last_error or "curl 请求失败"))


def _curl_download(urls, target, timeout=120):
    last_error = None
    for url in urls:
        try:
            _run_process(
                None,
                [
                    "curl",
                    "-fL",
                    "--retry",
                    "2",
                    "--connect-timeout",
                    "8",
                    "--max-time",
                    str(timeout),
                    "-H",
                    "User-Agent: sillyGirl",
                    "-o",
                    target,
                    url,
                ],
                timeout,
            )
            return
        except Exception as exc:
            last_error = exc
    raise RuntimeError(str(last_error or "curl 下载失败"))


def _release_proxy_prefixes():
    configured = str(os.environ.get("SILLYGIRL_GITHUB_PROXY") or "").strip()
    values = [configured] if configured else []
    values.extend(["https://gh-proxy.org", "https://ghproxy.net", "https://cdn.gh-proxy.org"])
    return values


def _release_download_urls(address):
    urls = [f"{prefix.rstrip('/')}/{address}" for prefix in _release_proxy_prefixes()]
    urls.append(address)
    return list(dict.fromkeys(urls))


def _fetch_release_metadata(options, timeout):
    repo = str(options.get("releaseRepo") or os.environ.get("SILLYGIRL_RELEASE_REPO") or "smallfawn/sillyGirl").strip()
    tag = str(options.get("releaseTag") or "").strip()
    api_path = f"releases/tags/{urllib.parse.quote(tag, safe='')}" if tag else "releases/latest"
    url = f"https://api.github.com/repos/{repo}/{api_path}"
    text = _curl_text(_release_download_urls(url), timeout, ["-H", "Accept: application/vnd.github+json"])
    try:
        return json.loads(text)
    except Exception as exc:
        raise RuntimeError(f"GitHub Release 接口返回非 JSON：{text[:200]}") from exc


def _release_goos():
    if os.name == "nt":
        return "windows"
    if sys_platform := os.environ.get("SILLYGIRL_RUNTIME_GOOS"):
        return sys_platform
    if os.uname().sysname.lower() == "darwin":
        return "darwin"
    return "linux"


def _release_goarch():
    machine = (os.environ.get("SILLYGIRL_RUNTIME_GOARCH") or platform.machine()).lower()
    if machine in ("x86_64", "amd64"):
        return "amd64"
    if machine in ("aarch64", "arm64"):
        return "arm64"
    return machine


def _select_release_asset(release, options):
    assets = release.get("assets") if isinstance(release, dict) else []
    assets = assets if isinstance(assets, list) else []
    configured = str(options.get("releaseAsset") or "").strip()
    if configured:
        return next((item for item in assets if item.get("name") == configured or configured in str(item.get("name") or "")), None)
    goos = _release_goos()
    goarch = _release_goarch()
    suffix = ".zip" if goos == "windows" else ".tar.gz"
    return next((item for item in assets if f"_{goos}_{goarch}" in str(item.get("name") or "") and str(item.get("name") or "").endswith(suffix)), None)


def _parse_release_checksum(text, file_name):
    for line in str(text or "").splitlines():
        parts = line.strip().split()
        checked_name = os.path.basename(parts[1].lstrip("*")) if len(parts) >= 2 else ""
        if len(parts) >= 2 and checked_name == file_name and re.match(r"^[a-fA-F0-9]{64}$", parts[0]):
            return parts[0]
    return ""


def _sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _verify_release_checksum(release, asset, archive, timeout):
    assets = release.get("assets") if isinstance(release, dict) else []
    assets = assets if isinstance(assets, list) else []
    checksums = next((item for item in assets if str(item.get("name") or "").lower() == "checksums.txt"), None)
    if not checksums or not checksums.get("browser_download_url"):
        raise RuntimeError("Release 缺少 checksums.txt，拒绝更新")
    text = _curl_text(_release_download_urls(checksums["browser_download_url"]), timeout)
    expected = _parse_release_checksum(text, asset.get("name") or "")
    if not expected:
        raise RuntimeError(f"checksums.txt 中缺少 {asset.get('name')} 的 SHA256")
    actual = _sha256_file(archive)
    if actual.lower() != expected.lower():
        raise RuntimeError(f"Release 包校验失败：{asset.get('name')}")


def _safe_extract_tar(archive, target):
    target_abs = os.path.abspath(target)
    with tarfile.open(archive) as package:
        for member in package.getmembers():
            member_path = os.path.abspath(os.path.join(target, member.name))
            if member_path != target_abs and not member_path.startswith(target_abs + os.sep):
                raise RuntimeError("Release 包包含非法路径，已拒绝解压")
        package.extractall(target)


def _safe_extract_zip(archive, target):
    target_abs = os.path.abspath(target)
    with zipfile.ZipFile(archive) as package:
        for member in package.namelist():
            member_path = os.path.abspath(os.path.join(target, member))
            if member_path != target_abs and not member_path.startswith(target_abs + os.sep):
                raise RuntimeError("Release 包包含非法路径，已拒绝解压")
        package.extractall(target)


def _extract_release_archive(archive, target):
    if str(archive).lower().endswith(".zip"):
        _safe_extract_zip(archive, target)
        return
    _safe_extract_tar(archive, target)


def _walk_files(root):
    for base, _dirs, files in os.walk(root):
        for name in files:
            yield os.path.join(base, name)


def _find_release_binary(root):
    suffix = ".exe" if os.name == "nt" else ""
    for file in _walk_files(root):
        name = os.path.basename(file)
        if name.startswith("sillyGirl_") and (name.endswith(suffix) if suffix else not name.endswith((".zip", ".tar.gz"))):
            return file
    return ""


def _find_release_proto3(root):
    for base, dirs, _files in os.walk(root):
        for name in dirs:
            candidate = os.path.join(base, name)
            if name == "proto3" and os.path.exists(os.path.join(candidate, "sillygirl.py")):
                return candidate
    return ""


def _release_executable_path(options):
    configured = str(options.get("executablePath") or os.environ.get("SILLYGIRL_EXEC_PATH") or "").strip()
    if configured:
        return os.path.abspath(configured)
    if os.name == "nt":
        return os.path.abspath(os.path.join(os.getcwd(), "sillyGirl.exe"))
    return "/app/sillyGirl"


def _install_release_payload(tmp_dir, executable_path):
    binary = _find_release_binary(tmp_dir)
    if not binary:
        raise RuntimeError("Release 包中没有找到 sillyGirl 可执行文件")
    target_dir = os.path.dirname(executable_path)
    os.makedirs(target_dir, exist_ok=True)
    if os.name == "nt":
        ready_path = re.sub(r"\.exe$", ".ready.exe", executable_path, flags=re.I)
        shutil.copyfile(binary, ready_path)
        return
    tmp_target = f"{executable_path}.new-{int(time.time())}"
    backup = f"{executable_path}.bak-{int(time.time())}"
    shutil.copyfile(binary, tmp_target)
    os.chmod(tmp_target, 0o755)
    backed_up = False
    try:
        if os.path.exists(executable_path):
            os.rename(executable_path, backup)
            backed_up = True
        os.rename(tmp_target, executable_path)
        if os.path.exists(backup):
            os.remove(backup)
    except Exception:
        if os.path.exists(tmp_target):
            os.remove(tmp_target)
        if backed_up and os.path.exists(backup) and not os.path.exists(executable_path):
            os.rename(backup, executable_path)
        raise
    proto3 = _find_release_proto3(tmp_dir)
    if proto3:
        target_proto3 = os.path.join(target_dir, "proto3")
        if os.path.exists(target_proto3):
            shutil.rmtree(target_proto3)
        shutil.copytree(proto3, target_proto3)


def _normalize_version_text(value):
    return re.sub(r"^[vV]", "", str(value or "").strip().replace("refs/tags/", ""))


async def _update(options=None):
    try:
        options = options or {}
        if not isinstance(options, dict):
            raise RuntimeError("update options 必须是 dict")
        timeout = max(10, min(int(options.get("timeout") or 120), 600))
        before = os.environ.get("SILLYGIRL_VERSION") or "unknown"
        release = await asyncio.to_thread(_fetch_release_metadata, options, timeout)
        asset = _select_release_asset(release, options)
        if not asset:
            raise RuntimeError(f"未找到适配当前系统的 Release 包：{release.get('tag_name') or release.get('name') or 'latest'}")
        tmp_dir = tempfile.mkdtemp(prefix="sillygirl-update-")
        archive = os.path.join(tmp_dir, re.sub(r'[\\/:*?"<>|]', "_", asset.get("name") or "sillyGirl-release"))
        try:
            await asyncio.to_thread(_curl_download, _release_download_urls(asset.get("browser_download_url")), archive, timeout)
            await asyncio.to_thread(_verify_release_checksum, release, asset, archive, timeout)
            await asyncio.to_thread(_extract_release_archive, archive, tmp_dir)
            await asyncio.to_thread(_install_release_payload, tmp_dir, _release_executable_path(options))
        finally:
            shutil.rmtree(tmp_dir, ignore_errors=True)
        restarted = options.get("restart") is not False
        if restarted:
            await _restart()
        return {
            "status": True,
            "message": "更新完成",
            "data": {
                "mode": "release",
                "repo": release.get("html_url") or f"release:{release.get('tag_name') or ''}",
                "before": before,
                "after": _normalize_version_text(release.get("tag_name") or release.get("name") or ""),
                "changed": True,
                "output": f"已通过 curl 下载并安装 Release 包：{asset.get('name')}。",
                "restarted": restarted,
            },
        }
    except Exception as exc:
        return {"status": False, "message": str(exc), "data": None}


class Console:
    def __init__(self, plugin_id_value):
        self.plugin_id = plugin_id_value

    def log(self, *args):
        self.send_console_request("log", *args)

    def info(self, *args):
        self.send_console_request("info", *args)

    def error(self, *args):
        self.send_console_request("error", *args)

    def debug(self, *args):
        self.send_console_request("debug", *args)

    def send_console_request(self, console_type, *args):
        content = " ".join(map(str, args))
        request = srpc_pb2.ConsoleRequest(type=console_type, content=content, plugin_id=self.plugin_id)
        try:
            get_stub().Console(request, metadata=metadata)
        except Exception:
            print(content)


console = Console(plugin_id)
