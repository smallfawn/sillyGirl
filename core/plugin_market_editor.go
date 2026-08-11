package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
)

type marketPluginScriptRequest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type marketPluginStatusRequest struct {
	Status *bool `json:"status"`
}

var (
	pluginPublicMetaPattern       = regexp.MustCompile(`(?im)^([ \t]*(?:\*[ \t]*)?@public[ \t]+)(true|false|1|yes|on)[ \t]*$`)
	pluginLegacyPublicMetaPattern = regexp.MustCompile(`(?im)^([ \t]*(?://|#+)[ \t]*\[[ \t]*public[ \t]*:[ \t]*)(true|false|1|yes|on)([ \t]*\][^\r\n]*)$`)
	pluginLegacyMetaLinePattern   = regexp.MustCompile(`^[ \t]*(?://|#+)[ \t]*\[[ \t]*[\d\w+-]+[ \t]*:`)
)

func initMarketPluginEditor() {
	GinApi(GET, "/api/admin/local-plugins/:id", RequireAuth, getMarketPluginScript)
	GinApi(GET, "/api/admin/local-plugins/:id/dependents", RequireAuth, getMarketPluginDependents)
	GinApi(GET, "/api/admin/local-plugins/:id/dependency-plan", RequireAuth, getInstalledPluginDependencyPlan)
	GinApi(POST, "/api/admin/local-plugins", RequireAuth, createMarketPluginScript)
	GinApi(POST, "/api/admin/local-plugins/:id", RequireAuth, saveMarketPluginScript)
	GinApi(POST, "/api/admin/local-plugins/:id/status", RequireAuth, setMarketPluginStatus)
	GinApi(POST, "/api/admin/local-plugins/:id/deletions", RequireAuth, deleteMarketPluginScript)
}

func getInstalledPluginDependencyPlan(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	plugin := installedPluginByUUID(id)
	if plugin == nil || strings.TrimSpace(plugin.Path) == "" {
		ApiNotFound(ctx, "已下载插件不存在")
		return
	}
	runtime := normalizeDependencyRuntime(plugin.Type)
	pluginName := nodePluginIdentityFromPath(plugin.Path)
	dependencyPlugins := listDependencyPlugins(runtime)
	dependencyPlugin, err := dependencyPluginByName(dependencyPlugins, pluginName, runtime)
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	dependencies, err := readPluginDependencies(runtime, dependencyPlugin)
	if err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	missingPackages := make([]nodeDependencyRow, 0)
	for _, dependency := range dependencies {
		if !dependency.Installed {
			missingPackages = append(missingPackages, dependency)
		}
	}
	tool := pnpmDependencyStatus()
	if runtime == PYTHON {
		tool = pipxDependencyStatus()
	}
	ApiOK(ctx, map[string]interface{}{
		"runtime":             runtime,
		"plugin":              pluginName,
		"plugin_title":        firstNonEmpty(plugin.Title, nodePluginNameFromPath(plugin.Path), pluginName),
		"dependencies":        missingPackages,
		"module_dependencies": missingPluginModuleDependencies(plugin, installedPluginSnapshot()),
		"tool":                tool,
	})
}

func setMarketPluginStatus(ctx *gin.Context) {
	req := marketPluginStatusRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	if req.Status == nil {
		ApiUnprocessable(ctx, "插件 status 必须是布尔值")
		return
	}
	id := strings.TrimSpace(ctx.Param("id"))
	f, err := nodeFunctionByID(id)
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	if f.Module {
		ApiConflict(ctx, "依赖模块没有独立运行状态")
		return
	}
	if _, err := checkedNodeScriptPath(f.Path); err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	if err := updatePluginStatusAnnotation(f, *req.Status); err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	ApiOK(ctx, gin.H{"id": f.UUID, "status": *req.Status})
}

func getMarketPluginDependents(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	f, err := nodeFunctionByID(id)
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	dependents, err := pluginModuleDependents(f, installedPluginSnapshot())
	if err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	ApiOK(ctx, dependents)
}

