package core

import (
	"time"

	"github.com/beego/beego/v2/client/httplib"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

func Init() {
	initLoc()
	sillyGirl = MakeBucket("sillyGirl")
	// utils.ReadYaml(utils.ExecPath+"/conf/", &Config, "https://raw.githubusercontent.com/smallfawn/sillyGirl/main/conf/demo_config.yaml")
	initToHandleMessage()
	sillyGirl.Set("compiled_at", currentAppVersion())
	sillyGirl.Set("version", currentAppVersion())
	rememberLatestAppVersion(currentAppVersion(), versionAcceleratedURLs(remoteVersionRawURL)[0])
	go refreshAppVersionLoop()
	console.Log("当前版本: %s", currentAppVersion())
	initWeb()
	initCarry()
	sillyGirl.Set("started_at", time.Now().Format("2006-01-02 15:04:05"))
	storage.Watch(sillyGirl, "compiled_at", func(old, new, key string) *storage.Final {
		if old != new {
			return &storage.Final{
				Message: "正式版升级请使用 GitHub Release 包或 Docker 镜像更新。",
			}
		}
		return nil
	})
	storage.Watch(sillyGirl, "started_at", func(old, new, key string) *storage.Final {
		if old != new {
			go func() {
				time.Sleep(time.Second)
				utils.Daemon()
			}()
			return &storage.Final{
				Message: "1秒重启！",
			}
		}
		return nil
	})

	api_key := sillyGirl.GetString("api_key")
	if api_key == "" {
		api_key := time.Now().UnixNano()
		sillyGirl.Set("api_key", api_key)
	}
	// if sillyGirl.GetString("uuid") == "" {
	sillyGirl.Set("uuid", utils.GenUUID())
	// }
	httplib.SetDefaultSetting(httplib.BeegoHTTPSettings{
		ConnectTimeout:   time.Second * 10,
		ReadWriteTimeout: time.Second * 10,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/99.0.4844.51 Safari/537.36",
	})
	initPlugins()
	initReboot()
	initListenReply()
	// initPluginFile()
	initWebPluginList()
	go initPluginList()
	initPluginPublish()

}
