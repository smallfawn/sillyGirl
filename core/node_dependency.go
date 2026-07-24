package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

type nodeDependencyPlugin struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	File  string `json:"file"`
	Path  string `json:"path"`
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

const defaultPnpmRegistry = "https://registry.npmmirror.com"

var nodeSillygirlRuntimeDependencies = map[string]string{
	"@grpc/grpc-js":   "^1.8.18",
	"express":         "^4.21.2",
	"google-protobuf": "^3.21.2",
}

func init() {
	GinApi(GET, "/api/node/dependencies", RequireAuth, func(ctx *gin.Context) {
		pluginName := strings.TrimSpace(ctx.Query("plugin"))
		plugins := listNodeDependencyPlugins()
		pnpm, err := resolvePnpmCommand()
		data := map[string]interface{}{
			"plugins":      plugins,
			"plugin":       pluginName,
			"dependencies": []nodeDependencyRow{},
			"pnpm": map[string]interface{}{
				"available": err == nil,
				"path":      pnpm.Bin,
				"message":   "",
				"registry":  pnpmRegistry(),
			},
		}
		if err != nil {
			data["pnpm"].(map[string]interface{})["message"] = err.Error()
		}
		if pluginName != "" {
			plugin, err := nodeDependencyPluginByName(plugins, pluginName)
			if err != nil {
				ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
				return
			}
			deps, err := readNodeDependencies(plugin)
			if err != nil {
				ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
				return
			}
			data["dependencies"] = deps
		} else {
			rows, err := readSharedNodeDependencies(plugins)
			if err != nil {
				ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
				return
			}
			data["dependencies"] = rows
		}
		ctx.JSON(200, map[string]interface{}{"success": true, "data": data})
	})

	GinApi(PUT, "/api/node/dependency/registry", RequireAuth, func(ctx *gin.Context) {
		req := struct {
			Registry string `json:"registry"`
		}{}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		registry, err := normalizePnpmRegistry(req.Registry)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		sillyGirl.Set("pnpm_registry", registry)
		ctx.JSON(200, map[string]interface{}{"success": true, "data": map[string]string{"registry": registry}})
	})

	GinApi(POST, "/api/node/dependency", RequireAuth, func(ctx *gin.Context) {
		req := nodeDependencyRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		output, err := installNodeDependency(req.Plugin, req.Package, req.Dev)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error(), "data": output})
			return
		}
		ctx.JSON(200, map[string]interface{}{"success": true, "data": output})
	})

	GinApi(DELETE, "/api/node/dependency", RequireAuth, func(ctx *gin.Context) {
		req := nodeDependencyRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		output, err := removeNodeDependency(req.Plugin, req.Package)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error(), "data": output})
			return
		}
		ctx.JSON(200, map[string]interface{}{"success": true, "data": output})
	})

	GinApi(GET, "/api/node/script", RequireAuth, func(ctx *gin.Context) {
		id := strings.TrimSpace(ctx.Query("id"))
		f, err := nodeFunctionByID(id)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		data, err := os.ReadFile(f.Path)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"id":      f.UUID,
				"name":    f.Title,
				"plugin":  nodePluginNameFromPath(f.Path),
				"path":    f.Path,
				"content": string(data),
			},
		})
	})

	GinApi(POST, "/api/node/script", RequireAuth, func(ctx *gin.Context) {
		req := nodeScriptRequest{}
		_ = ctx.BindJSON(&req)
		fileName, err := normalizeNodeScriptFileName(req.Name)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		title := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		pluginName := safePluginDirName(title)
		fileName = pluginName + ".js"
		_, index, err := createNodePlugin(pluginName, title, fileName)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := AddNodePlugin(strings.ReplaceAll(index, "\\", "/"), pluginName, NODE); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"id":     nameUuid(pluginName),
				"plugin": pluginName,
				"path":   index,
				"file":   filepath.Base(index),
			},
		})
	})

	GinApi(PUT, "/api/node/script", RequireAuth, func(ctx *gin.Context) {
		req := nodeScriptRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		f, err := nodeFunctionByID(req.ID)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := AddNodePlugin(strings.ReplaceAll(path, "\\", "/"), nodePluginNameFromPath(path), NODE); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		ctx.JSON(200, map[string]interface{}{"success": true})
	})

	GinApi(DELETE, "/api/node/script", RequireAuth, func(ctx *gin.Context) {
		req := nodeScriptRequest{}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		f, err := nodeFunctionByID(req.ID)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := removeNodePluginScript(path); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		AddNodePlugin(strings.ReplaceAll(path, "\\", "/"), nodePluginNameFromPath(path), UNKNOWN)
		ctx.JSON(200, map[string]interface{}{"success": true})
	})
}