func getMarketPluginScript(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		ApiUnprocessable(ctx, "缺少插件 ID")
		return
	}
	if f, err := nodeFunctionByID(id); err == nil {
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiOK(ctx, map[string]interface{}{
			"id":        f.UUID,
			"title":     f.Title,
			"name":      nodePluginNameFromPath(path),
			"type":      f.Type,
			"path":      path,
			"installed": true,
			"editable":  true,
			"content":   string(data),
		})
		return
	}
	f := marketPluginByID(id)
	if f == nil {
		ApiNotFound(ctx, "插件市场未找到该插件")
		return
	}
	content, class, err := readMarketPluginRemoteContent(f)
	if err != nil {
		ApiBadGateway(ctx, err.Error())
		return
	}
	ApiOK(ctx, map[string]interface{}{
		"id":        f.UUID,
		"title":     f.Title,
		"name":      availablePluginName(filepath.Join(nodePluginsRoot(), "local"), firstNonEmpty(f.Title, strings.TrimSuffix(filepath.Base(f.Address), filepath.Ext(f.Address)), "plugin")),
		"type":      class,
		"installed": false,
		"editable":  true,
		"content":   content,
	})
}

func createMarketPluginScript(ctx *gin.Context) {
	req := marketPluginScriptRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	content, err := validateAndNormalizeLocalPluginContent(req.Content)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	class := normalizeMarketPluginScriptType(req.Type, req.Name, content)
	if err := validateLocalPluginRequestName(req.Name, content, class); err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	f, path, err := writeNewLocalMarketPlugin(req.Name, class, content)
	if err != nil {
		if strings.Contains(err.Error(), "存在") {
			ApiConflict(ctx, err.Error())
		} else {
			ApiInternalError(ctx, err.Error())
		}
		return
	}
	ApiCreated(ctx, "/api/admin/local-plugins/"+f.UUID, map[string]interface{}{"id": f.UUID, "title": f.Title, "type": f.Type, "path": path})
}

func saveMarketPluginScript(ctx *gin.Context) {
	req := marketPluginScriptRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	req.ID = strings.TrimSpace(ctx.Param("id"))
	if f, err := nodeFunctionByID(req.ID); err == nil {
		content, err := validateExistingPluginContent(req.Content)
		if err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		if err := validateLocalPluginRequestName(nodePluginNameFromPath(path), content, f.Type); err != nil {
			ApiUnprocessable(ctx, err.Error())
			return
		}
		refreshed, err := replaceLoadedPluginSource(path, []byte(content), nodePluginIdentityFromPath(path), f.Type)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiOK(ctx, map[string]interface{}{"id": refreshed.UUID, "title": refreshed.Title, "type": refreshed.Type, "path": path})
		return
	}
	ApiNotFound(ctx, "本地插件不存在")
}

func replaceLoadedPluginSource(path string, content []byte, identity, class string) (*common.Function, error) {
	pluginLock.Lock()
	defer pluginLock.Unlock()
	previous, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return nil, err
	}
	if err := addNodePluginLocked(strings.ReplaceAll(path, "\\", "/"), identity, class); err != nil {
		if restoreErr := os.WriteFile(path, previous, 0644); restoreErr != nil {
			return nil, fmt.Errorf("%w；恢复原插件源码失败：%v", err, restoreErr)
		}
		if restoreErr := addNodePluginLocked(strings.ReplaceAll(path, "\\", "/"), identity, class); restoreErr != nil {
			return nil, fmt.Errorf("%w；恢复原插件运行状态失败：%v", err, restoreErr)
		}
		return nil, err
	}
	loaded := loadedNodePluginLocked(nameUuid(identity))
	if loaded == nil {
		return nil, errors.New("插件源码已写入，但重载后未找到插件")
	}
	return loaded, nil
}

func deleteMarketPluginScript(ctx *gin.Context) {
	req := marketPluginScriptRequest{ID: strings.TrimSpace(ctx.Param("id"))}
	f, err := nodeFunctionByID(req.ID)
	if err != nil {
		ApiNotFound(ctx, err.Error())
		return
	}
	if err := ensurePluginModuleUnused(f, installedPluginSnapshot()); err != nil {
		ApiConflict(ctx, err.Error())
		return
	}
	path, err := checkedNodeScriptPath(f.Path)
	if err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	if err := removeNodePluginScript(path); err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	AddNodePlugin(strings.ReplaceAll(path, "\\", "/"), nodePluginIdentityFromPath(path), UNKNOWN)
	ApiOK(ctx, nil)
}

