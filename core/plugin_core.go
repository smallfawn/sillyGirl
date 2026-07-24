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
var plugin_dir = nodePluginsRoot()

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
				Error: errors.New("旧内嵌 JS 插件数据已不支持，请使用 /data/plugins/*.js 的 NodeJS 插件"),
			}
		}
		pluginLock.Lock()
		defer pluginLock.Unlock()
		if new == "install" {
			var marketPlugin *common.Function
			for _, p := range plugin_list {
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
			if err := installGithubNodePlugin(marketPlugin.Address); err != nil {
				return &storage.Final{
					Error: errors.New("安装异常！" + err.Error()),
				}
			}
			return &storage.Final{
				Now:     storage.EMPTY,
				Message: fmt.Sprintf("已安装 %s", marketPlugin.Title),
			}
		}
		if new == "uninstall" {
			new = ""
			fin = &storage.Final{
				Now: storage.EMPTY,
			}

		}
		for i := range Functions {
			if Functions[i].UUID == key {
				current := Functions[i]
				DestroyAdapterByUUID(key)
				current.Running = false
				if len(current.CronIds) != 0 {
					for _, id := range current.CronIds {
						CRON.Remove(cron.EntryID(id))
					}
				}
				Functions = append(Functions[:i], Functions[i+1:]...)
				CancelPluginCrons(key)
				CancelPluginWebs(key)
				CancelPluginlistening(key)
				remStatic(key)
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
						if isFlatNodePluginPath(filename) {
							os.Remove(filename)
						} else {
							os.RemoveAll(filepath.Dir(filename))
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

func GetFunctionByUUID(uuid string) *common.Function {
	for _, f := range Functions {
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