func listNodeDependencyPlugins() []nodeDependencyPlugin {
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
		if index, class := FindMainIndex(strings.ReplaceAll(path, "\\", "/")); index != "" && class == NODE {
			name := nodePluginNameFromPath(index)
			title := name
			for _, f := range Functions {
				if f != nil && f.Type == NODE && f.Path != "" && samePath(f.Path, index) {
					title = firstNonEmpty(f.Title, title)
					break
				}
			}
			rows = append(rows, nodeDependencyPlugin{
				Name:  name,
				Title: title,
				File:  filepath.Base(index),
				Path:  index,
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func nodeDependencyPluginByName(plugins []nodeDependencyPlugin, name string) (nodeDependencyPlugin, error) {
	for _, plugin := range plugins {
		if plugin.Name == name {
			return plugin, nil
		}
	}
	if index, err := nodePluginScriptPath(name); err == nil {
		return nodeDependencyPlugin{Name: name, Title: name, File: filepath.Base(index), Path: index}, nil
	}
	return nodeDependencyPlugin{}, errors.New("NodeJS 脚本插件不存在")
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
	case "node_modules", "package.json", "pnpm-lock.yaml", "package-lock.json", "yarn.lock", "demo.main.js":
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
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("请选择 NodeJS 脚本插件")
	}
	if strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return "", errors.New("插件名称不合法")
	}
	root := nodePluginsRoot()
	index := filepath.Clean(filepath.Join(root, name+".js"))
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
		return "", errors.New("NodeJS 脚本插件不存在")
	}
	if index, class := FindMainIndex(strings.ReplaceAll(dir, "\\", "/")); index == "" || class != NODE {
		return "", errors.New("该插件不是 NodeJS 脚本插件")
	} else {
		return filepath.Clean(index), nil
	}
}

func checkedNodePluginDir(dir string) (string, error) {
	root := nodePluginsRoot()
	clean := filepath.Clean(dir)
	rel, err := filepath.Rel(root, clean)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) || rel == "." {
		return "", errors.New("NodeJS 插件目录不合法")
	}
	return clean, nil
}

func checkedNodeScriptPath(path string) (string, error) {
	clean := filepath.Clean(path)
	if !strings.EqualFold(filepath.Ext(clean), ".js") {
		return "", errors.New("只允许编辑 NodeJS 插件入口 JS 文件")
	}
	root := nodePluginsRoot()
	rel, err := filepath.Rel(root, clean)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) || rel == "." {
		return "", errors.New("NodeJS 插件文件路径不合法")
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
			if f.Type != NODE {
				return nil, errors.New("该脚本不是 NodeJS 脚本")
			}
			if f.Path == "" {
				return nil, errors.New("NodeJS 脚本缺少文件路径")
			}
			return f, nil
		}
	}
	return nil, errors.New("NodeJS 脚本不存在")
}

func safePluginDirName(name string) string {
	name = safePackageName(name)
	if name == "" {
		name = "script"
	}
	root := nodePluginsRoot()
	base := name
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(root, name+".js")); os.IsNotExist(err) {
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
	} else if !strings.EqualFold(ext, ".js") {
		return "", errors.New("脚本文件名必须是 .js 文件")
	}
	title := strings.TrimSuffix(name, filepath.Ext(name))
	if strings.TrimSpace(title) == "" || title == "." {
		return "", errors.New("脚本文件名不能为空")
	}
	return name, nil
}

