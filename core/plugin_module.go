package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

const maxPluginDependencyScanBytes int64 = 4 << 20

type pluginModuleDependent struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

func installMarketPluginWithModuleDependencies(root *common.Function, market, installed []*common.Function, install func(string) error, dependencyInstallers ...func(string, string) error) error {
	if root == nil {
		return fmt.Errorf("待安装插件不存在")
	}
	installedIDs := map[string]bool{}
	for _, plugin := range installed {
		if plugin != nil {
			installedIDs[plugin.UUID] = true
		}
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var installDependency func(string, string) error
	if len(dependencyInstallers) != 0 {
		installDependency = dependencyInstallers[0]
	}
	var walk func(*common.Function, bool) error
	walk = func(plugin *common.Function, force bool) error {
		key := firstNonEmpty(plugin.UUID, strings.ToLower(plugin.Type+":"+marketPluginFileName(plugin)))
		if visiting[key] {
			return fmt.Errorf("插件模块依赖存在循环：%s", firstNonEmpty(plugin.Title, marketPluginFileName(plugin), key))
		}
		if visited[key] || (!force && installedIDs[plugin.UUID]) {
			return nil
		}
		visiting[key] = true
		for _, dependency := range plugin.ModuleDependencies {
			publisher := pluginDependencyPublisher(plugin)
			if module, found, err := lookupModuleDependency(dependency, plugin.Type, publisher, installed); err != nil {
				delete(visiting, key)
				return fmt.Errorf("%s：%w", firstNonEmpty(plugin.Title, plugin.UUID), err)
			} else if found && module != nil {
				continue
			}
			module, err := findMarketModuleDependency(dependency, plugin.Type, publisher, market)
			if err != nil {
				delete(visiting, key)
				return fmt.Errorf("%s：%w", firstNonEmpty(plugin.Title, plugin.UUID), err)
			}
			if err := walk(module, false); err != nil {
				delete(visiting, key)
				return err
			}
		}
		delete(visiting, key)
		visited[key] = true
		if installDependency != nil {
			for _, dependency := range plugin.Dependencies {
				if err := installDependency(plugin.Type, dependency); err != nil {
					return fmt.Errorf("安装 %s 的运行依赖 %s 失败：%w", firstNonEmpty(plugin.Title, plugin.UUID), dependency, err)
				}
			}
		}
		if strings.TrimSpace(plugin.Address) == "" {
			return fmt.Errorf("插件 %s 缺少下载地址", firstNonEmpty(plugin.Title, plugin.UUID))
		}
		if err := install(plugin.Address); err != nil {
			return fmt.Errorf("安装 %s 失败：%w", firstNonEmpty(plugin.Title, plugin.UUID), err)
		}
		installedIDs[plugin.UUID] = true
		return nil
	}
	return walk(root, true)
}

func installMarketPluginRuntimeDependency(runtime, dependency string) error {
	if normalizeDependencyRuntime(runtime) == PYTHON {
		_, err := installPythonDependency("__shared__", dependency)
		return err
	}
	_, err := installNodeDependency("__shared__", dependency, false)
	return err
}

func findMarketModuleDependency(dependency, runtime, publisher string, market []*common.Function) (*common.Function, error) {
	module, found, err := lookupModuleDependency(dependency, runtime, publisher, market)
	if err != nil {
		return nil, err
	}
	if found {
		return module, nil
	}
	wanted, _ := normalizePluginModuleDependency(dependency, runtime)
	return nil, fmt.Errorf("插件市场缺少依赖模块 %s", wanted)
}

func lookupModuleDependency(dependency, runtime, publisher string, plugins []*common.Function) (*common.Function, bool, error) {
	wanted, ok := normalizePluginModuleDependency(dependency, runtime)
	if !ok {
		return nil, false, fmt.Errorf("模块依赖格式错误：%s", dependency)
	}
	var nonModule *common.Function
	for _, candidate := range plugins {
		if candidate == nil || !strings.EqualFold(candidate.Type, runtime) {
			continue
		}
		if publisher != "" && !strings.EqualFold(pluginDependencyPublisher(candidate), publisher) {
			continue
		}
		if !strings.EqualFold(marketPluginFileName(candidate), wanted) {
			continue
		}
		if candidate.Module {
			return candidate, true, nil
		}
		nonModule = candidate
	}
	if nonModule != nil {
		return nil, true, fmt.Errorf("%s 未声明 [module:true]", wanted)
	}
	return nil, false, nil
}

func pluginDependencyPublisher(plugin *common.Function) string {
	if plugin == nil {
		return ""
	}
	if strings.HasPrefix(plugin.Address, githubNodePluginScheme+"://") {
		if source, _, _, _, err := parseGithubNodePluginAddress(plugin.Address); err == nil {
			return pluginPublisherDirName(source.Owner)
		}
	}
	if plugin.Path != "" {
		return nodePluginPublisherFromPath(plugin.Path)
	}
	if organization := strings.Trim(strings.TrimSpace(plugin.Organization), "/"); organization != "" {
		return pluginPublisherDirName(strings.Split(organization, "/")[0])
	}
	return ""
}

func marketPluginFileName(plugin *common.Function) string {
	if plugin == nil {
		return ""
	}
	if strings.HasPrefix(plugin.Address, githubNodePluginScheme+"://") {
		if _, pluginPath, _, _, err := parseGithubNodePluginAddress(plugin.Address); err == nil {
			return "./" + path.Base(pluginPath)
		}
	}
	if plugin.Path != "" {
		return "./" + filepath.Base(plugin.Path)
	}
	if plugin.Suffix != "" && plugin.Title != "" {
		return "./" + plugin.Title + plugin.Suffix
	}
	return ""
}

func pluginModuleDependents(module *common.Function, installed []*common.Function) ([]pluginModuleDependent, error) {
	if module == nil || !module.Module {
		return []pluginModuleDependent{}, nil
	}
	target, ok := normalizePluginModuleDependency("./"+filepath.Base(module.Path), module.Type)
	if !ok {
		return []pluginModuleDependent{}, nil
	}
	result := make([]pluginModuleDependent, 0)
	for _, plugin := range installed {
		if plugin == nil || plugin.UUID == module.UUID || strings.TrimSpace(plugin.Path) == "" {
			continue
		}
		if !strings.EqualFold(nodePluginPublisherFromPath(plugin.Path), nodePluginPublisherFromPath(module.Path)) {
			continue
		}
		file, err := os.Open(plugin.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("检查插件 %s 的模块依赖失败：%w", firstNonEmpty(plugin.Title, plugin.UUID), err)
		}
		data, readErr := utils.ReadAllLimit(file, maxPluginDependencyScanBytes)
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("检查插件 %s 的模块依赖失败：%w", firstNonEmpty(plugin.Title, plugin.UUID), readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("关闭插件 %s 源码失败：%w", firstNonEmpty(plugin.Title, plugin.UUID), closeErr)
		}
		dependsOnModule := false
		for _, dependency := range parseDeclaredModuleDependencies(string(data), plugin.Type) {
			if strings.EqualFold(dependency, target) {
				dependsOnModule = true
				break
			}
		}
		if dependsOnModule {
			result = append(result, pluginModuleDependent{
				ID:    plugin.UUID,
				Title: firstNonEmpty(plugin.Title, strings.TrimSuffix(filepath.Base(plugin.Path), filepath.Ext(plugin.Path)), plugin.UUID),
				Type:  plugin.Type,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Title < result[j].Title })
	return result, nil
}

func ensurePluginModuleUnused(module *common.Function, installed []*common.Function) error {
	dependents, err := pluginModuleDependents(module, installed)
	if err != nil {
		return err
	}
	if len(dependents) == 0 {
		return nil
	}
	names := make([]string, 0, len(dependents))
	for _, dependent := range dependents {
		names = append(names, dependent.Title)
	}
	return fmt.Errorf("依赖模块正在被以下插件使用：%s；请先处理依赖插件", strings.Join(names, "、"))
}
