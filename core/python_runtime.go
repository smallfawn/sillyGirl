package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
	proto3assets "github.com/smallfawn/sillyGirl/proto3"
	"github.com/smallfawn/sillyGirl/utils"
)

const pythonMinimumVersion = "3.12"

// pythonConfigPreloadScript mirrors the Node config-registration preload: real
// modules are preferred, while imports that are still absent during the first
// plugin install resolve to an inert placeholder. This lets a top-level
// form call export its schema before @depe packages are
// installed, without hiding syntax errors or other runtime exceptions.
const pythonConfigPreloadScript = `
from __future__ import annotations

import importlib.abc
import importlib.util
import json
import os
import runpy
import sys
import types


class _SillyGirlMissingDependency(str):
    def __new__(cls):
        return super().__new__(cls, "")

    def __getattr__(self, _name):
        return self

    def __call__(self, *_args, **_kwargs):
        return self

    def __getitem__(self, _key):
        return self

    def __iter__(self):
        return iter(())

    def __await__(self):
        async def _done():
            return self
        return _done().__await__()

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return False

    async def __aenter__(self):
        return self

    async def __aexit__(self, *_args):
        return False

    def __mro_entries__(self, _bases):
        return (object,)


_MISSING_VALUE = _SillyGirlMissingDependency()


class _SillyGirlMissingModule(types.ModuleType):
    def __init__(self, name):
        super().__init__(name)
        self.__path__ = []
        self.__all__ = []

    def __getattr__(self, _name):
        return _MISSING_VALUE


class _SillyGirlMissingImportFinder(importlib.abc.MetaPathFinder, importlib.abc.Loader):
    def find_spec(self, fullname, _path=None, _target=None):
        return importlib.util.spec_from_loader(fullname, self, is_package=True)

    def create_module(self, spec):
        return _SillyGirlMissingModule(spec.name)

    def exec_module(self, _module):
        return None


# Appending keeps built-in, stdlib, local and installed packages authoritative.
# The finder runs only after the normal import machinery found no module.
sys.meta_path.append(_SillyGirlMissingImportFinder())
sys.dont_write_bytecode = True


def _is_schema_node(value):
    return bool(getattr(value, "__schemaNode", False) and getattr(value, "schema", None) is not None)


def _normalize_form_field(value, path="field"):
    if _is_schema_node(value):
        return value.schema
    raise TypeError(f"form schema {path} must use form.string()/form.boolean()/form.select() helpers")


def _normalize_config_schema(fields):
    if not isinstance(fields, dict) or _is_schema_node(fields):
        raise TypeError('new form(...) only accepts an object like {"token": form.string().title("Token")}')
    return {
        "type": "object",
        "properties": {
            key: _normalize_form_field(value, key)
            for key, value in fields.items()
            if not str(key).startswith("_")
        },
    }


def _schema_defaults(schema):
    schema = schema.schema if _is_schema_node(schema) else (schema or {})
    if not isinstance(schema, dict):
        return None
    if "default" in schema:
        return schema["default"]
    if schema.get("type") == "object" or schema.get("properties"):
        result = {}
        for key, value in (schema.get("properties") or {}).items():
            item = _schema_defaults(value)
            if item is not None:
                result[key] = item
        return result
    if schema.get("type") == "array":
        return []
    return None


class _SchemaNode:
    def __init__(self, node_type, extra=None):
        setattr(self, "__schemaNode", True)
        self.schema = {"type": node_type}
        self.schema.update(extra or {})

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
    elif isinstance(options, dict):
        values = list(options.keys())
        node.schema["enum"] = values
        node.schema["enumNames"] = [str(options[key]) for key in values]
    return node


class _Form:
    def __call__(self, schema):
        json_schema = _normalize_config_schema(schema)
        target = os.environ.get("SILLYGIRL_CONFIG_SCHEMA_FILE", "")
        data = json.dumps(json_schema, ensure_ascii=False, separators=(",", ":"))
        if target:
            with open(target, "w", encoding="utf-8") as fp:
                fp.write(data)
        else:
            print("__SILLYGIRL_CONFIG_SCHEMA__" + data)
        raise SystemExit(0)

    def string(self): return _SchemaNode("string")
    def number(self): return _SchemaNode("number")
    def integer(self): return _SchemaNode("integer")
    def boolean(self): return _SchemaNode("boolean")
    def array(self, item=None): return _SchemaNode("array", {} if item is None else {"items": _normalize_form_field(item, "array item")})
    def object(self, props=None): return _SchemaNode("object", {"properties": {key: _normalize_form_field(value, key) for key, value in (props or {}).items()}})
    def select(self, options): return _apply_schema_options(_SchemaNode("string"), options)
    def defaults(self, fields): return _schema_defaults(_normalize_config_schema(fields))


class _Dummy:
    def __init__(self, *_args, **_kwargs):
        pass
    def __getattr__(self, _name):
        return self
    def __call__(self, *_args, **_kwargs):
        return self
    def __getitem__(self, _key):
        return ""
    def __setitem__(self, _key, _value):
        return None
    def __iter__(self):
        return iter(())
    def __await__(self):
        async def _done():
            return self
        return _done().__await__()


_dummy = _Dummy()
_form = _Form()
_sillygirl = types.ModuleType("sillygirl")
_sillygirl.Adapter = _Dummy
_sillygirl.Bucket = _Dummy
_sillygirl.Sender = _Dummy
_sillygirl.form = _form
_sillygirl.sender = _dummy
_sillygirl.container = _dummy
_sillygirl.utils = _dummy
_sillygirl.console = _dummy
sys.modules["sillygirl"] = _sillygirl

if len(sys.argv) != 2:
    raise SystemExit("usage: sillygirl-config-preload.py PLUGIN_PATH")

runpy.run_path(sys.argv[1], run_name="__main__")
`

