package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	proto3assets "github.com/smallfawn/sillyGirl/proto3"
	"github.com/smallfawn/sillyGirl/utils"
)

type nodeDependencyPlugin struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	File  string `json:"file"`
	Path  string `json:"path"`
	Type  string `json:"type"`
}

type nodeDependencyRow struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Dev         bool   `json:"dev"`
	Installed   bool   `json:"installed"`
	Source      string `json:"source"`
	Plugin      string `json:"plugin"`
	PluginTitle string `json:"plugin_title"`
	PluginFile  string `json:"plugin_file"`
	Type        string `json:"type"`
}

type nodeDependencyManifest struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Private         bool              `json:"private"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type nodeDependencyRequest struct {
	Plugin  string `json:"plugin"`
	Package string `json:"package"`
	Dev     bool   `json:"dev"`
	Runtime string `json:"runtime"`
}

type nodeScriptRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type pnpmCommand struct {
	Bin  string
	Args []string
}

type pipxCommand struct {
	Bin  string
	Args []string
}

const defaultPnpmRegistry = "https://registry.npmmirror.com"
const defaultPipxRegistry = "https://pypi.tuna.tsinghua.edu.cn/simple"
const pythonPipxRuntimePackage = "sillygirl-python-runtime"
const pythonGrpcRuntimeDependency = "grpcio==1.83.0"
const pythonProtobufRuntimeDependency = "protobuf==7.35.1"

var nodeSillygirlRuntimeDependencies = map[string]string{
	"@grpc/grpc-js":   "^1.8.18",
	"google-protobuf": "^3.21.2",
}

var (
	jsDocBlockPattern        = regexp.MustCompile(`(?s)/\*\*(.*?)\*/`)
	pythonDocBlockPattern    = regexp.MustCompile(`(?s)(?:"""|''')(.*?)(?:"""|''')`)
	dependencyMetaPattern    = regexp.MustCompile(`(?m)^\s*\*\s*@depe\s+(.+?)\s*$`)
	pythonDependencyPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
	pythonPackageArgPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\-\[\],<>=!~*+]*$`)
	nodePackageArgPattern    = regexp.MustCompile(`^[A-Za-z0-9@._~/-]+$`)
	unsafePackageNamePattern = regexp.MustCompile(`[^a-z0-9._-]+`)
)

var pythonPipxRuntimeDependencies = []string{
	pythonGrpcRuntimeDependency,
	pythonProtobufRuntimeDependency,
}

var nodePnpmOnlyBuiltDependencies = []string{
	"protobufjs",
}

var pythonRuntimeEnvCache = struct {
	sync.Mutex
	ready bool
}{}

var nodeRuntimeDependencyCache sync.Map
var nodeSillygirlModuleCache sync.Map
var pythonInstalledPackagesCache = struct {
	sync.Mutex
	values map[string]string
	expire time.Time
}{}
var pnpmCommandCache = struct {
	sync.Mutex
	cmd pnpmCommand
}{}
var pipxCommandCache = struct {
	sync.Mutex
	cmd pipxCommand
}{}
var nodeCommandCache = struct {
	sync.Mutex
	bin string
}{}
var nodeRuntimeNodePathCache sync.Map

func init() {
	GinApi(GET, "/api/admin/plugin/dependencies", RequireAuth, handlePluginDependencies)
	GinApi(GET, "/api/admin/node/dependencies", RequireAuth, handlePluginDependencies)

	GinApi(GET, "/api/admin/plugin/dependency/registry", RequireAuth, handlePluginDependencyRegistry)
	GinApi(PUT, "/api/admin/plugin/dependency/registry", RequireAuth, handleSetPluginDependencyRegistry)

	GinApi(PUT, "/api/admin/node/dependency/registry", RequireAuth, func(ctx *gin.Context) {
		req := struct {
			Registry string `json:"registry"`
		}{}
		if err := ctx.BindJSON(&req); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		registry, err := normalizePnpmRegistry(req.Registry)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		sillyGirl.Set("pnpm_registry", registry)
		ApiOK(ctx, map[string]string{"registry": registry})
	})

	GinApi(GET, "/api/admin/node/dependency/registry", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, map[string]string{"registry": pnpmRegistry()})
	})

	GinApi(POST, "/api/admin/plugin/dependency", RequireAuth, handleInstallPluginDependency)
	GinApi(POST, "/api/admin/node/dependency", RequireAuth, handleInstallPluginDependency)

	GinApi(DELETE, "/api/admin/plugin/dependency", RequireAuth, handleRemovePluginDependency)
	GinApi(DELETE, "/api/admin/node/dependency", RequireAuth, handleRemovePluginDependency)

	GinApi(GET, "/api/admin/node/script", RequireAuth, func(ctx *gin.Context) {
		id := strings.TrimSpace(ctx.Query("id"))
		f, err := nodeFunctionByID(id)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data, err := os.ReadFile(f.Path)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, map[string]interface{}{
			"id":      f.UUID,
			"name":    f.Title,
			"plugin":  nodePluginNameFromPath(f.Path),
			"path":    f.Path,
			"content": string(data),
		})
	})

	GinApi(POST, "/api/admin/node/script", RequireAuth, func(ctx *gin.Context) {
		req := nodeScriptRequest{}
		_ = ctx.BindJSON(&req)
		fileName, err := normalizeNodeScriptFileName(req.Name)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		title := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		pluginName := safePluginDirName(title)
		class := pluginClassFromExt(filepath.Ext(fileName))
		fileName = pluginName + filepath.Ext(fileName)
		_, index, err := createNodePlugin(pluginName, title, fileName, class)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if err := AddNodePlugin(strings.ReplaceAll(index, "\\", "/"), pluginName, class); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, map[string]interface{}{
			"id":     nameUuid(pluginName),
			"plugin": pluginName,
			"path":   index,
			"file":   filepath.Base(index),
		})
	})

	GinApi(PUT, "/api/admin/node/script", RequireAuth, func(ctx *gin.Context) {
		req := nodeScriptRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		f, err := nodeFunctionByID(req.ID)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if err := AddNodePlugin(strings.ReplaceAll(path, "\\", "/"), nodePluginNameFromPath(path), f.Type); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, nil)
	})

	GinApi(DELETE, "/api/admin/node/script", RequireAuth, func(ctx *gin.Context) {
		req := nodeScriptRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		f, err := nodeFunctionByID(req.ID)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if err := removeNodePluginScript(path); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		AddNodePlugin(strings.ReplaceAll(path, "\\", "/"), nodePluginNameFromPath(path), UNKNOWN)
		ApiOK(ctx, nil)
	})
}

func handlePluginDependencies(ctx *gin.Context) {
	runtime := normalizeDependencyRuntime(ctx.Query("runtime"))
	pluginName := strings.TrimSpace(ctx.Query("plugin"))
	plugins := listDependencyPlugins(runtime)
	data := map[string]interface{}{
		"runtime":      runtime,
		"plugins":      plugins,
		"plugin":       pluginName,
		"dependencies": []nodeDependencyRow{},
		"pnpm":         pnpmDependencyStatus(),
		"pipx":         pipxDependencyStatus(),
	}
	if runtime == PYTHON {
		data["tool"] = data["pipx"]
	} else {
		data["tool"] = data["pnpm"]
	}
	if pluginName != "" {
		plugin, err := dependencyPluginByName(plugins, pluginName, runtime)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		deps, err := readPluginDependencies(runtime, plugin)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data["dependencies"] = deps
	} else {
		rows, err := readSharedPluginDependencies(runtime, plugins)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data["dependencies"] = rows
	}
	ApiOK(ctx, data)
}

func handlePluginDependencyRegistry(ctx *gin.Context) {
	runtime := normalizeDependencyRuntime(ctx.Query("runtime"))
	if runtime == PYTHON {
		ApiOK(ctx, map[string]string{"registry": pipxRegistry()})
		return
	}
	ApiOK(ctx, map[string]string{"registry": pnpmRegistry()})
}

func handleSetPluginDependencyRegistry(ctx *gin.Context) {
	req := struct {
		Runtime  string `json:"runtime"`
		Registry string `json:"registry"`
	}{}
	if err := ctx.BindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	runtime := normalizeDependencyRuntime(req.Runtime)
	if runtime == PYTHON {
		registry, err := normalizePipxRegistry(req.Registry)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		sillyGirl.Set("pipx_registry", registry)
		invalidatePipxRuntimeEnvCache()
		ApiOK(ctx, map[string]string{"registry": registry})
		return
	}
	registry, err := normalizePnpmRegistry(req.Registry)
	if err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	sillyGirl.Set("pnpm_registry", registry)
	ApiOK(ctx, map[string]string{"registry": registry})
}

func handleInstallPluginDependency(ctx *gin.Context) {
	req := nodeDependencyRequest{}
	if err := ctx.BindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	runtime := normalizeDependencyRuntime(req.Runtime)
	output := ""
	var err error
	if runtime == PYTHON {
		output, err = installPythonDependency(req.Plugin, req.Package)
	} else {
		output, err = installNodeDependency(req.Plugin, req.Package, req.Dev)
	}
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if strings.TrimSpace(output) != "" {
			message += "：" + strings.TrimSpace(output)
		}
		ApiFail(ctx, message)
		return
	}
	ApiOK(ctx, output)
}

func handleRemovePluginDependency(ctx *gin.Context) {
	req := nodeDependencyRequest{}
	if err := ctx.BindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	runtime := normalizeDependencyRuntime(req.Runtime)
	output := ""
	var err error
	if runtime == PYTHON {
		output, err = removePythonDependency(req.Plugin, req.Package)
	} else {
		output, err = removeNodeDependency(req.Plugin, req.Package)
	}
	if err != nil {
		message := strings.TrimSpace(err.Error())
		if strings.TrimSpace(output) != "" {
			message += "：" + strings.TrimSpace(output)
		}
		ApiFail(ctx, message)
		return
	}
	ApiOK(ctx, output)
}

func normalizeDependencyRuntime(runtime string) string {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case PYTHON, "py":
		return PYTHON
	default:
		return NODE
	}
}

func pnpmDependencyStatus() map[string]interface{} {
	pnpm, err := resolvePnpmCommand()
	status := map[string]interface{}{
		"available": err == nil,
		"path":      pnpm.Bin,
		"message":   "",
		"registry":  pnpmRegistry(),
	}
	if err != nil {
		status["message"] = err.Error()
	}
	return status
}

func pipxDependencyStatus() map[string]interface{} {
	pipx, err := resolvePipxCommand()
	_, _, pyErr := resolvePythonCommand()
	status := map[string]interface{}{
		"available": err == nil && pyErr == nil,
		"path":      strings.TrimSpace(strings.Join(append([]string{pipx.Bin}, pipx.Args...), " ")),
		"message":   "",
		"registry":  pipxRegistry(),
		"target":    pythonPipxVenvDir(),
	}
	if err != nil {
		status["message"] = err.Error()
	} else if pyErr != nil {
		status["message"] = pyErr.Error()
	}
	return status
}

func listDependencyPlugins(runtime string) []nodeDependencyPlugin {
	runtime = normalizeDependencyRuntime(runtime)
	root := nodePluginsRoot()
	files, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	rows := []nodeDependencyPlugin{}
	for _, file := range files {
		if shouldIgnoreNodePluginEntry(file.Name()) {
			continue
		}
		path := filepath.Join(root, file.Name())
		if index, class := FindMainIndex(strings.ReplaceAll(path, "\\", "/")); index != "" && class == runtime {
			name := nodePluginNameFromPath(index)
			title := name
			for _, f := range Functions {
				if f != nil && f.Type == runtime && f.Path != "" && samePath(f.Path, index) {
					title = firstNonEmpty(f.Title, title)
					break
				}
			}
			rows = append(rows, nodeDependencyPlugin{
				Name:  name,
				Title: title,
				File:  filepath.Base(index),
				Path:  index,
				Type:  runtime,
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func dependencyPluginByName(plugins []nodeDependencyPlugin, name string, runtime string) (nodeDependencyPlugin, error) {
	runtime = normalizeDependencyRuntime(runtime)
	for _, plugin := range plugins {
		if plugin.Name == name {
			return plugin, nil
		}
	}
	if index, err := pluginScriptPath(name, runtime); err == nil {
		return nodeDependencyPlugin{Name: name, Title: name, File: filepath.Base(index), Path: index, Type: runtime}, nil
	}
	return nodeDependencyPlugin{}, fmt.Errorf("%s 脚本插件不存在", dependencyRuntimeLabel(runtime))
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func nodePluginsRoot() string {
	return filepath.Clean(filepath.Join(utils.GetDataHome(), "plugins"))
}

func shouldIgnoreNodePluginEntry(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "node_modules", "python_packages", "package.json", "pnpm-lock.yaml", "package-lock.json", "yarn.lock", "demo.main.js":
		return true
	}
	return false
}

func nodePluginNameFromPath(path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if strings.EqualFold(filepath.Ext(clean), ".js") || strings.EqualFold(filepath.Ext(clean), ".py") {
		return strings.TrimSuffix(filepath.Base(clean), filepath.Ext(clean))
	}
	return ""
}

func nodePluginDir(name string) (string, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(name) == "__shared__" {
		return nodePluginsRoot(), nil
	}
	if _, err := nodePluginScriptPath(name); err != nil {
		return "", err
	}
	return nodePluginsRoot(), nil
}

func nodePluginScriptPath(name string) (string, error) {
	return pluginScriptPath(name, NODE)
}

func pluginScriptPath(name string, runtime string) (string, error) {
	runtime = normalizeDependencyRuntime(runtime)
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("请选择 %s 脚本插件", dependencyRuntimeLabel(runtime))
	}
	if strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return "", errors.New("插件名称不合法")
	}
	root := nodePluginsRoot()
	index := filepath.Clean(filepath.Join(root, name+dependencyRuntimeSuffix(runtime)))
	rel, err := filepath.Rel(root, index)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", errors.New("插件路径不合法")
	}
	if info, err := os.Stat(index); err == nil && !info.IsDir() {
		return index, nil
	}
	dir := filepath.Clean(filepath.Join(root, name))
	rel, err = filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", errors.New("插件路径不合法")
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s 脚本插件不存在", dependencyRuntimeLabel(runtime))
	}
	if index, class := FindMainIndex(strings.ReplaceAll(dir, "\\", "/")); index == "" || class != runtime {
		return "", fmt.Errorf("该插件不是 %s 脚本插件", dependencyRuntimeLabel(runtime))
	} else {
		return filepath.Clean(index), nil
	}
}

func dependencyRuntimeSuffix(runtime string) string {
	if normalizeDependencyRuntime(runtime) == PYTHON {
		return ".py"
	}
	return ".js"
}

func dependencyRuntimeLabel(runtime string) string {
	if normalizeDependencyRuntime(runtime) == PYTHON {
		return "Python"
	}
	return "NodeJS"
}

func checkedNodeScriptPath(path string) (string, error) {
	clean := filepath.Clean(path)
	if !isSupportedScriptExt(filepath.Ext(clean)) {
		return "", errors.New("只允许编辑 JS 或 Python 插件入口文件")
	}
	root := nodePluginsRoot()
	rel, err := filepath.Rel(root, clean)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) || rel == "." {
		return "", errors.New("脚本插件文件路径不合法")
	}
	return clean, nil
}

func nodeFunctionByID(id string) (*common.Function, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("缺少脚本 ID")
	}
	for _, f := range Functions {
		if f.UUID == id {
			if f.Type != NODE && f.Type != PYTHON {
				return nil, errors.New("该脚本不是文件脚本")
			}
			if f.Path == "" {
				return nil, errors.New("脚本缺少文件路径")
			}
			return f, nil
		}
	}
	return nil, errors.New("脚本不存在")
}

func isSupportedScriptExt(ext string) bool {
	return strings.EqualFold(ext, ".js") || strings.EqualFold(ext, ".py")
}

func pluginClassFromExt(ext string) string {
	switch {
	case strings.EqualFold(ext, ".py"):
		return PYTHON
	default:
		return NODE
	}
}

func safePluginDirName(name string) string {
	name = safePackageName(name)
	if name == "" {
		name = "script"
	}
	root := nodePluginsRoot()
	base := name
	for i := 1; ; i++ {
		jsMissing := false
		pyMissing := false
		if _, err := os.Stat(filepath.Join(root, name+".js")); os.IsNotExist(err) {
			jsMissing = true
		}
		if _, err := os.Stat(filepath.Join(root, name+".py")); os.IsNotExist(err) {
			pyMissing = true
		}
		if jsMissing && pyMissing {
			if _, err := os.Stat(filepath.Join(root, name)); os.IsNotExist(err) {
				return name
			}
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}

func normalizeNodeScriptFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "script-" + time.Now().Format("20060102150405")
	}
	if strings.ContainsAny(name, `/\:<>"|?*`) || strings.Contains(name, "..") {
		return "", errors.New("脚本文件名不合法")
	}
	ext := filepath.Ext(name)
	if ext == "" {
		name += ".js"
	} else if !isSupportedScriptExt(ext) {
		return "", errors.New("脚本文件名必须是 .js 或 .py 文件")
	}
	title := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.TrimSpace(title) == "" || title == "." {
		return "", errors.New("脚本文件名不能为空")
	}
	return name, nil
}

func createNodePlugin(pluginName, title, fileName string, class string) (string, string, error) {
	root := nodePluginsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", "", err
	}
	content := ""
	switch class {
	case NODE:
		if err := ensureNodeSillygirlModule(root); err != nil {
			return "", "", err
		}
		if err := ensureNodePackageJSON(root, "sillygirl-plugins"); err != nil {
			return "", "", err
		}
		content = strings.TrimRight(defaultScript(title), "\n") + `

async function main() {
  await s.reply("pong");
}

main();
`
	case PYTHON:
		if _, err := ensurePythonSillygirlModule(); err != nil {
			return "", "", err
		}
		content = defaultPythonScript(title)
	default:
		return "", "", errors.New("不支持的脚本类型")
	}
	index := filepath.Join(root, fileName)
	if _, err := checkedNodeScriptPath(index); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(index, []byte(content), 0644); err != nil {
		return "", "", err
	}
	return root, index, nil
}

func removeNodePluginScript(path string) error {
	root := nodePluginsRoot()
	clean := filepath.Clean(path)
	if filepath.Dir(clean) == root {
		return os.Remove(clean)
	}
	dir := filepath.Dir(clean)
	rel, err := filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) || rel == "." {
		return errors.New("NodeJS 插件路径不合法")
	}
	return os.RemoveAll(dir)
}

func readNodeDependencies(plugin nodeDependencyPlugin) ([]nodeDependencyRow, error) {
	manifest := nodeDependencyManifest{}
	dir := nodePluginWorkDir(plugin.Path)
	if err := ensureNodePackageJSON(dir, "sillygirl-plugins"); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("package.json 解析失败：%v", err)
	}
	rowsByName := map[string]nodeDependencyRow{}
	source := fmt.Sprintf("%s / %s", firstNonEmpty(plugin.Title, plugin.Name), firstNonEmpty(plugin.File, "main.js"))
	for name, version := range manifest.Dependencies {
		rowsByName[name] = nodeDependencyRow{Name: name, Version: version, Dev: false, Installed: true, Source: source, Type: NODE}
	}
	for name, version := range manifest.DevDependencies {
		rowsByName[name] = nodeDependencyRow{Name: name, Version: version, Dev: true, Installed: true, Source: source, Type: NODE}
	}
	for _, name := range nodePluginRequiredDependencies(plugin.Path) {
		if _, ok := rowsByName[name]; !ok {
			rowsByName[name] = nodeDependencyRow{Name: name, Version: "", Installed: false, Source: source, Type: NODE}
		}
	}
	rows := make([]nodeDependencyRow, 0, len(rowsByName))
	for _, row := range rowsByName {
		row.Plugin = plugin.Name
		row.PluginTitle = firstNonEmpty(plugin.Title, plugin.Name)
		row.PluginFile = firstNonEmpty(plugin.File, "main.js")
		row.Type = NODE
		row.Source = source
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].PluginTitle != rows[j].PluginTitle {
			return rows[i].PluginTitle < rows[j].PluginTitle
		}
		if rows[i].Installed != rows[j].Installed {
			return !rows[i].Installed
		}
		if rows[i].Dev != rows[j].Dev {
			return !rows[i].Dev
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

func readSharedNodeDependencies(plugins []nodeDependencyPlugin) ([]nodeDependencyRow, error) {
	dir := nodePluginWorkDir("")
	if err := ensureNodePackageJSON(dir, "sillygirl-plugins"); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "package.json")
	manifest := nodeDependencyManifest{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("package.json 解析失败：%v", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	operationPlugin := "__shared__"
	if len(plugins) > 0 {
		operationPlugin = plugins[0].Name
	}
	rows := []nodeDependencyRow{}
	for name, version := range manifest.Dependencies {
		rows = append(rows, nodeDependencyRow{
			Name:        name,
			Version:     version,
			Dev:         false,
			Installed:   true,
			Source:      "共享依赖 / package.json",
			Plugin:      operationPlugin,
			PluginTitle: "共享依赖",
			PluginFile:  "package.json",
			Type:        NODE,
		})
	}
	for name, version := range manifest.DevDependencies {
		rows = append(rows, nodeDependencyRow{
			Name:        name,
			Version:     version,
			Dev:         true,
			Installed:   true,
			Source:      "共享依赖 / package.json",
			Plugin:      operationPlugin,
			PluginTitle: "共享依赖",
			PluginFile:  "package.json",
			Type:        NODE,
		})
	}

	installed := map[string]bool{}
	for name := range manifest.Dependencies {
		installed[name] = true
	}
	for name := range manifest.DevDependencies {
		installed[name] = true
	}
	for _, plugin := range plugins {
		for _, name := range nodePluginRequiredDependencies(plugin.Path) {
			if installed[name] {
				continue
			}
			rows = append(rows, nodeDependencyRow{
				Name:        name,
				Version:     "",
				Installed:   false,
				Source:      fmt.Sprintf("%s / %s", firstNonEmpty(plugin.Title, plugin.Name), firstNonEmpty(plugin.File, "main.js")),
				Plugin:      plugin.Name,
				PluginTitle: firstNonEmpty(plugin.Title, plugin.Name),
				PluginFile:  firstNonEmpty(plugin.File, "main.js"),
				Type:        NODE,
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Installed != rows[j].Installed {
			return !rows[i].Installed
		}
		if rows[i].PluginTitle != rows[j].PluginTitle {
			return rows[i].PluginTitle < rows[j].PluginTitle
		}
		if rows[i].Dev != rows[j].Dev {
			return !rows[i].Dev
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

func readPluginDependencies(runtime string, plugin nodeDependencyPlugin) ([]nodeDependencyRow, error) {
	if normalizeDependencyRuntime(runtime) == PYTHON {
		return readPythonDependencies(plugin), nil
	}
	return readNodeDependencies(plugin)
}

func readSharedPluginDependencies(runtime string, plugins []nodeDependencyPlugin) ([]nodeDependencyRow, error) {
	if normalizeDependencyRuntime(runtime) == PYTHON {
		return readSharedPythonDependencies(plugins), nil
	}
	return readSharedNodeDependencies(plugins)
}

func readPythonDependencies(plugin nodeDependencyPlugin) []nodeDependencyRow {
	installed, _ := installedPythonPackages()
	rowsByName := map[string]nodeDependencyRow{}
	operationPlugin := "__shared__"
	operationTitle := "共享依赖"
	operationFile := "python_packages"
	if plugin.Name != "" {
		operationPlugin = plugin.Name
		operationTitle = firstNonEmpty(plugin.Title, plugin.Name)
		operationFile = firstNonEmpty(plugin.File, "main.py")
	}
	for name, version := range installed {
		rowsByName[name] = nodeDependencyRow{
			Name:        name,
			Version:     version,
			Installed:   true,
			Source:      "共享依赖 / python_packages",
			Plugin:      operationPlugin,
			PluginTitle: operationTitle,
			PluginFile:  operationFile,
			Type:        PYTHON,
		}
	}
	source := fmt.Sprintf("%s / %s", firstNonEmpty(plugin.Title, plugin.Name), firstNonEmpty(plugin.File, "main.py"))
	for _, name := range pythonPluginRequiredDependencies(plugin.Path) {
		if _, ok := rowsByName[name]; ok {
			continue
		}
		rowsByName[name] = nodeDependencyRow{
			Name:        name,
			Installed:   false,
			Source:      source,
			Plugin:      operationPlugin,
			PluginTitle: operationTitle,
			PluginFile:  operationFile,
			Type:        PYTHON,
		}
	}
	rows := make([]nodeDependencyRow, 0, len(rowsByName))
	for _, row := range rowsByName {
		rows = append(rows, row)
	}
	sortDependencyRows(rows)
	return rows
}

func readSharedPythonDependencies(plugins []nodeDependencyPlugin) []nodeDependencyRow {
	installed, _ := installedPythonPackages()
	installedNames := map[string]bool{}
	rows := []nodeDependencyRow{}
	for name, version := range installed {
		installedNames[name] = true
		rows = append(rows, nodeDependencyRow{
			Name:        name,
			Version:     version,
			Installed:   true,
			Source:      "共享依赖 / python_packages",
			Plugin:      "__shared__",
			PluginTitle: "共享依赖",
			PluginFile:  "python_packages",
			Type:        PYTHON,
		})
	}
	for _, plugin := range plugins {
		for _, name := range pythonPluginRequiredDependencies(plugin.Path) {
			if installedNames[name] {
				continue
			}
			rows = append(rows, nodeDependencyRow{
				Name:        name,
				Installed:   false,
				Source:      fmt.Sprintf("%s / %s", firstNonEmpty(plugin.Title, plugin.Name), firstNonEmpty(plugin.File, "main.py")),
				Plugin:      plugin.Name,
				PluginTitle: firstNonEmpty(plugin.Title, plugin.Name),
				PluginFile:  firstNonEmpty(plugin.File, "main.py"),
				Type:        PYTHON,
			})
		}
	}
	sortDependencyRows(rows)
	return rows
}

func sortDependencyRows(rows []nodeDependencyRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Installed != rows[j].Installed {
			return !rows[i].Installed
		}
		if rows[i].PluginTitle != rows[j].PluginTitle {
			return rows[i].PluginTitle < rows[j].PluginTitle
		}
		if rows[i].Dev != rows[j].Dev {
			return !rows[i].Dev
		}
		return rows[i].Name < rows[j].Name
	})
}

func nodePluginRequiredDependencies(scriptOrDir string) []string {
	index, class := FindMainIndex(strings.ReplaceAll(scriptOrDir, "\\", "/"))
	if index == "" || class != NODE {
		return nil
	}
	data, err := os.ReadFile(index)
	if err != nil {
		return nil
	}
	return parseDeclaredDependencies(string(data), NODE)
}

func pythonPluginRequiredDependencies(scriptOrDir string) []string {
	index, class := FindMainIndex(strings.ReplaceAll(scriptOrDir, "\\", "/"))
	if index == "" || class != PYTHON {
		return nil
	}
	data, err := os.ReadFile(index)
	if err != nil {
		return nil
	}
	return parseDeclaredDependencies(string(data), PYTHON)
}

func nodePluginWorkDir(scriptOrDir string) string {
	root := nodePluginsRoot()
	if scriptOrDir == "" {
		return root
	}
	clean := filepath.Clean(scriptOrDir)
	if rel, err := filepath.Rel(root, clean); err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return root
	}
	if strings.EqualFold(filepath.Ext(clean), ".js") || strings.EqualFold(filepath.Ext(clean), ".py") {
		return filepath.Dir(clean)
	}
	return clean
}

func parseDeclaredDependencies(content string, runtime string) []string {
	block := ""
	if match := jsDocBlockPattern.FindStringSubmatch(content); len(match) > 1 {
		block = match[1]
	}
	if block == "" {
		if match := pythonDocBlockPattern.FindStringSubmatch(content); len(match) > 1 {
			block = match[1]
		}
	}
	if block == "" {
		return nil
	}
	values := []string{}
	matches := dependencyMetaPattern.FindAllStringSubmatch(block, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		raw := strings.TrimSpace(match[1])
		if raw == "" {
			continue
		}
		list := []string{}
		if err := json.Unmarshal([]byte(raw), &list); err == nil {
			values = append(values, list...)
			continue
		}
		manifest := map[string]string{}
		if err := json.Unmarshal([]byte(raw), &manifest); err == nil {
			for name := range manifest {
				values = append(values, name)
			}
			continue
		}
		for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == '，' || r == ' ' || r == '\t'
		}) {
			values = append(values, strings.Trim(item, `"'`))
		}
	}
	return normalizeDependencyNamesForRuntime(values, runtime)
}

func normalizeDependencyNames(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		name := normalizeDependencyName(value)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func normalizeDependencyNamesForRuntime(values []string, runtime string) []string {
	if normalizeDependencyRuntime(runtime) == PYTHON {
		return normalizePythonDependencyNames(values)
	}
	return normalizeDependencyNames(values)
}

func normalizeDependencyName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, ".") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "node:") {
		return ""
	}
	if value == "sillygirl" || nodeBuiltinModules[value] {
		return ""
	}
	parts := strings.Split(value, "/")
	if strings.HasPrefix(value, "@") {
		if len(parts) < 2 {
			return ""
		}
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func normalizePythonDependencyNames(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		name := normalizePythonDependencyName(value)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func normalizePythonDependencyName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, ".") || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "..") {
		return ""
	}
	value = strings.Split(value, ";")[0]
	value = strings.TrimSpace(value)
	for _, sep := range []string{"==", ">=", "<=", "~=", "!=", ">", "<"} {
		if idx := strings.Index(value, sep); idx >= 0 {
			value = value[:idx]
			break
		}
	}
	if idx := strings.Index(value, "["); idx >= 0 {
		value = value[:idx]
	}
	value = strings.ToLower(strings.Trim(value, " \t\r\n._-"))
	if pythonIgnoredModules[strings.ReplaceAll(strings.Split(value, ".")[0], "_", "-")] {
		return ""
	}
	value = strings.ReplaceAll(value, "_", "-")
	if value == "" || pythonIgnoredModules[value] {
		return ""
	}
	if !pythonDependencyPattern.MatchString(value) {
		return ""
	}
	return value
}

var nodeBuiltinModules = map[string]bool{
	"assert": true, "async_hooks": true, "buffer": true, "child_process": true, "cluster": true,
	"console": true, "constants": true, "crypto": true, "dgram": true, "diagnostics_channel": true,
	"dns": true, "domain": true, "events": true, "fs": true, "http": true, "http2": true,
	"https": true, "inspector": true, "module": true, "net": true, "os": true, "path": true,
	"perf_hooks": true, "process": true, "punycode": true, "querystring": true, "readline": true,
	"repl": true, "stream": true, "string_decoder": true, "timers": true, "tls": true, "trace_events": true,
	"tty": true, "url": true, "util": true, "v8": true, "vm": true, "wasi": true, "worker_threads": true,
	"zlib": true,
}

var pythonIgnoredModules = map[string]bool{
	"__future__": true, "_thread": true, "abc": true, "argparse": true, "array": true,
	"asyncio": true, "base64": true, "binascii": true, "bisect": true, "calendar": true,
	"collections": true, "concurrent": true, "contextlib": true, "contextvars": true,
	"copy": true, "csv": true, "ctypes": true, "datetime": true, "decimal": true,
	"email": true, "enum": true, "errno": true, "functools": true, "gc": true,
	"getopt": true, "glob": true, "gzip": true, "hashlib": true, "heapq": true,
	"hmac": true, "html": true, "http": true, "importlib": true, "inspect": true,
	"io": true, "itertools": true, "json": true, "logging": true, "math": true,
	"multiprocessing": true, "operator": true, "os": true, "pathlib": true, "pickle": true,
	"platform": true, "queue": true, "random": true, "re": true, "secrets": true,
	"shlex": true, "shutil": true, "signal": true, "socket": true, "sqlite3": true,
	"ssl": true, "statistics": true, "string": true, "struct": true, "subprocess": true,
	"sys": true, "tempfile": true, "threading": true, "time": true, "traceback": true,
	"types": true, "typing": true, "unicodedata": true, "urllib": true, "uuid": true,
	"warnings": true, "weakref": true, "xml": true, "zipfile": true,
	"sillygirl": true, "srpc-pb2": true, "srpc-pb2-grpc": true,
}

func ensureNodePackageJSON(dir, pluginName string) error {
	path := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(path); err == nil {
		normalized, changed, err := normalizeNodePackageJSON(data, pluginName)
		if err != nil {
			return fmt.Errorf("package.json 解析失败：%v", err)
		}
		if changed {
			if err := os.WriteFile(path, normalized, 0644); err != nil {
				return err
			}
		}
		return ensureNodePnpmWorkspace(dir)
	} else if !os.IsNotExist(err) {
		return err
	}
	manifest := nodeDependencyManifest{
		Name:         safePackageName(pluginName),
		Version:      "1.0.0",
		Private:      true,
		Dependencies: nodeSillygirlRuntimeDependencyCopy(),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return err
	}
	return ensureNodePnpmWorkspace(dir)
}

func ensureNodeSillygirlModule(dir string) error {
	dir = filepath.Clean(dir)
	if _, ok := nodeSillygirlModuleCache.Load(dir); ok {
		return nil
	}
	nodeModules := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nodeModules, 0755); err != nil {
		return err
	}
	moduleDir := filepath.Join(nodeModules, "sillygirl")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return err
	}
	if err := copyNodeRuntimeFile("sillygirl.js", filepath.Join(moduleDir, "index.js")); err != nil {
		return err
	}
	if err := copyNodeRuntimeFile("srpc.js", filepath.Join(moduleDir, "srpc.js")); err != nil {
		return err
	}
	if err := writeFileIfChanged(filepath.Join(moduleDir, "sillygirl.d.ts"), []byte(typeat), 0644); err != nil {
		return err
	}
	if err := writeFileIfChanged(filepath.Join(nodeModules, "sillygirl.d.ts"), []byte(typeat), 0644); err != nil {
		return err
	}
	packageJSON := []byte(`{"name":"sillygirl","main":"index.js","types":"sillygirl.d.ts","private":true}
`)
	if err := writeFileIfChanged(filepath.Join(moduleDir, "package.json"), packageJSON, 0644); err != nil {
		return err
	}
	nodeSillygirlModuleCache.Store(dir, true)
	return nil
}

func copyNodeRuntimeFile(name, target string) error {
	for _, source := range nodeRuntimeSourceCandidates(name) {
		if err := copyFile(source, target); err == nil {
			return nil
		}
	}
	if data, err := proto3assets.ReadRuntimeFile(name); err == nil {
		return writeFileIfChanged(target, data, 0644)
	}
	return fmt.Errorf("缺少 NodeJS sillygirl 运行时文件：%s", name)
}

func nodeRuntimeSourceCandidates(name string) []string {
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

func dedupeCleanPaths(candidates []string) []string {
	seen := map[string]bool{}
	paths := []string{}
	for _, item := range candidates {
		item = filepath.Clean(strings.TrimSpace(item))
		if item == "." || item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, item)
	}
	return paths
}

func copyFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writeFileIfChanged(target, data, 0644)
}

func writeFileIfChanged(target string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, data) {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(target, data, perm)
}

func nodeSillygirlRuntimeDependencyCopy() map[string]string {
	deps := map[string]string{}
	for name, version := range nodeSillygirlRuntimeDependencies {
		deps[name] = version
	}
	return deps
}

func ensureNodePnpmWorkspace(dir string) error {
	path := filepath.Join(dir, "pnpm-workspace.yaml")
	content := "packages:\n  - .\nallowBuilds:\n"
	for _, name := range nodePnpmOnlyBuiltDependencies {
		content += fmt.Sprintf("  %s: true\n", name)
	}
	if data, err := os.ReadFile(path); err == nil && string(data) == content {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func normalizeNodePackageJSON(data []byte, pluginName string) ([]byte, bool, error) {
	manifest := map[string]interface{}{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false, err
	}
	changed := false
	if strings.TrimSpace(fmt.Sprint(manifest["name"])) == "" || fmt.Sprint(manifest["name"]) == "<nil>" {
		manifest["name"] = safePackageName(pluginName)
		changed = true
	}
	if strings.TrimSpace(fmt.Sprint(manifest["version"])) == "" || fmt.Sprint(manifest["version"]) == "<nil>" {
		manifest["version"] = "1.0.0"
		changed = true
	}
	if _, ok := manifest["private"]; !ok {
		manifest["private"] = true
		changed = true
	}
	for _, field := range []string{"dependencies", "devDependencies"} {
		value, exists := manifest[field]
		if !exists {
			continue
		}
		normalized, fieldChanged := normalizeNodePackageDependencyField(value)
		if fieldChanged {
			manifest[field] = normalized
			changed = true
		}
	}
	dependencies, depChanged := normalizeNodePackageDependencyField(manifest["dependencies"])
	if manifest["dependencies"] == nil || depChanged {
		changed = true
	}
	for name, version := range nodeSillygirlRuntimeDependencies {
		if _, ok := dependencies[name]; !ok {
			dependencies[name] = version
			changed = true
		}
	}
	manifest["dependencies"] = dependencies
	if _, ok := manifest["pnpm"]; ok {
		delete(manifest, "pnpm")
		changed = true
	}
	if !changed {
		return data, false, nil
	}
	normalized, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(normalized, '\n'), true, nil
}

func normalizeNodePackageDependencyField(value interface{}) (map[string]string, bool) {
	if value == nil {
		return map[string]string{}, true
	}
	raw, ok := value.(map[string]interface{})
	if !ok {
		return map[string]string{}, true
	}
	normalized := map[string]string{}
	changed := false
	for name, version := range raw {
		text, ok := version.(string)
		if !ok {
			text = strings.TrimSpace(fmt.Sprint(version))
			changed = true
		}
		if text == "" || text == "<nil>" {
			text = "*"
			changed = true
		}
		normalized[name] = text
	}
	return normalized, changed
}

func safePackageName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = unsafePackageNamePattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-._")
	if name == "" {
		return "sillygirl-plugin"
	}
	return name
}

func validateNodePackageArg(pkg string) error {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return errors.New("依赖名称不能为空")
	}
	if strings.ContainsAny(pkg, " \t\r\n\\:") || strings.Contains(pkg, "..") || strings.HasPrefix(pkg, "-") {
		return errors.New("依赖名称不合法")
	}
	if !nodePackageArgPattern.MatchString(pkg) {
		return errors.New("依赖名称只能包含字母、数字、@、/、.、_、-、~")
	}
	return nil
}

func validatePythonPackageArg(pkg string) error {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return errors.New("依赖名称不能为空")
	}
	if strings.ContainsAny(pkg, " \t\r\n\\/:") || strings.Contains(pkg, "..") || strings.HasPrefix(pkg, "-") {
		return errors.New("Python 依赖名称不合法")
	}
	if !pythonPackageArgPattern.MatchString(pkg) {
		return errors.New("Python 依赖名称只能包含包名、extras 和版本约束")
	}
	if normalizePythonDependencyName(pkg) == "" {
		return errors.New("Python 依赖名称不合法")
	}
	return nil
}

func installNodeDependency(pluginName, pkg string, dev bool) (string, error) {
	if err := validateNodePackageArg(pkg); err != nil {
		return "", err
	}
	dir, err := nodePluginDir(pluginName)
	if err != nil {
		return "", err
	}
	if err := ensureNodePackageJSON(dir, "sillygirl-plugins"); err != nil {
		return "", err
	}
	args := []string{"add", pkg}
	if dev {
		args = append(args, "-D")
	}
	output, err := runPnpm(dir, args...)
	if err == nil {
		output = finishNodeDependencyInstall(dir, pluginName, output)
	}
	return output, err
}

// finishNodeDependencyInstall clears the runtime dependency cache and retries
// config-only execution. A plugin can be imported before its npm dependencies
// exist; in that case its source hash does not change after pnpm succeeds, so
// the file watcher cannot be relied on to register the missing config schema.
func finishNodeDependencyInstall(dir, pluginName, output string) string {
	invalidateNodeRuntimeDependencyCache(dir)
	return appendPluginConfigRefreshResult(output, pluginName, NODE)
}

func finishPythonDependencyInstall(pluginName, output string) string {
	invalidatePipxRuntimeEnvCache()
	return appendPluginConfigRefreshResult(output, pluginName, PYTHON)
}

func appendPluginConfigRefreshResult(output, pluginName, runtime string) string {
	if err := refreshPluginConfigSchemas(pluginName, runtime); err != nil {
		console.Warn("%s 依赖已安装，插件配置表单重载失败：%v", dependencyRuntimeLabel(runtime), err)
		if strings.TrimSpace(output) != "" {
			output += "\n"
		}
		output += "插件配置表单重载失败：" + err.Error()
	}
	return output
}

// refreshNodePluginConfigSchemas bypasses the normal source-hash fast path and
// only reruns scripts which declare a config form. Dependencies are shared by
// plugins of the same runtime, so an empty/shared target retries every form
// plugin for that runtime.
func refreshNodePluginConfigSchemas(pluginName string) error {
	return refreshPluginConfigSchemas(pluginName, NODE)
}

func refreshPythonPluginConfigSchemas(pluginName string) error {
	return refreshPluginConfigSchemas(pluginName, PYTHON)
}

func refreshPluginConfigSchemas(pluginName, runtime string) error {
	pluginName = strings.TrimSpace(pluginName)
	runtime = normalizeDependencyRuntime(runtime)
	plugins := listDependencyPlugins(runtime)
	if pluginName != "" && pluginName != "__shared__" {
		plugin, err := dependencyPluginByName(plugins, pluginName, runtime)
		if err != nil {
			return err
		}
		plugins = []nodeDependencyPlugin{plugin}
	}

	failures := make([]string, 0)
	for _, plugin := range plugins {
		data, err := os.ReadFile(plugin.Path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", plugin.Name, err))
			continue
		}
		f, _ := pluginParse(string(data), nameUuid(plugin.Name))
		if f == nil || !f.HasForm {
			continue
		}
		var registerErr error
		if runtime == PYTHON {
			registerErr = registerPythonPluginConfigSchema(plugin.Path, nameUuid(plugin.Name))
		} else {
			registerErr = registerNodePluginConfigSchema(plugin.Path, nameUuid(plugin.Name))
		}
		if registerErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", plugin.Name, registerErr))
		}
	}
	if len(failures) != 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func installPythonDependency(pluginName, pkg string) (string, error) {
	if err := validatePythonPackageArg(pkg); err != nil {
		return "", err
	}
	if err := validatePythonDependencyPlugin(pluginName); err != nil {
		return "", err
	}
	if err := ensurePipxRuntimeEnv(); err != nil {
		return "", err
	}
	output, err := runPipx([]string{"runpip", pythonPipxRuntimePackage, "install", "--upgrade", "--no-cache-dir", pkg}, pipxInstallEnv())
	if err == nil {
		output = finishPythonDependencyInstall(pluginName, output)
	}
	return output, err
}

func removeNodeDependency(pluginName, pkg string) (string, error) {
	if err := validateNodePackageArg(pkg); err != nil {
		return "", err
	}
	dir, err := nodePluginDir(pluginName)
	if err != nil {
		return "", err
	}
	output, err := runPnpm(dir, "remove", pkg)
	if err == nil {
		invalidateNodeRuntimeDependencyCache(dir)
	}
	return output, err
}

func removePythonDependency(pluginName, pkg string) (string, error) {
	if err := validatePythonPackageArg(pkg); err != nil {
		return "", err
	}
	if err := validatePythonDependencyPlugin(pluginName); err != nil {
		return "", err
	}
	name := normalizePythonDependencyName(pkg)
	if name == "" {
		return "", errors.New("Python 依赖名称不合法")
	}
	if err := ensurePipxRuntimeEnv(); err != nil {
		return "", err
	}
	output, err := runPipx([]string{"runpip", pythonPipxRuntimePackage, "uninstall", "-y", name}, nil)
	if err != nil {
		if strings.Contains(strings.ToLower(output), "not installed") {
			return "", fmt.Errorf("未找到 Python 依赖：%s", name)
		}
		return output, err
	}
	invalidatePipxRuntimeEnvCache()
	return output, nil
}

func ensureNodeRuntimeDependencies(dir string) error {
	cacheKey := filepath.Clean(dir)
	if _, ok := nodeRuntimeDependencyCache.Load(cacheKey); ok {
		return nil
	}
	if err := ensureNodePackageJSON(dir, "sillygirl-plugins"); err != nil {
		return err
	}
	missing := false
	for name := range nodeSillygirlRuntimeDependencies {
		if !nodeRuntimeDependencyInstalled(dir, name) {
			missing = true
			break
		}
	}
	if !missing {
		nodeRuntimeDependencyCache.Store(cacheKey, true)
		return nil
	}
	_, err := runPnpm(dir, "install", "--ignore-scripts")
	if err == nil || nodeRuntimeDependenciesInstalled(dir) {
		nodeRuntimeDependencyCache.Store(cacheKey, true)
		return nil
	}
	return err
}

func invalidateNodeRuntimeDependencyCache(dir string) {
	if strings.TrimSpace(dir) == "" {
		nodeRuntimeDependencyCache.Range(func(key, _ any) bool {
			nodeRuntimeDependencyCache.Delete(key)
			return true
		})
		return
	}
	nodeRuntimeDependencyCache.Delete(filepath.Clean(dir))
}

func validatePythonDependencyPlugin(pluginName string) error {
	pluginName = strings.TrimSpace(pluginName)
	if pluginName == "" || pluginName == "__shared__" {
		return nil
	}
	plugins := listDependencyPlugins(PYTHON)
	_, err := dependencyPluginByName(plugins, pluginName, PYTHON)
	return err
}

func pythonPackagesDir() string {
	return filepath.Clean(filepath.Join(nodePluginsRoot(), "python_packages"))
}

func pythonPipxHome() string {
	return pythonPackagesDir()
}

func pythonPipxBinDir() string {
	return filepath.Join(pythonPipxHome(), "bin")
}

func pythonPipxVenvDir() string {
	return filepath.Join(pythonPipxHome(), "venvs", pythonPipxRuntimePackage)
}

func pythonPipxSitePackageDirs() []string {
	venv := pythonPipxVenvDir()
	candidates := []string{
		filepath.Join(venv, "Lib", "site-packages"),
		filepath.Join(venv, "lib", "python"+pythonRequiredVersion, "site-packages"),
	}
	if matches, err := filepath.Glob(filepath.Join(venv, "lib", "python*", "site-packages")); err == nil {
		candidates = append(candidates, matches...)
	}
	seen := map[string]bool{}
	dirs := []string{}
	for _, candidate := range candidates {
		clean := filepath.Clean(candidate)
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		dirs = append(dirs, clean)
	}
	return dirs
}

func ensurePythonPackagesDir() (string, error) {
	dir := pythonPackagesDir()
	return dir, os.MkdirAll(dir, 0755)
}

func installedPythonPackages() (map[string]string, error) {
	pythonInstalledPackagesCache.Lock()
	if pythonInstalledPackagesCache.values != nil && time.Now().Before(pythonInstalledPackagesCache.expire) {
		values := map[string]string{}
		for name, version := range pythonInstalledPackagesCache.values {
			values[name] = version
		}
		pythonInstalledPackagesCache.Unlock()
		return values, nil
	}
	pythonInstalledPackagesCache.Unlock()

	if info, err := os.Stat(pythonPipxVenvDir()); err != nil || !info.IsDir() {
		return map[string]string{}, nil
	}
	output, err := runPipx([]string{"runpip", pythonPipxRuntimePackage, "list", "--format", "json"}, nil)
	if err != nil {
		return nil, err
	}
	output = extractJSONList(output)
	rows := []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}{}
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, row := range rows {
		name := normalizePythonDependencyName(row.Name)
		if name == "" || name == pythonPipxRuntimePackage {
			continue
		}
		result[name] = row.Version
	}
	pythonInstalledPackagesCache.Lock()
	pythonInstalledPackagesCache.values = map[string]string{}
	for name, version := range result {
		pythonInstalledPackagesCache.values[name] = version
	}
	pythonInstalledPackagesCache.expire = time.Now().Add(30 * time.Second)
	pythonInstalledPackagesCache.Unlock()
	return result, nil
}

func extractJSONList(output string) string {
	output = strings.TrimSpace(output)
	start := strings.Index(output, "[")
	end := strings.LastIndex(output, "]")
	if start >= 0 && end >= start {
		return output[start : end+1]
	}
	return output
}

func ensurePipxRuntimeEnv() error {
	pythonRuntimeEnvCache.Lock()
	defer pythonRuntimeEnvCache.Unlock()
	if pythonRuntimeEnvCache.ready {
		return nil
	}
	if err := ensurePipxRuntimeEnvUncached(); err != nil {
		return err
	}
	pythonRuntimeEnvCache.ready = true
	return nil
}

func invalidatePipxRuntimeEnvCache() {
	pythonRuntimeEnvCache.Lock()
	pythonRuntimeEnvCache.ready = false
	pythonRuntimeEnvCache.Unlock()
	invalidateInstalledPythonPackagesCache()
	pythonPathEnvCache.Range(func(key, _ any) bool {
		pythonPathEnvCache.Delete(key)
		return true
	})
}

func invalidateInstalledPythonPackagesCache() {
	pythonInstalledPackagesCache.Lock()
	pythonInstalledPackagesCache.values = nil
	pythonInstalledPackagesCache.expire = time.Time{}
	pythonInstalledPackagesCache.Unlock()
}

func ensurePipxRuntimeEnvUncached() error {
	if _, err := ensurePythonPackagesDir(); err != nil {
		return err
	}
	if _, err := runPipx([]string{"runpip", pythonPipxRuntimePackage, "--version"}, nil); err != nil {
		runtimePackageDir, err := ensurePipxRuntimePackage()
		if err != nil {
			return err
		}
		args := []string{"install", "--force"}
		if bin, baseArgs, err := resolvePythonCommand(); err == nil && len(baseArgs) == 0 {
			args = append(args, "--python", bin)
		}
		args = append(args, runtimePackageDir)
		if _, err := runPipx(args, nil); err != nil {
			return err
		}
	}
	return ensurePipxRuntimeDependencies()
}

func ensurePipxRuntimeDependencies() error {
	installed, err := installedPythonPackages()
	if err != nil {
		return err
	}
	missing := []string{}
	for _, name := range pythonPipxRuntimeDependencies {
		if !pythonRuntimeDependencyInstalled(name, installed) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	args := append([]string{"runpip", pythonPipxRuntimePackage, "install", "--upgrade", "--no-cache-dir"}, missing...)
	_, err = runPipx(args, pipxInstallEnv())
	return err
}

func pythonRuntimeDependencyInstalled(requirement string, installed map[string]string) bool {
	name := normalizePythonDependencyName(requirement)
	version, ok := installed[name]
	if name == "" || !ok {
		return false
	}
	if expected, ok := pythonDependencyExactVersion(requirement); ok {
		return strings.TrimSpace(version) == expected
	}
	return true
}

func pythonDependencyExactVersion(requirement string) (string, bool) {
	idx := strings.Index(requirement, "==")
	if idx < 0 {
		return "", false
	}
	version := strings.TrimSpace(strings.Split(requirement[idx+2:], ";")[0])
	return version, version != ""
}

func ensurePipxRuntimePackage() (string, error) {
	root := filepath.Join(pythonPipxHome(), "_runtime_package")
	pkg := filepath.Join(root, "sillygirl_python_runtime")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		return "", err
	}
	pyproject := `[project]
name = "sillygirl-python-runtime"
version = "1.0.0"
requires-python = ">=3.12"

[project.scripts]
sillygirl-python-runtime = "sillygirl_python_runtime:main"
`
	initPy := `def main():
    return 0
`
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(pkg, "__init__.py"), []byte(initPy), 0644); err != nil {
		return "", err
	}
	return root, nil
}

func runPipx(args []string, extraEnv map[string]string) (string, error) {
	pipx, err := resolvePipxCommand()
	if err != nil {
		return "", err
	}
	cmdArgs := append(append([]string{}, pipx.Args...), args...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pipx.Bin, cmdArgs...)
	cmd.Dir = nodePluginsRoot()
	cmd.Env = pipxEnv()
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	data, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(data))
	if ctx.Err() == context.DeadlineExceeded {
		return output, errors.New("pipx 执行超时")
	}
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		return output, fmt.Errorf("pipx 执行失败：%s", output)
	}
	return output, nil
}

func pipxEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PIPX_HOME="+pythonPipxHome(),
		"PIPX_BIN_DIR="+pythonPipxBinDir(),
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
	)
	if bin, args, err := resolvePythonCommand(); err == nil && len(args) == 0 {
		env = append(env, "PIPX_DEFAULT_PYTHON="+bin)
	}
	return env
}

func pipxInstallEnv() map[string]string {
	env := map[string]string{}
	if registry := pipxRegistry(); registry != "" {
		env["PIP_INDEX_URL"] = registry
	}
	return env
}

func resolvePipxCommand() (pipxCommand, error) {
	pipxCommandCache.Lock()
	if pipxCommandCache.cmd.Bin != "" {
		cmd := pipxCommandCache.cmd
		cmd.Args = append([]string{}, cmd.Args...)
		pipxCommandCache.Unlock()
		return cmd, nil
	}
	pipxCommandCache.Unlock()
	if env := strings.TrimSpace(os.Getenv("SILLYGIRL_PIPX")); env != "" {
		bin, args := splitCommand(env)
		if bin == "" {
			return pipxCommand{}, errors.New("SILLYGIRL_PIPX 为空")
		}
		if !commandWorks(bin, args, "--version") {
			return pipxCommand{}, errors.New("SILLYGIRL_PIPX 不能执行 pipx")
		}
		cmd := pipxCommand{Bin: bin, Args: args}
		cachePipxCommand(cmd)
		return cmd, nil
	}
	for _, name := range []string{"pipx", "pipx.cmd", "pipx.exe"} {
		if path, err := exec.LookPath(name); err == nil && commandWorks(path, nil, "--version") {
			cmd := pipxCommand{Bin: path}
			cachePipxCommand(cmd)
			return cmd, nil
		}
	}
	if bin, args, err := resolvePythonCommand(); err == nil {
		cmdArgs := append(append([]string{}, args...), "-m", "pipx")
		if commandWorks(bin, cmdArgs, "--version") {
			cmd := pipxCommand{Bin: bin, Args: cmdArgs}
			cachePipxCommand(cmd)
			return cmd, nil
		}
	}
	return pipxCommand{}, errors.New("未找到 pipx，请先安装 pipx 或使用 Docker 镜像内置运行时")
}

func cachePipxCommand(cmd pipxCommand) {
	pipxCommandCache.Lock()
	cmd.Args = append([]string{}, cmd.Args...)
	pipxCommandCache.cmd = cmd
	pipxCommandCache.Unlock()
}

func commandWorks(bin string, baseArgs []string, args ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmdArgs := append(append([]string{}, baseArgs...), args...)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Env = pipxEnvWithoutPythonDefault()
	return cmd.Run() == nil
}

func pipxEnvWithoutPythonDefault() []string {
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PIPX_HOME="+pythonPipxHome(),
		"PIPX_BIN_DIR="+pythonPipxBinDir(),
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
	)
	return env
}

func nodeDependencyInstalled(dir, name string) bool {
	return nodeDependencyInstalledAt(filepath.Join(dir, "node_modules"), name)
}

func nodeRuntimeDependencyInstalled(dir, name string) bool {
	if nodeDependencyInstalled(dir, name) {
		return true
	}
	for _, root := range nodeRuntimeModulePaths() {
		if nodeDependencyInstalledAt(root, name) {
			return true
		}
	}
	return false
}

func nodeDependencyInstalledAt(root, name string) bool {
	if root == "" {
		return false
	}
	parts := strings.Split(name, "/")
	path := filepath.Join(append([]string{root}, parts...)...)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func nodeRuntimeDependenciesInstalled(dir string) bool {
	for name := range nodeSillygirlRuntimeDependencies {
		if !nodeRuntimeDependencyInstalled(dir, name) {
			return false
		}
	}
	return true
}

func nodeRuntimeNodePath() string {
	key := os.Getenv("SILLYGIRL_NODE_PATH") + "\x00" + os.Getenv("NODE_PATH")
	if value, ok := nodeRuntimeNodePathCache.Load(key); ok {
		return value.(string)
	}
	value := strings.Join(nodeRuntimeModulePaths(), string(os.PathListSeparator))
	nodeRuntimeNodePathCache.Store(key, value)
	return value
}

func nodeRuntimeModulePaths() []string {
	candidates := []string{}
	for _, env := range []string{os.Getenv("SILLYGIRL_NODE_PATH"), os.Getenv("NODE_PATH")} {
		candidates = append(candidates, filepath.SplitList(env)...)
	}
	candidates = append(candidates,
		filepath.Join(utils.ExecPath, "node-runtime", "node_modules"),
		filepath.Join(filepath.Dir(utils.ExecPath), "node-runtime", "node_modules"),
		"/app/node-runtime/node_modules",
	)
	seen := map[string]bool{}
	paths := []string{}
	for _, item := range candidates {
		item = filepath.Clean(strings.TrimSpace(item))
		if item == "." || item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, item)
	}
	return paths
}

func runPnpm(dir string, args ...string) (string, error) {
	pnpm, err := resolvePnpmCommand()
	if err != nil {
		return "", err
	}
	registry := pnpmRegistry()
	cmdArgs := pnpmCommandArgs(pnpm.Args, args, registry)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pnpm.Bin, cmdArgs...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	data, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(data))
	if ctx.Err() == context.DeadlineExceeded {
		return output, errors.New("pnpm 执行超时")
	}
	if err != nil {
		if output == "" {
			output = err.Error()
		}
		return output, fmt.Errorf("pnpm 执行失败：%s", output)
	}
	return output, nil
}

func pnpmCommandArgs(base []string, args []string, registry string) []string {
	cmdArgs := append([]string{}, base...)
	cmdArgs = append(cmdArgs, args...)
	// pnpm remove does not accept the registry option. It only updates the
	// existing lockfile/package.json, so no registry override is needed.
	if registry != "" && (len(args) == 0 || args[0] != "remove") {
		cmdArgs = append(cmdArgs, "--registry", registry)
	}
	return cmdArgs
}

func resolvePnpmCommand() (pnpmCommand, error) {
	pnpmCommandCache.Lock()
	if pnpmCommandCache.cmd.Bin != "" {
		cmd := pnpmCommandCache.cmd
		cmd.Args = append([]string{}, cmd.Args...)
		pnpmCommandCache.Unlock()
		return cmd, nil
	}
	pnpmCommandCache.Unlock()
	if env := strings.TrimSpace(os.Getenv("SILLYGIRL_PNPM")); env != "" {
		cmd := pnpmCommand{Bin: env}
		cachePnpmCommand(cmd)
		return cmd, nil
	}
	for _, name := range []string{"pnpm", "pnpm.cmd", "pnpm.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			cmd := pnpmCommand{Bin: path}
			cachePnpmCommand(cmd)
			return cmd, nil
		}
	}
	for _, name := range []string{"corepack", "corepack.cmd", "corepack.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			cmd := pnpmCommand{Bin: path, Args: []string{"pnpm"}}
			cachePnpmCommand(cmd)
			return cmd, nil
		}
	}
	return pnpmCommand{}, errors.New("未找到 pnpm，请先安装 pnpm 或启用 Node.js corepack")
}

func cachePnpmCommand(cmd pnpmCommand) {
	pnpmCommandCache.Lock()
	cmd.Args = append([]string{}, cmd.Args...)
	pnpmCommandCache.cmd = cmd
	pnpmCommandCache.Unlock()
}

func pnpmRegistry() string {
	registry := strings.TrimSpace(sillyGirl.GetString("pnpm_registry"))
	if registry == "" {
		return defaultPnpmRegistry
	}
	return registry
}

func normalizePnpmRegistry(registry string) (string, error) {
	registry = strings.TrimSpace(registry)
	if registry == "" {
		registry = defaultPnpmRegistry
	}
	parsed, err := url.Parse(registry)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("pnpm 镜像地址格式错误")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("pnpm 镜像地址只支持 http 或 https")
	}
	return strings.TrimRight(registry, "/"), nil
}

func pipxRegistry() string {
	registry := strings.TrimSpace(sillyGirl.GetString("pipx_registry"))
	if registry == "" {
		return defaultPipxRegistry
	}
	return registry
}

func normalizePipxRegistry(registry string) (string, error) {
	registry = strings.TrimSpace(registry)
	if registry == "" {
		registry = defaultPipxRegistry
	}
	parsed, err := url.Parse(registry)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("pipx 源地址格式错误")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("pipx 源地址只支持 http 或 https")
	}
	return strings.TrimRight(registry, "/"), nil
}

func resolveNodeCommand() (string, error) {
	nodeCommandCache.Lock()
	if nodeCommandCache.bin != "" {
		bin := nodeCommandCache.bin
		nodeCommandCache.Unlock()
		return bin, nil
	}
	nodeCommandCache.Unlock()
	if env := strings.TrimSpace(os.Getenv("SILLYGIRL_NODE")); env != "" {
		cacheNodeCommand(env)
		return env, nil
	}
	for _, name := range []string{"node", "node.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			cacheNodeCommand(path)
			return path, nil
		}
	}
	return "", errors.New("未找到 node，请先安装 Node.js 或使用 Docker 镜像内置 Node")
}

func cacheNodeCommand(bin string) {
	nodeCommandCache.Lock()
	nodeCommandCache.bin = bin
	nodeCommandCache.Unlock()
}