func marketPluginByID(id string) *common.Function {
	for _, f := range pluginMarketItemsSnapshot() {
		if f != nil && f.UUID == id {
			return f
		}
	}
	initPluginList()
	for _, f := range pluginMarketItemsSnapshot() {
		if f != nil && f.UUID == id {
			return f
		}
	}
	return nil
}

func readMarketPluginRemoteContent(f *common.Function) (string, string, error) {
	if f == nil || !strings.HasPrefix(f.Address, githubNodePluginScheme+"://") {
		return "", "", errors.New("该插件没有可编辑的脚本地址")
	}
	source, pluginPath, rawURL, class, err := parseGithubNodePluginAddress(f.Address)
	if err != nil {
		return "", "", err
	}
	if rawURL == "" {
		rawURL = githubRawURL(source.Owner, source.Repo, source.Branch, pluginPath)
	}
	data, err := httpGetBytes(rawURL, 30*time.Second)
	if err != nil {
		return "", "", err
	}
	if class == "" || class == UNKNOWN {
		class = pluginClassFromExt(filepath.Ext(pluginPath))
	}
	return string(data), class, nil
}

func validateAndNormalizeLocalPluginContent(content string) (string, error) {
	return validatePluginContent(content, true)
}

func validateExistingPluginContent(content string) (string, error) {
	return validatePluginContent(content, false)
}

func validatePluginContent(content string, forcePrivate bool) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("插件源码不能为空")
	}
	meta := pluginMetaMap(content)
	missing := []string{}
	for _, key := range []string{"title", "name", "desc", "version"} {
		if strings.TrimSpace(meta[key]) == "" {
			missing = append(missing, map[string]string{"title": "[title: xxx]", "name": "[name: 文件名]", "desc": "[desc: xxx]", "version": "[version: vx.y.z]"}[key])
		}
	}
	if strings.TrimSpace(meta["rule"]) == "" && strings.TrimSpace(meta["cron"]) == "" && !parsePluginBool(meta["on_start"]) && !parsePluginBool(meta["web"]) && !parsePluginBool(meta["module"]) {
		missing = append(missing, "[rule: xxx] 或 [cron: xxx]/[on_start: true]/[web: true]/[module: true]")
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("插件注释缺少必须字段：%s", strings.Join(missing, "、"))
	}
	if _, err := normalizePluginMetaFileName(meta["name"], normalizeMarketPluginScriptType("", "", content)); err != nil {
		return "", err
	}
	if forcePrivate {
		content = forceLocalPluginPrivate(content)
	}
	return content, nil
}

func pluginMetaMap(content string) map[string]string {
	out := map[string]string{}
	for _, match := range pluginMetaEntries(content) {
		if len(match) >= 3 {
			out[normalizePluginMetaKey(match[1])] = strings.TrimSpace(match[2])
		}
	}
	return out
}

