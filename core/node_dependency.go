package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
			rows := []nodeDependencyRow{}
			for _, plugin := range plugins {
				deps, err := readNodeDependencies(plugin)
				if err != nil {
					ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
					return
				}
				rows = append(rows, deps...)
			}
			data["dependencies"] = rows
		}
		ctx.JSON(200, map[string]interface{}{"success": true, "data": data})
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
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "script-" + time.Now().Format("20060102150405")
		}
		pluginName := safePluginDirName(name)
		dir, err := createNodePlugin(pluginName, name)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		index := filepath.Join(dir, "main.js")
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
		dir, err := checkedNodePluginDir(filepath.Dir(f.Path))
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		AddNodePlugin(strings.ReplaceAll(filepath.Join(dir, "main.js"), "\\", "/"), filepath.Base(dir), UNKNOWN)
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
		if !file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, file.Name())
		if index, class := FindMainIndex(strings.ReplaceAll(dir, "\\", "/")); index != "" && class == NODE {
			title := file.Name()
			for _, f := range Functions {
				if f != nil && f.Type == NODE && f.Path != "" && samePath(f.Path, index) {
					title = firstNonEmpty(f.Title, title)
					break
				}
			}
			rows = append(rows, nodeDependencyPlugin{
				Name:  file.Name(),
				Title: title,
				File:  filepath.Base(index),
				Path:  dir,
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
	if dir, err := nodePluginDir(name); err == nil {
		return nodeDependencyPlugin{Name: name, Title: name, File: "main.js", Path: dir}, nil
	}
	return nodeDependencyPlugin{}, errors.New("NodeJS 脚本插件不存在")
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func nodePluginsRoot() string {
	return filepath.Clean(filepath.Join(utils.GetDataHome(), "plugins"))
}

func nodePluginNameFromPath(path string) string {
	if path == "" {
		return ""
	}
	dir := filepath.Dir(filepath.Clean(path))
	if filepath.Base(path) == "main.js" {
		return filepath.Base(dir)
	}
	return ""
}

func nodePluginDir(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("请选择 NodeJS 脚本插件")
	}
	if strings.ContainsAny(name, `/\:`) || strings.Contains(name, "..") {
		return "", errors.New("插件名称不合法")
	}
	root := nodePluginsRoot()
	dir := filepath.Clean(filepath.Join(root, name))
	rel, err := filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", errors.New("插件目录不合法")
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", errors.New("NodeJS 脚本插件不存在")
	}
	if index, class := FindMainIndex(strings.ReplaceAll(dir, "\\", "/")); index == "" || class != NODE {
		return "", errors.New("该插件不是 NodeJS 脚本插件")
	}
	return dir, nil
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
	if filepath.Base(clean) != "main.js" {
		return "", errors.New("只允许编辑 NodeJS 插件入口 main.js")
	}
	if _, err := checkedNodePluginDir(filepath.Dir(clean)); err != nil {
		return "", err
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
		if _, err := os.Stat(filepath.Join(root, name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}

func createNodePlugin(pluginName, title string) (string, error) {
	root := nodePluginsRoot()
	dir := filepath.Join(root, pluginName)
	if _, err := checkedNodePluginDir(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "sillygirl.d.ts"), []byte(typeat), 0644); err != nil {
		return "", err
	}
	if err := ensureNodePackageJSON(dir, pluginName); err != nil {
		return "", err
	}
	content := strings.TrimRight(defaultScript(title), "\n") + `

async function main() {
  await s.reply("pong");
}

main();
`
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(content), 0644); err != nil {
		return "", err
	}
	return dir, nil
}

func readNodeDependencies(plugin nodeDependencyPlugin) ([]nodeDependencyRow, error) {
	manifest := nodeDependencyManifest{}
	dir := plugin.Path
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
	for _, name := range nodePluginRequiredDependencies(dir) {
		if _, ok := rowsByName[name]; !ok {
			rowsByName[name] = nodeDependencyRow{Name: name, Version: "", Installed: false, Source: source}
		}
	}
	pluginName := filepath.Base(dir)
	for _, name := range nodePluginIndexDependencies(pluginName) {
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

func nodePluginRequiredDependencies(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, "main.js"))
	if err != nil {
		return nil
	}
	return parseNodeRequires(string(data))
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
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	manifest := nodeDependencyManifest{
		Name:         safePackageName(pluginName),
		Version:      "1.0.0",
		Private:      true,
		Dependencies: map[string]string{},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
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
	if err := ensureNodePackageJSON(dir, pluginName); err != nil {
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

func runPnpm(dir string, args ...string) (string, error) {
	pnpm, err := resolvePnpmCommand()
	if err != nil {
		return "", err
	}
	cmdArgs := append([]string{}, pnpm.Args...)
	cmdArgs = append(cmdArgs, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pnpm.Bin, cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+nodePathWithRuntime())
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
	nodeDir := filepath.Join(utils.ExecPath, "language", "node")
	for _, name := range []string{"pnpm", "pnpm.cmd", "pnpm.exe"} {
		path := filepath.Join(nodeDir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return pnpmCommand{Bin: path}, nil
		}
	}
	for _, name := range []string{"corepack", "corepack.cmd", "corepack.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return pnpmCommand{Bin: path, Args: []string{"pnpm"}}, nil
		}
		path := filepath.Join(nodeDir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return pnpmCommand{Bin: path, Args: []string{"pnpm"}}, nil
		}
	}
	return pnpmCommand{}, errors.New("未找到 pnpm，请先安装 pnpm 或启用 Node.js corepack")
}

func nodePathWithRuntime() string {
	nodeDir := filepath.Join(utils.ExecPath, "language", "node")
	path := os.Getenv("PATH")
	if path == "" {
		return nodeDir
	}
	return nodeDir + string(os.PathListSeparator) + path
}