var pythonSillygirlModuleCache sync.Map
var pythonPathEnvCache sync.Map
var pythonCommandCache = struct {
	sync.Mutex
	bin  string
	args []string
}{}

func ensurePythonSillygirlModule() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("SILLYGIRL_PYTHON_PATH")); configured != "" {
		if _, ok := pythonSillygirlModuleCache.Load(configured); ok {
			return configured, nil
		}
		if err := validatePythonRuntimePath(configured); err != nil {
			return "", err
		}
		pythonSillygirlModuleCache.Store(configured, true)
		return configured, nil
	}
	dir := filepath.Join(utils.ExecPath, "language", "python")
	if _, ok := pythonSillygirlModuleCache.Load(dir); ok {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	for _, name := range []string{"sillygirl.py", "srpc_pb2.py", "srpc_pb2_grpc.py"} {
		if err := copyPythonRuntimeFile(name, filepath.Join(dir, name)); err != nil {
			return "", err
		}
	}
	pythonSillygirlModuleCache.Store(dir, true)
	return dir, nil
}

func ensurePythonConfigPreload(runtimePath string) (string, error) {
	path := filepath.Join(runtimePath, "sillygirl-config-preload.py")
	if err := writeFileIfChanged(path, []byte(pythonConfigPreloadScript), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func validatePythonRuntimePath(dir string) error {
	for _, name := range []string{"sillygirl.py", "srpc_pb2.py", "srpc_pb2_grpc.py"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return fmt.Errorf("Python sillygirl 运行时文件不存在：%s", path)
		}
	}
	return nil
}

func copyPythonRuntimeFile(name, target string) error {
	for _, source := range pythonRuntimeSourceCandidates(name) {
		if err := copyFile(source, target); err == nil {
			return nil
		}
	}
	if data, err := proto3assets.ReadRuntimeFile(name); err == nil {
		return writeFileIfChanged(target, data, 0644)
	}
	return fmt.Errorf("缺少 Python sillygirl 运行时文件：%s", name)
}

func pythonRuntimeSourceCandidates(name string) []string {
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join("proto3", name),
		filepath.Join("..", "proto3", name),
		filepath.Join(utils.ExecPath, "proto3", name),
		filepath.Join(filepath.Dir(utils.ExecPath), "proto3", name),
		filepath.Join("/app", "proto3", name),
	}
	if wd != "" {
		candidates = append(candidates, filepath.Join(wd, "proto3", name))
	}
	return dedupeCleanPaths(candidates)
}

func resolvePythonCommand() (string, []string, error) {
	pythonCommandCache.Lock()
	if pythonCommandCache.bin != "" {
		bin := pythonCommandCache.bin
		args := append([]string{}, pythonCommandCache.args...)
		pythonCommandCache.Unlock()
		return bin, args, nil
	}
	pythonCommandCache.Unlock()

	if configured := strings.TrimSpace(os.Getenv("SILLYGIRL_PYTHON_BIN")); configured != "" {
		bin, args := splitCommand(configured)
		if bin == "" {
			return "", nil, errors.New("SILLYGIRL_PYTHON_BIN 为空")
		}
		if isSupportedPythonVersion(pythonCommandVersion(bin, args)) {
			cachePythonCommand(bin, args)
			return bin, args, nil
		}
		return "", nil, fmt.Errorf("SILLYGIRL_PYTHON_BIN 必须指向 Python %s 或更高版本", pythonMinimumVersion)
	}

	candidates := [][]string{
		{"python3.12"},
		{"python3"},
		{"python"},
		{"py", "-3.12"},
	}
	for _, candidate := range candidates {
		bin := candidate[0]
		args := candidate[1:]
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		if isSupportedPythonVersion(pythonCommandVersion(bin, args)) {
			cachePythonCommand(bin, args)
			return bin, args, nil
		}
	}
	return "", nil, fmt.Errorf("未找到 Python %s 或更高版本，请安装 Python 或使用 Docker 镜像内置运行时", pythonMinimumVersion)
}

