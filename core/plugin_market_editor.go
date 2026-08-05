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

var (
	pluginPublicMetaPattern       = regexp.MustCompile(`(?im)^([ \t]*(?:\*[ \t]*)?@public[ \t]+)(true|false|1|yes|on)[ \t]*$`)
	pluginLegacyPublicMetaPattern = regexp.MustCompile(`(?im)^([ \t]*(?://|#+)[ \t]*\[[ \t]*public[ \t]*:[ \t]*)(true|false|1|yes|on)([ \t]*\][^\r\n]*)$`)
	pluginLegacyMetaLinePattern   = regexp.MustCompile(`^[ \t]*(?://|#+)[ \t]*\[[ \t]*[\d\w+-]+[ \t]*:`)
)

func initMarketPluginEditor() {
	GinApi(GET, "/api/admin/plugins/local/script", RequireAuth, getMarketPluginScript)
	GinApi(POST, "/api/admin/plugins/local/script", RequireAuth, createMarketPluginScript)
	GinApi(PUT, "/api/admin/plugins/local/script", RequireAuth, saveMarketPluginScript)
	GinApi(DELETE, "/api/admin/plugins/local/script", RequireAuth, deleteMarketPluginScript)
}

func getMarketPluginScript(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Query("id"))
	if id == "" {
		ApiFail(ctx, "缺少插件 ID")
		return
	}
	if f, err := nodeFunctionByID(id); err == nil {
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			ApiFail(ctx, err.Error())
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
		ApiFail(ctx, "插件市场未找到该插件")
		return
	}
	content, class, err := readMarketPluginRemoteContent(f)
	if err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	ApiOK(ctx, map[string]interface{}{
		"id":        f.UUID,
		"title":     f.Title,
		"name":      safePluginDirName(firstNonEmpty(f.Title, strings.TrimSuffix(filepath.Base(f.Address), filepath.Ext(f.Address)), "plugin")),
		"type":      class,
		"installed": false,
		"editable":  true,
		"content":   content,
	})
}

func createMarketPluginScript(ctx *gin.Context) {
	req := marketPluginScriptRequest{}
	if err := ctx.BindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	content, err := validateAndNormalizeLocalPluginContent(req.Content)
	if err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	class := normalizeMarketPluginScriptType(req.Type, req.Name, content)
	if err := validateLocalPluginRequestName(req.Name, content, class); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	f, path, err := writeNewLocalMarketPlugin(req.Name, class, content)
	if err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	ApiOK(ctx, map[string]interface{}{"id": f.UUID, "title": f.Title, "type": f.Type, "path": path})
}

func saveMarketPluginScript(ctx *gin.Context) {
	req := marketPluginScriptRequest{}
	if err := ctx.BindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	content, err := validateAndNormalizeLocalPluginContent(req.Content)
	if err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	if f, err := nodeFunctionByID(req.ID); err == nil {
		path, err := checkedNodeScriptPath(f.Path)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		if err := AddNodePlugin(strings.ReplaceAll(path, "\\", "/"), nodePluginNameFromPath(path), f.Type); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, map[string]interface{}{"id": f.UUID, "title": f.Title, "type": f.Type, "path": path})
		return
	}
	class := normalizeMarketPluginScriptType(req.Type, req.Name, content)
	if err := validateLocalPluginRequestName(req.Name, content, class); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	f, path, err := writeNewLocalMarketPlugin(req.Name, class, content)
	if err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	ApiOK(ctx, map[string]interface{}{"id": f.UUID, "title": f.Title, "type": f.Type, "path": path})
}

func deleteMarketPluginScript(ctx *gin.Context) {
	req := marketPluginScriptRequest{}
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
}

func marketPluginByID(id string) *common.Function {
	for _, f := range plugin_list {
		if f != nil && f.UUID == id {
			return f
		}
	}
	initPluginList()
	for _, f := range plugin_list {
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
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("插件源码不能为空")
	}
	meta := pluginMetaMap(content)
	missing := []string{}
	for _, key := range []string{"title", "name", "desc", "version"} {
		if strings.TrimSpace(meta[key]) == "" {
			missing = append(missing, map[string]string{"title": "[title: xxx]", "name": "[name: 文件名]", "desc": "[description: xxx]", "version": "[version: vx.y.z]"}[key])
		}
	}
	if strings.TrimSpace(meta["rule"]) == "" && strings.TrimSpace(meta["cron"]) == "" && strings.TrimSpace(meta["on_start"]) != "true" && strings.TrimSpace(meta["web"]) != "true" {
		missing = append(missing, "[rule: xxx] 或 [cron: xxx]/[on_start: true]/[web: true]")
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("插件注释缺少必须字段：%s", strings.Join(missing, "、"))
	}
	if _, err := normalizePluginMetaFileName(meta["name"], normalizeMarketPluginScriptType("", "", content)); err != nil {
		return "", err
	}
	return forceLocalPluginPrivate(content), nil
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
	if strings.TrimSpace(requestName) == "" || metaName == "" {
		return nil
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
	pluginName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	root := nodePluginsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
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
	index := filepath.Join(root, fileName)
	if _, err := checkedNodeScriptPath(index); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(index, []byte(content), 0644); err != nil {
		return nil, "", err
	}
	if err := AddNodePlugin(strings.ReplaceAll(index, "\\", "/"), pluginName, class); err != nil {
		return nil, "", err
	}
	f, err := nodeFunctionByID(nameUuid(pluginName))
	if err != nil {
		return nil, "", err
	}
	return f, index, nil
}