func normalizePluginMetaFileName(value string, class string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.New("插件注释 [name: 文件名] 不能为空")
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	fileName, err := normalizeNodeScriptFileName(name + dependencyRuntimeSuffix(class))
	if err != nil {
		return "", fmt.Errorf("插件注释 [name: %s] 不是有效文件名：%v", value, err)
	}
	pluginName := safePluginDirName(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	if strings.TrimSpace(pluginName) == "" {
		return "", fmt.Errorf("插件注释 [name: %s] 不是有效文件名", value)
	}
	return pluginName + filepath.Ext(fileName), nil
}

func validateLocalPluginRequestName(requestName, content, class string) error {
	metaName := strings.TrimSpace(pluginMetaMap(content)["name"])
	if strings.TrimSpace(requestName) == "" {
		return errors.New("插件名称不能为空，且必须和 [name: 文件名] 一致")
	}
	if metaName == "" {
		return errors.New("插件源码缺少 [name: 文件名]")
	}
	requestFile, err := normalizePluginMetaFileName(requestName, class)
	if err != nil {
		return err
	}
	metaFile, err := normalizePluginMetaFileName(metaName, class)
	if err != nil {
		return err
	}
	requestBase := strings.TrimSuffix(requestFile, filepath.Ext(requestFile))
	metaBase := strings.TrimSuffix(metaFile, filepath.Ext(metaFile))
	if requestBase != metaBase {
		return fmt.Errorf("插件名称必须和 [name: %s] 一致，当前填写：%s", metaBase, requestBase)
	}
	return nil
}

func forceLocalPluginPrivate(content string) string {
	if pluginPublicMetaPattern.MatchString(content) {
		return pluginPublicMetaPattern.ReplaceAllString(content, "${1}false")
	}
	if pluginLegacyPublicMetaPattern.MatchString(content) {
		return pluginLegacyPublicMetaPattern.ReplaceAllString(content, "${1}false${3}")
	}
	lines := strings.Split(content, "\n")
	lastLegacyMetaLine := -1
	legacyPrefix := ""
	for i, line := range lines {
		if strings.TrimSpace(line) == "" && lastLegacyMetaLine < 0 {
			continue
		}
		if pluginLegacyMetaLinePattern.MatchString(line) {
			lastLegacyMetaLine = i
			if strings.Contains(strings.TrimSpace(line), "//") && strings.HasPrefix(strings.TrimSpace(line), "//") {
				legacyPrefix = "//"
			} else {
				legacyPrefix = "#"
			}
			continue
		}
		if lastLegacyMetaLine >= 0 {
			break
		}
		break
	}
	if lastLegacyMetaLine >= 0 {
		insert := legacyPrefix + "[public: false]"
		lines = append(lines[:lastLegacyMetaLine+1], append([]string{insert}, lines[lastLegacyMetaLine+1:]...)...)
		return strings.Join(lines, "\n")
	}
	seenPythonDocStart := false
	for i, line := range lines {
		if strings.Contains(line, `"""`) {
			if !seenPythonDocStart {
				seenPythonDocStart = true
				continue
			}
			lines = append(lines[:i], append([]string{"@public false"}, lines[i:]...)...)
			return strings.Join(lines, "\n")
		}
		if strings.Contains(line, "*/") {
			lines = append(lines[:i], append([]string{" * @public false"}, lines[i:]...)...)
			return strings.Join(lines, "\n")
		}
	}
	return "/*\n * @public false\n */\n" + content
}

func normalizeMarketPluginScriptType(value, name, content string) string {
	value = normalizeDependencyRuntime(value)
	if value == PYTHON || strings.EqualFold(filepath.Ext(name), ".py") {
		return PYTHON
	}
	if strings.Contains(content, "from sillygirl import") || strings.Contains(content, "import sillygirl") {
		return PYTHON
	}
	return NODE
}

func writeNewLocalMarketPlugin(name, class, content string) (*common.Function, string, error) {
	meta := pluginMetaMap(content)
	title := firstNonEmpty(meta["title"], strings.TrimSpace(name), "本地插件")
	fileName, err := normalizePluginMetaFileName(firstNonEmpty(meta["name"], strings.TrimSpace(name), title), class)
	if err != nil {
		return nil, "", err
	}
	root := nodePluginsRoot()
	target := filepath.Join(root, "local")
	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, "", err
	}
	if class == NODE {
		if err := ensureNodeSillygirlModule(root); err != nil {
			return nil, "", err
		}
		if err := ensureNodePackageJSON(root, "sillygirl-plugins"); err != nil {
			return nil, "", err
		}
	} else if _, err := ensurePythonSillygirlModule(); err != nil {
		return nil, "", err
	}
	index := filepath.Join(target, fileName)
	if _, err := checkedNodeScriptPath(index); err != nil {
		return nil, "", err
	}
	if err := ensurePluginBaseAvailable(target, fileName); err != nil {
		return nil, "", err
	}
	file, err := os.OpenFile(index, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, "", fmt.Errorf("插件文件已存在：%s", filepath.Base(index))
		}
		return nil, "", err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(index)
		return nil, "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(index)
		return nil, "", err
	}
	identity := nodePluginIdentityFromPath(index)
	if err := AddNodePlugin(strings.ReplaceAll(index, "\\", "/"), identity, class); err != nil {
		_ = os.Remove(index)
		return nil, "", err
	}
	f, err := nodeFunctionByID(nameUuid(identity))
	if err != nil {
		return nil, "", err
	}
	return f, index, nil
}