func cachePythonCommand(bin string, args []string) {
	pythonCommandCache.Lock()
	pythonCommandCache.bin = bin
	pythonCommandCache.args = append([]string{}, args...)
	pythonCommandCache.Unlock()
}

func splitCommand(command string) (string, []string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func pythonCommandVersion(bin string, args []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	versionArgs := append(append([]string{}, args...), "-c", "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')")
	output, err := exec.CommandContext(ctx, bin, versionArgs...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func isSupportedPythonVersion(version string) bool {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return false
	}
	minParts := strings.Split(pythonMinimumVersion, ".")
	major := utils.Int(parts[0])
	minor := utils.Int(parts[1])
	minMajor := utils.Int(minParts[0])
	minMinor := utils.Int(minParts[1])
	if major != minMajor {
		return major > minMajor
	}
	return minor >= minMinor
}

func pythonPluginPathEnv(runtimePath string) string {
	cacheKey := runtimePath + string(os.PathListSeparator) + os.Getenv("PYTHONPATH")
	if value, ok := pythonPathEnvCache.Load(cacheKey); ok {
		return value.(string)
	}
	items := []string{}
	candidates := append([]string{runtimePath}, pythonPipxSitePackageDirs()...)
	candidates = append(candidates, pythonPackagesDir(), os.Getenv("PYTHONPATH"))
	for _, item := range candidates {
		for _, path := range filepath.SplitList(item) {
			path = strings.TrimSpace(path)
			if path != "" {
				items = append(items, path)
			}
		}
	}
	seen := map[string]bool{}
	paths := []string{}
	for _, item := range items {
		key := strings.ToLower(filepath.Clean(item))
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, item)
	}
	value := strings.Join(paths, string(os.PathListSeparator))
	pythonPathEnvCache.Store(cacheKey, value)
	return value
}

func pythonRuntimeEnvVars(runtimePath string) []string {
	env := []string{
		"PYTHONPATH=" + pythonPluginPathEnv(runtimePath),
		"PYTHONUNBUFFERED=1",
	}
	if cacheDir, err := ensurePythonBytecodeCacheDir(); err == nil {
		env = append(env, "PYTHONPYCACHEPREFIX="+cacheDir)
	} else {
		env = append(env, "PYTHONDONTWRITEBYTECODE=1")
	}
	return env
}

func ensurePythonBytecodeCacheDir() (string, error) {
	dir := filepath.Join(pythonPackagesDir(), "pycache")
	return dir, os.MkdirAll(dir, 0755)
}

func registerPythonPluginConfigSchema(path, uuid string) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(uuid) == "" {
		return errors.New("插件路径或 UUID 为空")
	}
	bin, args, err := resolvePythonCommand()
	if err != nil {
		return err
	}
	pythonPath, err := ensurePythonSillygirlModule()
	if err != nil {
		return err
	}
	preload, err := ensurePythonConfigPreload(pythonPath)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp("", "sillygirl-python-plugin-schema-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	temp.Close()
	defer os.Remove(tempPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmdArgs := append(append([]string{}, args...), "-u", preload, path)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Dir = nodePluginWorkDir(path)
	env := pythonRuntimeEnvVars(pythonPath)
	env = append(env,
		"PLUGIN_ID="+uuid,
		"PLUGIN_CONFIG_JSON="+string(utils.JsonMarshal(getPluginUserConfig(uuid))),
		"SILLYGIRL_CONFIG_REGISTER_ONLY=true",
		"SILLYGIRL_CONFIG_SCHEMA_FILE="+tempPath,
	)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("配置注册超时：%v", ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("配置注册脚本执行失败：%v：%s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return errors.New("插件没有导出配置 schema")
	}
	schema := map[string]interface{}{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("配置 schema 解析失败：%v", err)
	}
	if len(schema) == 0 {
		return errors.New("配置 schema 为空")
	}
	if _, _, err := SetBucketKeyValue(pluginConfigSchemas, uuid, schema); err != nil {
		return err
	}
	console.Log("已注册 Python 插件配置 %s (%s)", filepath.Base(path), uuid)
	return nil
}
