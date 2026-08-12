package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
var authsLock sync.RWMutex
var password = ""
var name = ""
var setupLock sync.Mutex
var loginAttemptLock sync.Mutex
var loginAttempts = map[string]*loginAttemptState{}
var loginAttemptLastPrune time.Time

// credLock 保护并发读写的管理员密码/账号名。password/name 为包级变量，
// 在 HTTP 处理协程与 storage.Watch 回调间被并发访问，必须加锁避免数据竞争。
var credLock sync.RWMutex

func getAdminPassword() string {
	credLock.RLock()
	defer credLock.RUnlock()
	return password
}

func setAdminPassword(p string) {
	credLock.Lock()
	defer credLock.Unlock()
	password = p
}

func getAdminName() string {
	credLock.RLock()
	defer credLock.RUnlock()
	return name
}

func setAdminName(n string) {
	credLock.Lock()
	defer credLock.Unlock()
	name = n
}

const adminJWTExpireSeconds = 3 * 24 * 60 * 60
const adminPasswordHashCost = bcrypt.DefaultCost
const loginAttemptWindow = 15 * time.Minute
const maxLoginAttempts = 5
const maxTrackedLoginAttempts = 4096

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
			if adminSessionValidAt(auth, time.Now().Unix()) {
				cacheAdminSession(auth)
			}
		}
		return nil
	})
	setAdminPassword(sillyGirl.GetString("password"))
	name = sillyGirl.GetString("name", "傻妞")
	// if password == "" {
	// password = utils.GenUUID()
	// console.Info("可视化面板临时账号密码：%s %s", name, password)
	// }
	storage.Watch(sillyGirl, "password", func(old, new, key string) *storage.Final {
		new = strings.TrimSpace(new)
		if new == "" {
			setAdminPassword("")
			return &storage.Final{
				Now: new,
			}
		}
		if isAdminPasswordHash(new) {
			setAdminPassword(new)
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
		setAdminPassword(hashed)
		return &storage.Final{
			Now: password,
		}
	})
	storage.Watch(sillyGirl, "name", func(old, new, key string) *storage.Final {
		setAdminName(new)
		return nil
	})
	///可视化部分
	GinApi(GET, "/api/admin/setup", func(ctx *gin.Context) {
		ApiOK(ctx, map[string]interface{}{
			"initialized": strings.TrimSpace(getAdminPassword()) != "",
		})
	})
	GinApi(POST, "/api/admin/setup", func(ctx *gin.Context) {
		setupLock.Lock()
		defer setupLock.Unlock()
		if strings.TrimSpace(password) != "" {
			ApiConflict(ctx, "后台账号已初始化")
			return
		}
		payload := struct {
			Password string `json:"password"`
			Username string `json:"username"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		payload.Username = strings.TrimSpace(payload.Username)
		if payload.Username == "" {
			ApiUnprocessable(ctx, "账号不能为空")
			return
		}
		if strings.TrimSpace(payload.Password) == "" {
			ApiUnprocessable(ctx, "密码不能为空")
			return
		}
		sillyGirl.Set("name", payload.Username)
		sillyGirl.Set("password", payload.Password)
		setAdminName(payload.Username)
		token, err := createAdminJWTSession(ctx, getAdminName())
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		ApiCreated(ctx, "/api/admin/sessions/current", map[string]interface{}{
			"status":           "ok",
			"type":             "account",
			"currentAuthority": "admin",
			"token":            token,
			"expiresIn":        adminJWTExpireSeconds,
		})
	})
	GinApi(POST, "/api/admin/sessions", handleAdminLogin)
	GinApi(POST, "/api/admin/sessions/current/deletions", RequireAuth, DestroyAuth, func(ctx *gin.Context) {
		sillyGirl.Set("web_token", "")
		ApiOK(ctx, nil)
	})
	pluginNextUuid := sillyGirl.GetString("pluginNextUuid")
	if pluginNextUuid == "" {
		pluginNextUuid = utils.GenUUID()
		sillyGirl.Set("pluginNextUuid", pluginNextUuid)
	}
	GinApi(GET, "/api/admin/sessions/current", RequireAuth, func(ctx *gin.Context) {
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
		ApiOK(ctx, map[string]interface{}{
			"name":         sillyGirl.GetString("name"),
			"avatar":       "https://gw.alipayobjects.com/zos/antfincdn/XAosXuNZyF/BiazfanxmamNRoxxVxka.png",
			"plugins":      rrs,
			"adapters":     overviewAdapterStatuses(),
			"integrations": overviewIntegrationStatuses(),
			"user_stats":   overviewUserStats(),
			"version":      overviewVersionInfo(),
		})
	})
}

func handleAdminLogin(ctx *gin.Context) {
	auth := struct {
		Password string `json:"password"`
		Username string `json:"username"`
	}{}
	if err := json.NewDecoder(ctx.Request.Body).Decode(&auth); err != nil {
		ApiFail(ctx, "请求体不是有效 JSON")
		return
	}
	auth.Username = strings.TrimSpace(auth.Username)
	if strings.TrimSpace(getAdminPassword()) == "" {
		ApiConflict(ctx, "后台尚未初始化")
		return
	}
	attemptKey := "admin:" + auth.Username
	if loginAttemptBlocked(ctx, attemptKey) {
		ApiError(ctx, http.StatusTooManyRequests, "登录失败次数过多，请稍后再试")
		return
	}
	adminName := strings.TrimSpace(sillyGirl.GetString("name", "傻妞"))
	if verifyAdminPassword(auth.Password) && auth.Username == adminName {
		clearLoginAttempts(ctx, attemptKey)
		token, err := createAdminJWTSession(ctx, adminName)
		if err != nil {
			ApiInternalError(ctx, err.Error())
			return
		}
		console.Log("登录成功，当前有效令牌数%d，总数%d", len(ValidAuths()), adminSessionCount())
		ApiCreated(ctx, "/api/admin/sessions/current", map[string]interface{}{
			"status":           "ok",
			"type":             "account",
			"currentAuthority": "admin",
			"token":            token,
			"expiresIn":        adminJWTExpireSeconds,
		})
	} else {
		recordFailedLoginAttempt(ctx, attemptKey)
		ApiUnauthorized(ctx, "账号或密码错误")
	}
}

func overviewUserStats() map[string]interface{} {
	rows, err := listNormalUsers()
	if err != nil {
		return map[string]interface{}{
			"total": 0,
			"today": 0,
			"error": err.Error(),
		}
	}
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	today := 0
	for _, row := range rows {
		if row.CreatedAt >= startOfDay {
			today++
		}
	}
	return map[string]interface{}{
		"total": len(rows),
		"today": today,
	}
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
		{Platform: "clawbot", Label: "微信 ClawBot"},
		{Platform: "dingtalk", Label: "钉钉机器人"},
		{Platform: "flowbot", Label: "FlowBot 微信"},
		{Platform: "pagermaid", Label: "Pagermaid"},
		{Platform: "qq", Label: "QQ"},
		{Platform: "qqguild", Label: "QQ 官方频道机器人"},
		{Platform: "web", Label: "Web Bot"},
		{Platform: "telegram", Label: "Telegram Bot"},
	}
	rows := []map[string]interface{}{}
	for _, item := range platforms {
		botsID := GetAdapterBotsID(item.Platform)
		rows = append(rows, map[string]interface{}{
			"platform":   item.Platform,
			"label":      item.Label,
			"online":     len(botsID) > 0,
			"enabled":    AdapterConfigEnabled(item.Platform),
			"manageable": AdapterConfigManageable(item.Platform),
			"bots_id":    botsID,
			"count":      len(botsID),
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
		cacheAdminSession(auth)
	}
}

func RequireAuth(c *gin.Context) {
	if strings.TrimSpace(getAdminPassword()) == "" {
		ApiError(c, http.StatusUnauthorized, "后台未初始化，请先设置账号密码")
		c.Abort()
		return
	}
	token := authTokenFromRequest(c)
	_, err := CheckAuth(token)
	if err != nil {
		ApiError(c, http.StatusUnauthorized, err.Error())
		c.Abort()
	}
}

func CheckAuth(token string) (*Auth, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("请先登录！")
	}
	claims, err := parseAdminJWT(token)
	if err != nil {
		return nil, err
	}
	return checkSessionToken(claims.JTI)
}

func authTokenFromRequest(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader("token"))
}

// CheckAuthRequest validates an administrator JWT from the token request
// header. Adapters with optional public endpoints use it to distinguish an
// authenticated administrator without forcing a RequireAuth response.
func CheckAuthRequest(c *gin.Context) (*Auth, error) {
	return CheckAuth(authTokenFromRequest(c))
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
	cacheAdminSession(auth)
	token, err := signAdminJWT(adminJWTClaims{
		Sub: username,
		JTI: sessionToken,
		Iat: now,
		Exp: now + adminJWTExpireSeconds,
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

func checkSessionToken(token string) (*Auth, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("请先登录！")
	}
	auth := cachedAdminSession(token)
	if auth == nil {
		auth = storedAdminSession(token)
		if auth != nil {
			cacheAdminSession(auth)
		}
	}
	if auth == nil {
		return nil, errors.New("登录会话不存在或已失效！")
	}
	if auth.ExpiredAt != 0 {
		return nil, errors.New("授权已失效！")
	}
	if !adminSessionValidAt(auth, time.Now().Unix()) {
		auth.ExpiredAt = int(time.Now().Unix())
		authBucket.Create(auth)
		cacheAdminSession(auth)
		return nil, errors.New("授权已过期！")
	}
	return auth, nil
}

func adminSessionValidAt(auth *Auth, now int64) bool {
	if auth == nil || auth.ExpiredAt != 0 || auth.CreatedAt <= 0 || int64(auth.CreatedAt) > now {
		return false
	}
	return now-int64(auth.CreatedAt) <= adminJWTExpireSeconds
}

func cachedAdminSession(token string) *Auth {
	authsLock.RLock()
	defer authsLock.RUnlock()
	for _, auth := range auths {
		if auth != nil && auth.Token == token {
			copyAuth := *auth
			return &copyAuth
		}
	}
	return nil
}

func storedAdminSession(token string) *Auth {
	var found *Auth
	authBucket.Foreach(func(_, value []byte) error {
		if found != nil {
			return nil
		}
		auth := &Auth{}
		if json.Unmarshal(value, auth) == nil && auth.Token == token {
			found = auth
		}
		return nil
	})
	return found
}

func cacheAdminSession(auth *Auth) {
	if auth == nil || strings.TrimSpace(auth.Token) == "" {
		return
	}
	copyAuth := *auth
	authsLock.Lock()
	defer authsLock.Unlock()
	for i, current := range auths {
		if current != nil && current.Token == copyAuth.Token {
			auths[i] = &copyAuth
			return
		}
	}
	auths = append(auths, &copyAuth)
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
	if jwtClaimsExpired(time.Now().Unix(), claims.Iat, claims.Exp, adminJWTExpireSeconds) {
		return nil, errors.New("JWT 已过期")
	}
	if currentName := strings.TrimSpace(sillyGirl.GetString("name")); currentName != "" && claims.Sub != currentName {
		return nil, errors.New("JWT 用户不匹配")
	}
	return claims, nil
}

func jwtClaimsExpired(now, issuedAt, expiresAt, maxAgeSeconds int64) bool {
	if expiresAt <= now {
		return true
	}
	return issuedAt <= 0 || issuedAt > now || maxAgeSeconds <= 0 || issuedAt+maxAgeSeconds <= now
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
	stored := strings.TrimSpace(getAdminPassword())
	if stored == "" {
		return false
	}
	if isAdminPasswordHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(raw)) == nil
	}
	legacyMatch := false
	if decrypted, err := DecryptByAes(stored); err == nil {
		legacyMatch = string(decrypted) == raw
	}
	if raw == stored || legacyMatch {
		if hashed, err := hashAdminPassword(raw); err == nil {
			sillyGirl.Set("password", hashed)
			setAdminPassword(hashed)
		}
		return true
	}
	return false
}

func loginAttemptKeys(ctx *gin.Context, account string) []string {
	account = strings.ToLower(strings.TrimSpace(account))
	scope := "login"
	if prefix, _, ok := strings.Cut(account, ":"); ok && prefix != "" {
		scope = prefix
	}
	return []string{scope + "|ip:" + ctx.ClientIP(), scope + "|account:" + account}
}

func loginAttemptBlocked(ctx *gin.Context, account string) bool {
	loginAttemptLock.Lock()
	defer loginAttemptLock.Unlock()
	now := time.Now()
	pruneLoginAttemptsLocked(now)
	for _, key := range loginAttemptKeys(ctx, account) {
		state := loginAttempts[key]
		if state != nil && !state.LockedTil.IsZero() && state.LockedTil.After(now) {
			return true
		}
	}
	return false
}

func recordFailedLoginAttempt(ctx *gin.Context, account string) {
	loginAttemptLock.Lock()
	defer loginAttemptLock.Unlock()
	now := time.Now()
	pruneLoginAttemptsLocked(now)
	for _, key := range loginAttemptKeys(ctx, account) {
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
}

func clearLoginAttempts(ctx *gin.Context, account string) {
	loginAttemptLock.Lock()
	defer loginAttemptLock.Unlock()
	for _, key := range loginAttemptKeys(ctx, account) {
		delete(loginAttempts, key)
	}
}

func pruneLoginAttemptsLocked(now time.Time) {
	if len(loginAttempts) < maxTrackedLoginAttempts-2 && now.Sub(loginAttemptLastPrune) < time.Minute {
		return
	}
	loginAttemptLastPrune = now
	for key, state := range loginAttempts {
		if state == nil || (now.Sub(state.FirstSeen) > loginAttemptWindow && !state.LockedTil.After(now)) {
			delete(loginAttempts, key)
		}
	}
	for len(loginAttempts) >= maxTrackedLoginAttempts {
		oldestKey := ""
		var oldest time.Time
		for key, state := range loginAttempts {
			if state != nil && (oldestKey == "" || state.FirstSeen.Before(oldest)) {
				oldestKey, oldest = key, state.FirstSeen
			}
		}
		if oldestKey == "" {
			break
		}
		delete(loginAttempts, oldestKey)
	}
}

func ValidAuths() []*Auth {
	tmp := []*Auth{}
	authsLock.RLock()
	defer authsLock.RUnlock()
	for _, auth := range auths {
		if auth != nil && auth.ExpiredAt == 0 {
			copyAuth := *auth
			tmp = append(tmp, &copyAuth)
		}

	}
	return tmp
}

func adminSessionCount() int {
	authsLock.RLock()
	defer authsLock.RUnlock()
	return len(auths)
}
