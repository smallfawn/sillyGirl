package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
	"golang.org/x/crypto/bcrypt"
)

var authBucket = MakeBucket("auths")
var auths = []*Auth{}
var password = ""
var setupLock sync.Mutex
var loginAttemptLock sync.Mutex
var loginAttempts = map[string]*loginAttemptState{}

const adminJWTExpireSeconds = 86400
const adminPasswordHashCost = bcrypt.DefaultCost
const loginAttemptWindow = 15 * time.Minute
const maxLoginAttempts = 5

type adminJWTClaims struct {
	Sub string `json:"sub"`
	JTI string `json:"jti"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

type loginAttemptState struct {
	Count     int
	FirstSeen time.Time
	LockedTil time.Time
}

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
		new = strings.TrimSpace(new)
		if new == "" {
			password = ""
			return &storage.Final{
				Now: new,
			}
		}
		if isAdminPasswordHash(new) {
			password = new
			return &storage.Final{
				Now: new,
			}
		}
		hashed, err := hashAdminPassword(new)
		if err != nil {
			return &storage.Final{
				Error: err,
			}
		}
		password = hashed
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
				"success":      false,
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
		token, err := createAdminJWTSession(ctx, name)
		if err != nil {
			ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
			return
		}
		ctx.JSON(200, map[string]interface{}{
			"success":          true,
			"status":           "ok",
			"type":             "account",
			"currentAuthority": "admin",
			"token":            token,
			"expiresIn":        adminJWTExpireSeconds,
		})
	})
	GinApi(POST, "/api/login/account", func(ctx *gin.Context) {
		var auth = struct {
			Password string `json:"password"`
			Username string `json:"username"`
		}{}
		json.NewDecoder(ctx.Request.Body).Decode(&auth)
		auth.Username = strings.TrimSpace(auth.Username)
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
		if loginAttemptBlocked(ctx, auth.Username) {
			ctx.JSON(200, map[string]interface{}{
				"success":          true,
				"status":           "error",
				"type":             "account",
				"currentAuthority": "guest",
				"errorMessage":     "登录失败次数过多，请稍后再试",
			})
			return
		}
		if verifyAdminPassword(auth.Password) && auth.Username == name {
			clearLoginAttempts(ctx, auth.Username)
			token, err := createAdminJWTSession(ctx, name)
			if err != nil {
				ctx.JSON(200, map[string]interface{}{"success": false, "errorMessage": err.Error()})
				return
			}
			console.Log("登录成功，当前有效令牌数%d，总数%d", len(ValidAuths()), len(auths))
			ctx.JSON(200, map[string]interface{}{
				"success":          true,
				"status":           "ok",
				"type":             "account",
				"currentAuthority": "admin",
				"token":            token,
				"expiresIn":        adminJWTExpireSeconds,
			})
		} else {
			recordFailedLoginAttempt(ctx, auth.Username)
			ctx.JSON(200, map[string]interface{}{
				"success":          true,
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
	latest, source := latestAppVersion()
	return map[string]interface{}{
		"local":      currentAppVersion(),
		"remote":     latest,
		"source":     source,
		"repository": appRepository,
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
	daidaiPanels := getDaidaiPanels()
	return map[string]interface{}{
		"qinglong": overviewPanelStatus("青龙容器", len(qinglongPanels), countOnlineQinglongPanels(qinglongPanels)),
		"smallcat": overviewPanelStatus("smallcat", len(smallcatPanels), countOnlineSmallcatPanels(smallcatPanels)),
		"daidai":   overviewPanelStatus("呆呆容器", len(daidaiPanels), countOnlineDaidaiPanels(daidaiPanels)),
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

func countOnlineDaidaiPanels(panels []DaidaiPanel) int {
	count := 0
	for _, panel := range panels {
		if panel.Status == "online" {
			count++
		}
	}
	return count
}

func DestroyAuth(c *gin.Context) {
	token := authTokenFromRequest(c)
	auth, _ := CheckAuth(token)
	if auth != nil {
		auth.ExpiredAt = int(time.Now().Unix())
		authBucket.Create(auth)
	}
	c.SetCookie("token", "", -1, "/", "", false, true)
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
	token := authTokenFromRequest(c)
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
		sessionToken := token
		if strings.Count(token, ".") == 2 {
			claims, err := parseAdminJWT(token)
			if err != nil {
				return nil, err
			}
			sessionToken = claims.JTI
		}
		if auth, err := checkSessionToken(sessionToken); err == nil {
			return auth, nil
		} else {
			errorMessage = err.Error()
		}
	}
	return nil, errors.New(errorMessage)
}

func authTokenFromRequest(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	token, _ := c.Cookie("token")
	return strings.TrimSpace(token)
}

func createAdminJWTSession(ctx *gin.Context, username string) (string, error) {
	now := time.Now().Unix()
	sessionToken := utils.GenUUID()
	auth := &Auth{
		IP:        ctx.ClientIP(),
		UserAgent: ctx.Request.UserAgent(),
		Token:     sessionToken,
		CreatedAt: int(now),
	}
	authBucket.Create(auth)
	auths = append(auths, auth)
	token, err := signAdminJWT(adminJWTClaims{
		Sub: username,
		JTI: sessionToken,
		Iat: now,
		Exp: now + adminJWTExpireSeconds,
	})
	if err != nil {
		return "", err
	}
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("token", token, adminJWTExpireSeconds, "/", "", adminCookieSecure(ctx), true)
	return token, nil
}

func checkSessionToken(token string) (*Auth, error) {
	errorMessage := "请先登录！"
	for i := range auths {
		if auths[i].Token != token {
			errorMessage = "非法访问！"
			continue
		}
		if auths[i].ExpiredAt != 0 {
			return nil, errors.New("授权已失效！")
		}
		if math.Abs(float64(int(time.Now().Unix())-auths[i].CreatedAt)) > adminJWTExpireSeconds {
			auths[i].ExpiredAt = int(time.Now().Unix())
			authBucket.Create(auths[i])
			return nil, errors.New("授权已过期！")
		}
		return auths[i], nil
	}
	return nil, errors.New(errorMessage)
}

func signAdminJWT(claims adminJWTClaims) (string, error) {
	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return unsigned + "." + signAdminJWTPart(unsigned), nil
}

func parseAdminJWT(token string) (*adminJWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("JWT 格式错误")
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(signAdminJWTPart(unsigned))) {
		return nil, errors.New("JWT 签名无效")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("JWT 头解析失败")
	}
	header := map[string]string{}
	if err := json.Unmarshal(headerRaw, &header); err != nil || header["alg"] != "HS256" {
		return nil, errors.New("JWT 算法不支持")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("JWT 内容解析失败")
	}
	claims := &adminJWTClaims{}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, errors.New("JWT 内容无效")
	}
	if claims.JTI == "" || claims.Sub == "" {
		return nil, errors.New("JWT 缺少会话信息")
	}
	if claims.Exp <= time.Now().Unix() {
		return nil, errors.New("JWT 已过期")
	}
	if currentName := strings.TrimSpace(sillyGirl.GetString("name")); currentName != "" && claims.Sub != currentName {
		return nil, errors.New("JWT 用户不匹配")
	}
	return claims, nil
}

func signAdminJWTPart(unsigned string) string {
	mac := hmac.New(sha256.New, adminJWTSecret())
	mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func adminJWTSecret() []byte {
	sum := sha256.Sum256([]byte(GetMachineID() + "|" + password + "|sillyGirl-admin-jwt"))
	return sum[:]
}

func hashAdminPassword(raw string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), adminPasswordHashCost)
	return string(hash), err
}

func isAdminPasswordHash(value string) bool {
	return strings.HasPrefix(value, "$2a$") || strings.HasPrefix(value, "$2b$") || strings.HasPrefix(value, "$2y$")
}

func verifyAdminPassword(raw string) bool {
	stored := strings.TrimSpace(password)
	if stored == "" {
		return false
	}
	if isAdminPasswordHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(raw)) == nil
	}
	legacyEncrypted, _ := EncryptByAes([]byte(raw))
	if raw == stored || legacyEncrypted == stored {
		if hashed, err := hashAdminPassword(raw); err == nil {
			sillyGirl.Set("password", hashed)
			password = hashed
		}
		return true
	}
	return false
}

func adminCookieSecure(ctx *gin.Context) bool {
	if strings.EqualFold(strings.TrimSpace(sillyGirl.GetString("secure_cookie")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("SILLYGIRL_SECURE_COOKIE")), "true") {
		return true
	}
	return ctx.Request.TLS != nil || strings.EqualFold(ctx.GetHeader("X-Forwarded-Proto"), "https")
}

func loginAttemptKey(ctx *gin.Context, username string) string {
	return ctx.ClientIP() + "|" + strings.ToLower(strings.TrimSpace(username))
}

func loginAttemptBlocked(ctx *gin.Context, username string) bool {
	loginAttemptLock.Lock()
	defer loginAttemptLock.Unlock()
	state := loginAttempts[loginAttemptKey(ctx, username)]
	if state == nil {
		return false
	}
	now := time.Now()
	if !state.LockedTil.IsZero() && state.LockedTil.After(now) {
		return true
	}
	if now.Sub(state.FirstSeen) > loginAttemptWindow {
		delete(loginAttempts, loginAttemptKey(ctx, username))
	}
	return false
}

func recordFailedLoginAttempt(ctx *gin.Context, username string) {
	loginAttemptLock.Lock()
	defer loginAttemptLock.Unlock()
	key := loginAttemptKey(ctx, username)
	now := time.Now()
	state := loginAttempts[key]
	if state == nil || now.Sub(state.FirstSeen) > loginAttemptWindow {
		state = &loginAttemptState{FirstSeen: now}
		loginAttempts[key] = state
	}
	state.Count++
	if state.Count >= maxLoginAttempts {
		state.LockedTil = now.Add(loginAttemptWindow)
	}
}

func clearLoginAttempts(ctx *gin.Context, username string) {
	loginAttemptLock.Lock()
	defer loginAttemptLock.Unlock()
	delete(loginAttempts, loginAttemptKey(ctx, username))
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