func createNodePlugin(pluginName, title, fileName string) (string, string, error) {
	root := nodePluginsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", "", err
	}
	if err := ensureNodeSillygirlModule(root); err != nil {
		return "", "", err
	}
	if err := ensureNodePackageJSON(root, "sillygirl-plugins"); err != nil {
		return "", "", err
	}
	content := strings.TrimRight(defaultScript(title), "\n") + `

async function main() {
  await s.reply("pong");
}

main();
`
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
		rowsByName[name] = nodeDependencyRow{Name: name, Version: version, Dev: false, Installed: true, Source: source}
	}
	for name, version := range manifest.DevDependencies {
		rowsByName[name] = nodeDependencyRow{Name: name, Version: version, Dev: true, Installed: true, Source: source}
	}
	for _, name := range nodePluginRequiredDependencies(plugin.Path) {
		if _, ok := rowsByName[name]; !ok {
			rowsByName[name] = nodeDependencyRow{Name: name, Version: "", Installed: false, Source: source}
		}
	}
	for _, name := range nodePluginIndexDependencies(plugin.Name) {
		if _, ok := rowsByName[name]; ok {
			continue
		}
		rowsByName[name] = nodeDependencyRow{Name: name, Version: "", Installed: false, Source: source}
	}
	rows := make([]nodeDependencyRow, 0, len(rowsByName))
	for _, row := range rowsByName {
		row.Plugin = plugin.Name
		row.PluginTitle = firstNonEmpty(plugin.Title, plugin.Name)
		row.PluginFile = firstNonEmpty(plugin.File, "main.js")
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
		required := append(nodePluginRequiredDependencies(plugin.Path), nodePluginIndexDependencies(plugin.Name)...)
		for _, name := range normalizeDependencyNames(required) {
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

func nodePluginRequiredDependencies(scriptOrDir string) []string {
	index, class := FindMainIndex(strings.ReplaceAll(scriptOrDir, "\\", "/"))
	if index == "" || class != NODE {
		return nil
	}
	data, err := os.ReadFile(index)
	if err != nil {
		return nil
	}
	return parseNodeRequires(string(data))
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

func nodePluginIndexDependencies(pluginName string) []string {
	deps := []string{}
	for _, f := range plugin_list {
		if f == nil || f.Type != NODE {
			continue
		}
		if f.Title == pluginName || strings.TrimSuffix(f.Title, ".js") == pluginName {
			deps = append(deps, f.Dependencies...)
		}
	}
	return normalizeDependencyNames(deps)
}

func parseNodeRequires(content string) []string {
	matches := regexp.MustCompile(`\brequire\s*\(\s*["']([^"']+)["']\s*\)`).FindAllStringSubmatch(content, -1)
	deps := []string{}
	for _, match := range matches {
		if len(match) > 1 {
			deps = append(deps, match[1])
		}
	}
	return normalizeDependencyNames(deps)
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

func ensureNodePackageJSON(dir, pluginName string) error {
	path := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(path); err == nil {
		normalized, changed, err := normalizeNodePackageJSON(data, pluginName)
		if err != nil {
			return fmt.Errorf("package.json 解析失败：%v", err)
		}
		if changed {
			return os.WriteFile(path, normalized, 0644)
		}
		return nil
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
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func ensureNodeSillygirlModule(dir string) error {
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
	if err := os.WriteFile(filepath.Join(moduleDir, "sillygirl.d.ts"), []byte(typeat), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "sillygirl.d.ts"), []byte(typeat), 0644); err != nil {
		return err
	}
	packageJSON := []byte(`{"name":"sillygirl","main":"index.js","types":"sillygirl.d.ts","private":true}
`)
	return os.WriteFile(filepath.Join(moduleDir, "package.json"), packageJSON, 0644)
}

func copyNodeRuntimeFile(name, target string) error {
	for _, source := range nodeRuntimeSourceCandidates(name) {
		if err := copyFile(source, target); err == nil {
			return nil
		}
	}
	return fmt.Errorf("缺少 NodeJS sillygirl 运行时文件：%s", name)
}

func nodeRuntimeSourceCandidates(name string) []string {
	return []string{
		filepath.Join("proto3", name),
		filepath.Join("..", "proto3", name),
		filepath.Join(utils.ExecPath, "proto3", name),
		filepath.Join(filepath.Dir(utils.ExecPath), "proto3", name),
	}
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func nodeSillygirlRuntimeDependencyCopy() map[string]string {
	deps := map[string]string{}
	for name, version := range nodeSillygirlRuntimeDependencies {
		deps[name] = version
	}
	return deps
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
	name = regexp.MustCompile(`[^a-z0-9._-]+`).ReplaceAllString(name, "-")
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
	if !regexp.MustCompile(`^[A-Za-z0-9@._~/-]+$`).MatchString(pkg) {
		return errors.New("依赖名称只能包含字母、数字、@、/、.、_、-、~")
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
	return runPnpm(dir, args...)
}

func removeNodeDependency(pluginName, pkg string) (string, error) {
	if err := validateNodePackageArg(pkg); err != nil {
		return "", err
	}
	dir, err := nodePluginDir(pluginName)
	if err != nil {
		return "", err
	}
	return runPnpm(dir, "remove", pkg)
}

func ensureNodeRuntimeDependencies(dir string) error {
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
		return nil
	}
	_, err := runPnpm(dir, "install", "--ignore-scripts")
	if err == nil || nodeRuntimeDependenciesInstalled(dir) {
		return nil
	}
	return err
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
	return strings.Join(nodeRuntimeModulePaths(), string(os.PathListSeparator))
}

func nodeRuntimeModulePaths() []string {
	candidates := []string{}
	for _, env := range []string{os.Getenv("SILLYGIRL_NODE_PATH"), os.Getenv("NODE_PATH")} {
		for _, item := range filepath.SplitList(env) {
			candidates = append(candidates, item)
		}
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
	cmdArgs := append([]string{}, pnpm.Args...)
	cmdArgs = append(cmdArgs, args...)
	if registry != "" {
		cmdArgs = append(cmdArgs, "--registry", registry)
	}
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

func resolvePnpmCommand() (pnpmCommand, error) {
	if env := strings.TrimSpace(os.Getenv("SILLYGIRL_PNPM")); env != "" {
		return pnpmCommand{Bin: env}, nil
	}
	for _, name := range []string{"pnpm", "pnpm.cmd", "pnpm.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return pnpmCommand{Bin: path}, nil
		}
	}
	for _, name := range []string{"corepack", "corepack.cmd", "corepack.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return pnpmCommand{Bin: path, Args: []string{"pnpm"}}, nil
		}
	}
	return pnpmCommand{}, errors.New("未找到 pnpm，请先安装 pnpm 或启用 Node.js corepack")
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

func resolveNodeCommand() (string, error) {
	if env := strings.TrimSpace(os.Getenv("SILLYGIRL_NODE")); env != "" {
		return env, nil
	}
	for _, name := range []string{"node", "node.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("未找到 node，请先安装 Node.js 或使用 Docker 镜像内置 Node")
}
