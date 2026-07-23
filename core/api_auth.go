package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

var authBucket = MakeBucket("auths")
var auths = []*Auth{}
var password = ""
var setupLock sync.Mutex

func init() {
	storage.Watch(sillyGirl, "name", func(old, new, key string) *storage.Final {
		if old == new {
			return &storage.Final{
				Error: errors.New("unchanged"),
			}
		}
		return nil
	})
	authBucket.Foreach(func(b1, b2 []byte) error {
		auth := &Auth{}
		if json.Unmarshal(b2, auth) == nil {
			if math.Abs(float64(int(time.Now().Unix())-auth.CreatedAt)) < 86400 {
				auths = append(auths, auth)
			}
		}
		return nil
	})
	password = sillyGirl.GetString("password")
	var name = sillyGirl.GetString("name", "傻妞")
	// if password == "" {
	// password = utils.GenUUID()
	// console.Info("可视化面板临时账号密码：%s %s", name, password)
	// }
	storage.Watch(sillyGirl, "password", func(old, new, key string) *storage.Final {
		if new == "" {
			return &storage.Final{
				Now: new,
			}
		}
		password, _ = EncryptByAes([]byte(new))
		return &storage.Final{
			Now: password,
		}
	})
	storage.Watch(sillyGirl, "name", func(old, new, key string) *storage.Final {
		name = new
		return nil
	})
	///可视化部分
	GinApi(GET, "/api/setup/status", func(ctx *gin.Context) {
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"initialized": strings.TrimSpace(password) != "",
			},
		})
	})
	GinApi(POST, "/api/setup/admin", func(ctx *gin.Context) {
		setupLock.Lock()
		defer setupLock.Unlock()
		if strings.TrimSpace(password) != "" {
			ctx.JSON(200, map[string]interface{}{
				"success": false,
				"errorMessage": "后台账号已初始化",
			})
			return
		}
		payload := struct {
			Password string `json:"password"`
			Username string `json:"username"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		payload.Username = strings.TrimSpace(payload.Username)
		if payload.Username == "" {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": "账号不能为空"})
			return
		}
		if strings.TrimSpace(payload.Password) == "" {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": "密码不能为空"})
			return
		}
		sillyGirl.Set("name", payload.Username)
		sillyGirl.Set("password", payload.Password)
		name = payload.Username
		token := utils.GenUUID()
		auth := &Auth{
			IP:        ctx.ClientIP(),
			UserAgent: ctx.Request.UserAgent(),
			Token:     token,
			CreatedAt: int(time.Now().Unix()),
		}
		authBucket.Create(auth)
		auths = append(auths, auth)
		ctx.SetCookie("token", token, 86400, "/", "", false, true)
		ctx.JSON(200, map[string]interface{}{
			"success":          true,
			"status":           "ok",
			"type":             "account",
			"currentAuthority": "admin",
		})
	})
	GinApi(POST, "/api/login/account", func(ctx *gin.Context) {
		var auth = struct {
			Password string `json:"password"`
			Username string `json:"username"`
		}{}
		json.NewDecoder(ctx.Request.Body).Decode(&auth)
		if strings.TrimSpace(password) == "" {
			ctx.JSON(200, map[string]interface{}{
				"success":          true,
				"status":           "setup_required",
				"type":             "account",
				"currentAuthority": "guest",
				"setupRequired":    true,
			})
			return
		}
		epassword, _ := EncryptByAes([]byte(auth.Password))
		if (auth.Password == password || epassword == password) && auth.Username == name {
			token := utils.GenUUID()
			auth := &Auth{
				IP:        ctx.ClientIP(),
				UserAgent: ctx.Request.UserAgent(),
				Token:     token,
				CreatedAt: int(time.Now().Unix()),
			}
			authBucket.Create(auth)
			auths = append(auths, auth)
			console.Log("登录成功，当前有效令牌数%d，总数%d", len(ValidAuths()), len(auths))
			ctx.SetCookie("token", token, 86400, "/", "", false, true)
			ctx.JSON(200, map[string]interface{}{
				"status":           "ok",
				"type":             "account",
				"currentAuthority": "admin",
			})
		} else {
			ctx.JSON(200, map[string]interface{}{
				"status":           "error",
				"type":             "account",
				"currentAuthority": "guest",
			})
		}
	})
	GinApi(POST, "/api/login/outLogin", DestroyAuth, func(ctx *gin.Context) {
		sillyGirl.Set("web_token", "")
		ctx.JSON(200, map[string]interface{}{
			"success": true,
		})
	})
	pluginNextUuid := sillyGirl.GetString("pluginNextUuid")
	if pluginNextUuid == "" {
		pluginNextUuid = utils.GenUUID()
		sillyGirl.Set("pluginNextUuid", pluginNextUuid)
	}
	GinApi(GET, "/api/currentUser", RequireAuth, func(ctx *gin.Context) {
		rs := []Route{}
		for _, f := range Functions {
			if f.UUID == pluginNextUuid {
				pluginNextUuid = utils.GenUUID()
				sillyGirl.Set("pluginNextUuid", pluginNextUuid)
			}
			if f.UUID != "" {
				name := f.Title
				if name == "" {
					name = "无名脚本"
				}
				if f.Module {
					name = name + " 🔧"
				}
				if f.OnStart {
					name = name + " 💫"
				}
				if f.Encrypt {
					name = name + " 🔒"
				}
				if f.Public {
					name = name + " 👑"
				}
				rs = append(rs, Route{
					Path:      fmt.Sprintf(`/script/%s`, f.UUID),
					Name:      name,
					Type:      f.Type,
					File:      f.Path,
					Plugin:    nodePluginNameFromPath(f.Path),
					Component: "./Script",
					CreateAt:  f.CreateAt,
				})
			}
		}
		rrs := rs
		n := len(rrs)
		flag := true
		for i := 0; i < n && flag; i++ {
			flag = false
			for j := 0; j < n-i-1; j++ {
				if rrs[j].CreateAt < rrs[j+1].CreateAt {
					rrs[j], rrs[j+1] = rrs[j+1], rrs[j]
					flag = true
				}
			}
		}
		ctx.JSON(200, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"name":         sillyGirl.GetString("name"),
				"avatar":       "https://gw.alipayobjects.com/zos/antfincdn/XAosXuNZyF/BiazfanxmamNRoxxVxka.png",
				"plugins":      rrs,
				"adapters":     overviewAdapterStatuses(),
				"integrations": overviewIntegrationStatuses(),
				"version":      overviewVersionInfo(),
			},
		})
	})
}

func overviewVersionInfo() map[string]interface{} {
	local := compiled_at
	if strings.TrimSpace(local) == "" {
		local = "dev"
	}
	return map[string]interface{}{
		"local":      local,
		"remote":     strings.TrimSpace(firstNonEmpty(sillyGirl.GetString("remote_version"), sillyGirl.GetString("latest_version"))),
		"source":     "reserved",
		"repository": "https://github.com/smallfawn/sillyGirl",
	}
}

func overviewAdapterStatuses() []map[string]interface{} {
	platforms := []struct {
		Platform string
		Label    string
	}{
		{Platform: "pagermaid", Label: "Pagermaid"},
		{Platform: "qq", Label: "QQ"},
		{Platform: "web", Label: "Web"},
		{Platform: "telegram", Label: "Telegram Bot"},
	}
	rows := []map[string]interface{}{}
	for _, item := range platforms {
		botsID := GetAdapterBotsID(item.Platform)
		rows = append(rows, map[string]interface{}{
			"platform": item.Platform,
			"label":    item.Label,
			"online":   len(botsID) > 0,
			"bots_id":  botsID,
			"count":    len(botsID),
		})
	}
	return rows
}

func overviewIntegrationStatuses() map[string]interface{} {
	qinglongPanels := getQinglongPanels()
	smallcatPanels := getSmallcatPanels()
	return map[string]interface{}{
		"qinglong": overviewPanelStatus("青龙容器", len(qinglongPanels), countOnlineQinglongPanels(qinglongPanels)),
		"smallcat": overviewPanelStatus("smallcat", len(smallcatPanels), countOnlineSmallcatPanels(smallcatPanels)),
	}
}

func overviewPanelStatus(label string, count int, onlineCount int) map[string]interface{} {
	return map[string]interface{}{
		"label":        label,
		"count":        count,
		"online_count": onlineCount,
		"online":       count > 0 && onlineCount == count,
	}
}

func countOnlineQinglongPanels(panels []QinglongPanel) int {
	count := 0
	for _, panel := range panels {
		if panel.Status == "online" {
			count++
		}
	}
	return count
}

func countOnlineSmallcatPanels(panels []SmallcatPanel) int {
	count := 0
	for _, panel := range panels {
		if panel.Status == "online" {
			count++
		}
	}
	return count
}

func DestroyAuth(c *gin.Context) {
	token, _ := c.Cookie("token")
	auth, _ := CheckAuth(token)
	if auth != nil {
		auth.ExpiredAt = int(time.Now().Unix())
		authBucket.Create(auth)
	}
}

var tempAuth sync.Map

func getTempAuth() string {
	uuid := utils.GenUUID()
	tempAuth.Store(uuid, time.Now().Unix())
	return uuid
}

func checkTempAuth(uuid string) bool {
	unix, ok := tempAuth.LoadAndDelete(uuid)
	if !ok {
		return false
	}
	if time.Now().Unix()-unix.(int64) > 1 {
		return false
	}
	return true
}

func RequireAuth(c *gin.Context) {
	if strings.TrimSpace(password) == "" {
		c.JSON(401, map[string]interface{}{
			"data": map[string]interface{}{
				"isLogin":       false,
				"setupRequired": true,
			},
			"errorCode":    "401",
			"errorMessage": "后台未初始化，请先设置账号密码",
			"success":      true,
			"showType":     9,
		})
		panic(errors.New("后台未初始化，请先设置账号密码"))
	}
	token, _ := c.Cookie("token")
	_, err := CheckAuth(token)
	if err != nil && !checkTempAuth(token) {
		c.JSON(401, map[string]interface{}{
			"data": map[string]interface{}{
				"isLogin": false,
			},
			"errorCode":    "401",
			"errorMessage": err.Error(),
			"success":      true,
			"showType":     9,
		})
		panic(err)
	}
}

func CheckAuth(token string) (*Auth, error) {
	var errorMessage = "请先登录！"
	if token != "" {
		auths := auths
		for i := range auths {
			if auths[i].Token == token && auths[i].ExpiredAt == 0 {
				if math.Abs(float64(int(time.Now().Unix())-auths[i].CreatedAt)) > 86400 {
					auths[i].ExpiredAt = int(time.Now().Unix())
					authBucket.Create(auths[i])
					errorMessage = "授权已过期！"
				} else {
					return auths[i], nil
				}
			} else {
				errorMessage = "非法访问！"
			}
		}
	}
	return nil, errors.New(errorMessage)
}

func ValidAuths() []*Auth {
	tmp := []*Auth{}
	for _, auth := range auths {
		if auth.ExpiredAt == 0 {
			tmp = append(tmp, auth)
		}

	}
	return tmp
}
