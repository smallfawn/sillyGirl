package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

var pluginLock = new(sync.Mutex)

var mutexMap = make(map[string]*sync.Mutex)
var mutexMapMutex sync.Mutex

func GetMutex(uuid string) *sync.Mutex {
	mutexMapMutex.Lock()
	defer mutexMapMutex.Unlock()

	if mutex, ok := mutexMap[uuid]; ok {
		return mutex
	}

	mutex := &sync.Mutex{}
	mutexMap[uuid] = mutex
	return mutex
}

var RegistFuncs = map[string]interface{}{}

var plugins = MakeBucket("plugins")

type Route struct {
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	Type      string  `json:"type,omitempty"`
	File      string  `json:"file,omitempty"`
	Plugin    string  `json:"plugin,omitempty"`
	Component string  `json:"component,omitempty"`
	Routes    []Route `json:"routes,omitempty"`
	// Key       string  `json:"key,omitempty"`
	CreateAt string `json:"create_at"`
}

func CancelPluginlistening(uuid string) {
	// logs.Info(`k, c.Function, c.Function.Rules`)
	for _, wait := range waits {
		wait.Foreach(func(k int64, c *Carry) bool {
			if uuid == c.UUID {
				c.Chan <- errors.New("uinstall")
			}
			return true
		})
	}
}

var debug = sillyGirl.GetBool("debug", false)

func initPlugins() {
	storage.Watch(sillyGirl, "debug", func(old, new, key string) *storage.Final {
		debug = new == "true"
		return nil
	})

	storage.Watch(plugins, nil, func(old, new, key string) (fin *storage.Final) {
		if !isNameUuid(key) {
			if new == "" || new == "uninstall" {
				return &storage.Final{Now: storage.EMPTY}
			}
			return &storage.Final{
				Now:   storage.EMPTY,
				Error: errors.New("旧内嵌 JS 插件数据已不支持，请使用 /data/plugins/<发布者>/*.js 的 NodeJS 插件"),
			}
		}
		if new == "install" || new == "download" || new == "install-dependencies" {
			var marketPlugin *common.Function
			marketItems := pluginMarketItemsSnapshot()
			for _, p := range marketItems {
				if p.UUID == key {
					marketPlugin = p
					break
				}
			}
			if marketPlugin == nil {
				return &storage.Final{
					Error: errors.New("插件市场未找到该插件，请刷新插件列表后重试"),
				}
			}
			if !strings.HasPrefix(marketPlugin.Address, githubNodePluginScheme+"://") {
				return &storage.Final{
					Now:   storage.EMPTY,
					Error: errors.New("旧插件源已不支持，请导入 GitHub NodeJS 插件源"),
				}
			}
			var err error
			switch new {
			case "download":
				err = installGithubNodePlugin(marketPlugin.Address)
			case "install-dependencies":
				installed := installedPluginSnapshot()
				dependencyRoot := downloadedPluginDependencyRoot(marketPlugin, installed)
				err = installMarketPluginDependencies(dependencyRoot, marketItems, installed, installGithubNodePlugin, installMarketPluginRuntimeDependency)
			default:
				err = installMarketPluginWithModuleDependencies(marketPlugin, marketItems, installedPluginSnapshot(), installGithubNodePlugin, installMarketPluginRuntimeDependency)
			}
			if err != nil {
				return &storage.Final{
					Error: errors.New("安装异常！" + err.Error()),
				}
			}
			return &storage.Final{
				Now:     storage.EMPTY,
				Message: fmt.Sprintf("已处理 %s", marketPlugin.Title),
			}
		}
		pluginLock.Lock()
		defer pluginLock.Unlock()
		requestedUninstall := new == "uninstall"
		if new == "uninstall" {
			new = ""
			fin = &storage.Final{
				Now: storage.EMPTY,
			}

		}
		for i := range Functions {
			if Functions[i].UUID == key {
				current := Functions[i]
				if requestedUninstall {
					if err := ensurePluginModuleUnused(current, Functions); err != nil {
						return &storage.Final{Now: storage.EMPTY, Error: err}
					}
				}
				DestroyAdapterByUUID(key)
				current.Running = false
				if len(current.CronIds) != 0 {
					for _, id := range current.CronIds {
						CRON.Remove(cron.EntryID(id))
					}
				}
				Functions = append(Functions[:i], Functions[i+1:]...)
				CancelPluginlistening(key)
				storage.DisableHandle(key)
				if new == "reload" {
					go current.Reload()
				} else if new == "" {
					filename := current.Path
					processes.Range(func(key, value any) bool {
						p := key.(*exec.Cmd)
						s := value.(common.Sender)
						if s.GetPluginID() == current.UUID {
							console.Log("已终止 %s", current.Title)
							func() {
								defer func() {
									recover()
								}()
								if p.Process.Kill() == nil {
									processes.Delete(key)
								}
							}()
						}
						return true
					})
					if filename != "" {
						if err := removeNodePluginFiles(filename); err != nil {
							console.Warn("卸载插件文件失败 %s: %v", filename, err)
						}
					}
					console.Log("已卸载 %s%s", current.Title, current.Suffix)
				}
				break
			}
		}
		if fin != nil {
			return fin
		}
		return &storage.Final{Now: storage.EMPTY}
	})
}

func removeNodePluginFiles(filename string) error {
	filename = filepath.Clean(strings.TrimSpace(filename))
	if filename == "" {
		return nil
	}
	checked, checkedErr := checkedNodeScriptPath(filename)
	if checkedErr != nil {
		return checkedErr
	}
	filename = checked
	root := nodePluginsRoot()
	rel, err := filepath.Rel(root, filename)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("插件文件路径不合法")
	}
	if isSupportedScriptExt(filepath.Ext(filename)) {
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(filename)
		if !strings.EqualFold(parent, root) {
			_ = os.Remove(parent)
		}
		return nil
	}
	return errors.New("插件入口文件类型不合法")
}

func GetFunctionByUUID(uuid string) *common.Function {
	for _, f := range installedPluginSnapshot() {
		if f.UUID == uuid {
			return f
		}
	}
	return nil
}

func ChatID(p interface{}) string {
	switch p := p.(type) {
	case int:
		if p == 0 {
			return ""
		} else {
			return utils.Itoa(p)
		}
	case string:
		return p
	case nil:
		return ""
	default:
		return utils.Itoa(p)
	}
}
