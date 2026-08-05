package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/utils"
	"golang.org/x/crypto/bcrypt"
)

var userBucket = MakeBucket("users")

const userJWTExpireSeconds = 7 * 24 * 60 * 60

var (
	userNamePattern      = regexp.MustCompile(`^[A-Za-z0-9_\-.]{3,32}$`)
	userQQBindingPattern = regexp.MustCompile(`^\d{5,12}$`)
	userTGBindingPattern = regexp.MustCompile(`^-?\d{5,20}$`)
)

type normalUser struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Disabled     bool   `json:"disabled"`
}

type publicNormalUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	CreatedAt int64  `json:"created_at"`
}

type normalUserBindings struct {
	QQ              string   `json:"qq"`
	Telegram        string   `json:"telegram"`
	SmallcatOpenID  string   `json:"smallcat_openid"`
	SmallcatOpenIDs []string `json:"smallcat_openids"`
	UpdatedAt       int64    `json:"updated_at"`
}

type adminNormalUserRow struct {
	publicNormalUser
	Bindings   normalUserBindings `json:"bindings"`
	UpdatedAt  int64              `json:"updated_at"`
	Disabled   bool               `json:"disabled"`
	StorageKey string             `json:"storage_key"`
}

type adminNormalUserPayload struct {
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	Nickname        string   `json:"nickname"`
	Disabled        *bool    `json:"disabled"`
	QQ              string   `json:"qq"`
	Telegram        string   `json:"telegram"`
	SmallcatOpenIDs []string `json:"smallcat_openids"`
}

type userJWTClaims struct {
	Sub string `json:"sub"`
	UID string `json:"uid"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

func init() {
	GinApi(GET, "/api/admin/users", RequireAuth, func(ctx *gin.Context) {
		rows, err := listNormalUsers()
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"list":  rows,
			"total": len(rows),
		})
	})

	GinApi(POST, "/api/admin/users", RequireAuth, func(ctx *gin.Context) {
		payload := adminNormalUserPayload{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		user, err := createNormalUser(payload.Username, payload.Password, payload.Nickname)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		bindings, err := replaceNormalUserBindings(user.Username, payload.QQ, payload.Telegram, payload.SmallcatOpenIDs)
		if err != nil {
			_ = deleteNormalUser(user.Username)
			ApiFail(ctx, err.Error())
			return
		}
		if payload.Disabled != nil && user.Disabled != *payload.Disabled {
			user.Disabled = *payload.Disabled
			user.UpdatedAt = time.Now().Unix()
			if _, _, err := userBucket.Set(normalUserStorageKey(user.Username), utils.JsonMarshal(user)); err != nil {
				_ = deleteNormalUser(user.Username)
				ApiFail(ctx, err.Error())
				return
			}
		}
		ApiOK(ctx, adminNormalUserRowFor(user, bindings))
	})

	GinApi(PUT, "/api/admin/users", RequireAuth, func(ctx *gin.Context) {
		payload := adminNormalUserPayload{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		user, bindings, err := updateNormalUserByAdmin(payload)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, adminNormalUserRowFor(user, bindings))
	})

	GinApi(DELETE, "/api/admin/users", RequireAuth, func(ctx *gin.Context) {
		payload := struct {
			Username string `json:"username"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		if err := deleteNormalUser(payload.Username); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{"username": normalizeNormalUsername(payload.Username)})
	})

	GinApi(POST, "/api/user/register", func(ctx *gin.Context) {
		payload := struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Nickname string `json:"nickname"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		user, err := createNormalUser(payload.Username, payload.Password, payload.Nickname)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		token, err := createUserJWTCookie(ctx, user)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"token":     token,
			"expiresIn": userJWTExpireSeconds,
			"user":      toPublicNormalUser(user),
		})
	})

	GinApi(POST, "/api/user/login", func(ctx *gin.Context) {
		payload := struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		user, err := verifyNormalUser(payload.Username, payload.Password)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		token, err := createUserJWTCookie(ctx, user)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"token":     token,
			"expiresIn": userJWTExpireSeconds,
			"user":      toPublicNormalUser(user),
		})
	})

	GinApi(GET, "/api/user/me", RequireUserAuth, func(ctx *gin.Context) {
		user, ok := ctx.Get("normal_user")
		if !ok {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		ApiOK(ctx, gin.H{
			"user": toPublicNormalUser(user.(*normalUser)),
		})
	})

	GinApi(GET, "/api/user/profile", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		announcement := strings.TrimSpace(sillyGirl.GetString("user_announcement"))
		announcementEnabledValue := GetBucketKeyValue(sillyGirl, "user_announcement_enable")
		announcementEnabled := announcementEnabledValue == true || fmt.Sprint(announcementEnabledValue) == "true"
		ApiOK(ctx, gin.H{
			"user":            toPublicNormalUser(user),
			"bindings":        loadNormalUserBindings(user.Username),
			"smallcat_panels": publicSmallcatPanels(),
			"announcement": gin.H{
				"enabled": announcementEnabled,
				"content": announcement,
				"format":  normalizeUserAnnouncementFormat(sillyGirl.GetString("user_announcement_format")),
			},
		})
	})

	GinApi(PUT, "/api/user/bind", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			Platform string `json:"platform"`
			Value    string `json:"value"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		if !isPublicUserBindingPlatform(payload.Platform) {
			ApiFail(ctx, "普通用户只能绑定 QQ 或 Telegram；smallcat 请通过扫码登录")
			return
		}
		bindings, err := updateNormalUserBinding(user.Username, payload.Platform, payload.Value)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, bindings)
	})

	GinApi(DELETE, "/api/user/bind", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			Platform string `json:"platform"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		if !isPublicUserBindingPlatform(payload.Platform) {
			ApiFail(ctx, "普通用户只能解绑 QQ 或 Telegram")
			return
		}
		bindings, err := updateNormalUserBinding(user.Username, payload.Platform, "")
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, bindings)
	})

	GinApi(GET, "/api/user/smallcat/panels", RequireUserAuth, func(ctx *gin.Context) {
		ApiOK(ctx, publicSmallcatPanels())
	})

	GinApi(POST, "/api/user/smallcat/qr/start", RequireUserAuth, func(ctx *gin.Context) {
		payload := struct {
			Panel int         `json:"panel"`
			Type  interface{} `json:"type"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		panel, err := smallcatPanelByIndex(payload.Panel)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		body := map[string]interface{}{"type": payload.Type}
		raw, err := requestSmallcatJSON(panel, http.MethodPost, "/api/qr/start", body, nil)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data, message, ok := unwrapServiceData(decodeRawJSON(raw))
		if !ok {
			ApiFail(ctx, message)
			return
		}
		ApiOK(ctx, data)
	})

	GinApi(GET, "/api/user/smallcat/qr/status", RequireUserAuth, func(ctx *gin.Context) {
		panelIndex, _ := strconv.Atoi(ctx.Query("panel"))
		uuid := strings.TrimSpace(ctx.Query("uuid"))
		if uuid == "" {
			ApiFail(ctx, "缺少 uuid")
			return
		}
		panel, err := smallcatPanelByIndex(panelIndex)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		raw, err := requestSmallcatJSON(panel, http.MethodGet, "/api/qr/status", nil, map[string]string{"uuid": uuid})
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data, message, ok := unwrapServiceData(decodeRawJSON(raw))
		if !ok {
			ApiFail(ctx, message)
			return
		}
		ApiOK(ctx, data)
	})

	GinApi(POST, "/api/user/smallcat/login/confirm", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			Panel int    `json:"panel"`
			UUID  string `json:"uuid"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		payload.UUID = strings.TrimSpace(payload.UUID)
		if payload.UUID == "" {
			ApiFail(ctx, "缺少 uuid")
			return
		}
		panel, err := smallcatPanelByIndex(payload.Panel)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		raw, err := requestSmallcatJSON(panel, http.MethodGet, "/api/qr/status", nil, map[string]string{"uuid": payload.UUID})
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		result, message, ok := unwrapServiceData(decodeRawJSON(raw))
		if !ok {
			ApiFail(ctx, message)
			return
		}
		state := findStringInJSON(result, "state")
		wxCode := findStringInJSON(result, "wxCode", "wx_code", "code")
		if state != "confirmed" || wxCode == "" {
			if state == "" {
				state = "unknown"
			}
			ApiFail(ctx, "当前扫码状态："+state+"，请扫码确认后再点击确认登录")
			return
		}
		oauthState := findStringInJSON(result, "oauthState", "oauth_state", "state")
		addRaw, err := requestSmallcatJSON(panel, http.MethodPost, "/api/accounts/add", gin.H{
			"code":        wxCode,
			"uuid":        payload.UUID,
			"oauthState":  oauthState,
			"displayName": user.Nickname,
		}, nil)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		addResult, message, ok := unwrapServiceData(decodeRawJSON(addRaw))
		if !ok {
			ApiFail(ctx, message)
			return
		}
		openid := findStringInJSON(addResult, "openid", "openId", "open_id")
		if openid == "" {
			ApiFail(ctx, "smallcat 已确认扫码，但添加账号接口未返回 openid")
			return
		}
		bindings, err := updateNormalUserBinding(user.Username, "smallcat", openid)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"openid":   openid,
			"bindings": bindings,
			"status":   result,
			"raw":      addResult,
		})
	})

	GinApi(POST, "/api/user/smallcat/account/add", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			Panel       int    `json:"panel"`
			Code        string `json:"code"`
			Type        int    `json:"type"`
			DisplayName string `json:"displayName"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		payload.Code = strings.TrimSpace(payload.Code)
		payload.DisplayName = strings.TrimSpace(payload.DisplayName)
		if payload.Code == "" {
			ApiFail(ctx, "请输入授权码")
			return
		}
		panel, err := smallcatPanelByIndex(payload.Panel)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		raw, err := requestSmallcatJSON(panel, http.MethodPost, "/api/accounts/add", gin.H{
			"code":        payload.Code,
			"type":        payload.Type,
			"displayName": payload.DisplayName,
		}, nil)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data, message, ok := unwrapServiceData(decodeRawJSON(raw))
		if !ok {
			ApiFail(ctx, message)
			return
		}
		openid := findStringInJSON(data, "openid", "openId", "open_id")
		if openid == "" {
			ApiFail(ctx, "smallcat 添加账号成功，但接口未返回 openid")
			return
		}
		bindings, err := updateNormalUserBinding(user.Username, "smallcat", openid)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"openid":   openid,
			"bindings": bindings,
			"raw":      data,
		})
	})

	GinApi(POST, "/api/user/smallcat/code", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			Panel  int    `json:"panel"`
			OpenID string `json:"openid"`
			AppID  string `json:"appid"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		payload.OpenID = strings.TrimSpace(payload.OpenID)
		payload.AppID = strings.TrimSpace(payload.AppID)
		if payload.OpenID == "" || payload.AppID == "" {
			ApiFail(ctx, "openid 和 appid 不能为空")
			return
		}
		if !normalUserHasSmallcatOpenID(user.Username, payload.OpenID) {
			ApiFail(ctx, "只能为当前用户已绑定的 smallcat openid 生成 code")
			return
		}
		panel, err := smallcatPanelByIndex(payload.Panel)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		raw, err := requestSmallcatJSON(panel, http.MethodPost, "/wx/code", gin.H{
			"openid": payload.OpenID,
			"appid":  payload.AppID,
		}, nil)
		if err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		data, message, ok := unwrapServiceData(decodeRawJSON(raw))
		if !ok {
			ApiFail(ctx, message)
			return
		}
		ApiOK(ctx, data)
	})

	GinApi(POST, "/api/user/outlogin", func(ctx *gin.Context) {
		ctx.SetSameSite(http.SameSiteLaxMode)
		ctx.SetCookie("user_token", "", -1, "/", "", adminCookieSecure(ctx), true)
		ApiOK(ctx, nil)
	})
}

func createNormalUser(username string, password string, nickname string) (*normalUser, error) {
	username = normalizeNormalUsername(username)
	nickname = strings.TrimSpace(nickname)
	if err := validateNormalUsername(username); err != nil {
		return nil, err
	}
	if err := validateNormalPassword(password); err != nil {
		return nil, err
	}
	if existing, _ := loadNormalUser(username); existing != nil {
		return nil, errors.New("账号已存在")
	}
	if nickname == "" {
		nickname = username
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), adminPasswordHashCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	user := &normalUser{
		ID:           utils.GenUUID(),
		Username:     username,
		Nickname:     nickname,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, _, err := userBucket.Set(normalUserStorageKey(username), utils.JsonMarshal(user)); err != nil {
		return nil, err
	}
	return user, nil
}

func updateNormalUserByAdmin(payload adminNormalUserPayload) (*normalUser, normalUserBindings, error) {
	username := normalizeNormalUsername(payload.Username)
	if err := validateNormalUsername(username); err != nil {
		return nil, normalUserBindings{}, err
	}
	user, err := loadNormalUser(username)
	if err != nil {
		return nil, normalUserBindings{}, err
	}
	bindings, err := normalizedReplacementBindings(payload.QQ, payload.Telegram, payload.SmallcatOpenIDs)
	if err != nil {
		return nil, normalUserBindings{}, err
	}
	nickname := strings.TrimSpace(payload.Nickname)
	if nickname == "" {
		nickname = user.Username
	}
	if len([]rune(nickname)) > 64 {
		return nil, normalUserBindings{}, errors.New("昵称不能超过 64 位")
	}
	passwordHash := user.PasswordHash
	if payload.Password != "" {
		if err := validateNormalPassword(payload.Password); err != nil {
			return nil, normalUserBindings{}, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), adminPasswordHashCost)
		if err != nil {
			return nil, normalUserBindings{}, err
		}
		passwordHash = string(hash)
	}
	user.Nickname = nickname
	user.PasswordHash = passwordHash
	if payload.Disabled != nil {
		user.Disabled = *payload.Disabled
	}
	user.UpdatedAt = time.Now().Unix()
	bindings.UpdatedAt = user.UpdatedAt
	if _, _, err := userBucket.Set(normalUserStorageKey(username), utils.JsonMarshal(user)); err != nil {
		return nil, normalUserBindings{}, err
	}
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), utils.JsonMarshal(bindings)); err != nil {
		return nil, normalUserBindings{}, err
	}
	return user, bindings, nil
}

func replaceNormalUserBindings(username, qq, telegram string, openids []string) (normalUserBindings, error) {
	if _, err := loadNormalUser(username); err != nil {
		return normalUserBindings{}, err
	}
	bindings, err := normalizedReplacementBindings(qq, telegram, openids)
	if err != nil {
		return normalUserBindings{}, err
	}
	bindings.UpdatedAt = time.Now().Unix()
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), utils.JsonMarshal(bindings)); err != nil {
		return normalUserBindings{}, err
	}
	return bindings, nil
}

func normalizedReplacementBindings(qq, telegram string, openids []string) (normalUserBindings, error) {
	bindings := normalUserBindings{
		QQ:       strings.TrimSpace(qq),
		Telegram: strings.TrimSpace(telegram),
	}
	if bindings.QQ != "" && !userQQBindingPattern.MatchString(bindings.QQ) {
		return normalUserBindings{}, errors.New("QQ 号格式不正确")
	}
	if bindings.Telegram != "" && !userTGBindingPattern.MatchString(bindings.Telegram) {
		return normalUserBindings{}, errors.New("Telegram ID 格式不正确")
	}
	if len(openids) > 100 {
		return normalUserBindings{}, errors.New("smallcat openid 最多绑定 100 个")
	}
	for _, openid := range openids {
		openid = strings.TrimSpace(openid)
		if len([]rune(openid)) > 256 {
			return normalUserBindings{}, errors.New("smallcat openid 不能超过 256 位")
		}
		bindings.SmallcatOpenIDs = appendUniqueOpenID(bindings.SmallcatOpenIDs, openid)
	}
	return normalizeNormalUserBindings(bindings), nil
}

func deleteNormalUser(username string) error {
	username = normalizeNormalUsername(username)
	if err := validateNormalUsername(username); err != nil {
		return err
	}
	user, err := loadNormalUser(username)
	if err != nil {
		return err
	}
	authorizationPrefix := strings.TrimSpace(user.ID) + ":"
	authorizationKeys := []string{}
	pluginUserAuthorizations.Foreach(func(keyBytes, _ []byte) error {
		key := string(keyBytes)
		if strings.HasPrefix(key, authorizationPrefix) {
			authorizationKeys = append(authorizationKeys, key)
		}
		return nil
	})
	for _, key := range authorizationKeys {
		if _, _, err := pluginUserAuthorizations.Set(key, ""); err != nil {
			return err
		}
	}
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), ""); err != nil {
		return err
	}
	if _, _, err := userBucket.Set(normalUserStorageKey(username), ""); err != nil {
		return err
	}
	return nil
}

func adminNormalUserRowFor(user *normalUser, bindings normalUserBindings) adminNormalUserRow {
	return adminNormalUserRow{
		publicNormalUser: toPublicNormalUser(user),
		Bindings:         normalizeNormalUserBindings(bindings),
		UpdatedAt:        user.UpdatedAt,
		Disabled:         user.Disabled,
		StorageKey:       normalUserStorageKey(user.Username),
	}
}

func verifyNormalUser(username string, password string) (*normalUser, error) {
	username = normalizeNormalUsername(username)
	user, err := loadNormalUser(username)
	if err != nil {
		return nil, errors.New("账号或密码错误")
	}
	if user.Disabled {
		return nil, errors.New("账号已禁用")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("账号或密码错误")
	}
	return user, nil
}

func loadNormalUser(username string) (*normalUser, error) {
	raw := strings.TrimSpace(userBucket.GetString(normalUserStorageKey(username)))
	if raw == "" {
		return nil, errors.New("账号不存在")
	}
	user := &normalUser{}
	if err := json.Unmarshal([]byte(raw), user); err != nil {
		return nil, err
	}
	if user.Username == "" || user.ID == "" {
		return nil, errors.New("账号数据无效")
	}
	return user, nil
}

func RequireUserAuth(ctx *gin.Context) {
	token := userAuthTokenFromRequest(ctx)
	claims, err := parseUserJWT(token)
	if err != nil {
		ApiError(ctx, http.StatusUnauthorized, err.Error())
		ctx.Abort()
		return
	}
	user, err := loadNormalUser(claims.Sub)
	if err != nil || user.ID != claims.UID || user.Disabled {
		ApiError(ctx, http.StatusUnauthorized, "登录已失效")
		ctx.Abort()
		return
	}
	ctx.Set("normal_user", user)
}

func createUserJWTCookie(ctx *gin.Context, user *normalUser) (string, error) {
	now := time.Now().Unix()
	token, err := signUserJWT(userJWTClaims{
		Sub: user.Username,
		UID: user.ID,
		Iat: now,
		Exp: now + userJWTExpireSeconds,
	})
	if err != nil {
		return "", err
	}
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("user_token", token, userJWTExpireSeconds, "/", "", adminCookieSecure(ctx), true)
	return token, nil
}

func signUserJWT(claims userJWTClaims) (string, error) {
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
	return unsigned + "." + signUserJWTPart(unsigned), nil
}

func parseUserJWT(token string) (*userJWTClaims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("请先登录")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("JWT 格式错误")
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(signUserJWTPart(unsigned))) {
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
	claims := &userJWTClaims{}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, errors.New("JWT 内容无效")
	}
	if claims.Sub == "" || claims.UID == "" {
		return nil, errors.New("JWT 缺少用户信息")
	}
	if jwtClaimsExpired(time.Now().Unix(), claims.Iat, claims.Exp, userJWTExpireSeconds) {
		return nil, errors.New("JWT 已过期")
	}
	return claims, nil
}

func signUserJWTPart(unsigned string) string {
	mac := hmac.New(sha256.New, userJWTSecret())
	mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func userJWTSecret() []byte {
	sum := sha256.Sum256([]byte(GetMachineID() + "|sillyGirl-normal-user-jwt"))
	return sum[:]
}

func userAuthTokenFromRequest(ctx *gin.Context) string {
	header := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	token, _ := ctx.Cookie("user_token")
	return strings.TrimSpace(token)
}

func validateNormalUsername(username string) error {
	if !userNamePattern.MatchString(username) {
		return errors.New("账号只能包含 3-32 位字母、数字、下划线、横线或点")
	}
	return nil
}

func validateNormalPassword(password string) error {
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	if len([]rune(password)) > 128 {
		return errors.New("密码不能超过 128 位")
	}
	return nil
}

func normalizeNormalUsername(username string) string {
	return strings.TrimSpace(username)
}

func normalUserStorageKey(username string) string {
	return "user:" + strings.ToLower(strings.TrimSpace(username))
}

func normalUserBindingsStorageKey(username string) string {
	return "bindings:" + strings.ToLower(strings.TrimSpace(username))
}

func currentNormalUser(ctx *gin.Context) *normalUser {
	value, ok := ctx.Get("normal_user")
	if !ok {
		return nil
	}
	user, _ := value.(*normalUser)
	return user
}

func loadNormalUserBindings(username string) normalUserBindings {
	raw := strings.TrimSpace(userBucket.GetString(normalUserBindingsStorageKey(username)))
	if raw == "" {
		return normalUserBindings{}
	}
	bindings := normalUserBindings{}
	if json.Unmarshal([]byte(raw), &bindings) != nil {
		return normalUserBindings{}
	}
	return normalizeNormalUserBindings(bindings)
}

func updateNormalUserBinding(username string, platform string, value string) (normalUserBindings, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	value = strings.TrimSpace(value)
	bindings := loadNormalUserBindings(username)
	switch platform {
	case "qq":
		if value != "" && !userQQBindingPattern.MatchString(value) {
			return bindings, errors.New("QQ 号格式不正确")
		}
		bindings.QQ = value
	case "telegram", "tg", "tgid":
		if value != "" && !userTGBindingPattern.MatchString(value) {
			return bindings, errors.New("Telegram ID 格式不正确")
		}
		bindings.Telegram = value
	case "smallcat", "smallcat_openid":
		if value == "" {
			bindings.SmallcatOpenID = ""
			bindings.SmallcatOpenIDs = nil
			break
		}
		bindings.SmallcatOpenIDs = appendUniqueOpenID(bindings.SmallcatOpenIDs, value)
		if bindings.SmallcatOpenID == "" {
			bindings.SmallcatOpenID = bindings.SmallcatOpenIDs[0]
		}
	default:
		return bindings, errors.New("不支持的绑定类型")
	}
	bindings = normalizeNormalUserBindings(bindings)
	bindings.UpdatedAt = time.Now().Unix()
	if _, _, err := userBucket.Set(normalUserBindingsStorageKey(username), utils.JsonMarshal(bindings)); err != nil {
		return bindings, err
	}
	return bindings, nil
}

func isPublicUserBindingPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "qq", "telegram", "tg", "tgid":
		return true
	default:
		return false
	}
}

func normalizeNormalUserBindings(bindings normalUserBindings) normalUserBindings {
	bindings.QQ = strings.TrimSpace(bindings.QQ)
	bindings.Telegram = strings.TrimSpace(bindings.Telegram)
	openids := []string{}
	if text := strings.TrimSpace(bindings.SmallcatOpenID); text != "" {
		openids = appendUniqueOpenID(openids, text)
	}
	for _, openid := range bindings.SmallcatOpenIDs {
		openids = appendUniqueOpenID(openids, openid)
	}
	bindings.SmallcatOpenIDs = openids
	if len(openids) > 0 {
		bindings.SmallcatOpenID = openids[0]
	} else {
		bindings.SmallcatOpenID = ""
	}
	return bindings
}

func normalUserHasSmallcatOpenID(username string, openid string) bool {
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return false
	}
	bindings := loadNormalUserBindings(username)
	for _, item := range bindings.SmallcatOpenIDs {
		if item == openid {
			return true
		}
	}
	return bindings.SmallcatOpenID == openid
}

func appendUniqueOpenID(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}

func listNormalUsers() ([]adminNormalUserRow, error) {
	rows := []adminNormalUserRow{}
	var firstErr error
	userBucket.Foreach(func(keyBytes, valueBytes []byte) error {
		key := string(keyBytes)
		if !strings.HasPrefix(key, "user:") {
			return nil
		}
		user := &normalUser{}
		if err := json.Unmarshal(valueBytes, user); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("用户数据解析失败：%s", key)
			}
			return nil
		}
		if user.Username == "" || user.ID == "" {
			return nil
		}
		rows = append(rows, adminNormalUserRow{
			publicNormalUser: toPublicNormalUser(user),
			Bindings:         loadNormalUserBindings(user.Username),
			UpdatedAt:        user.UpdatedAt,
			Disabled:         user.Disabled,
			StorageKey:       key,
		})
		return nil
	})
	if firstErr != nil {
		return nil, firstErr
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt == rows[j].CreatedAt {
			return rows[i].Username < rows[j].Username
		}
		return rows[i].CreatedAt > rows[j].CreatedAt
	})
	return rows, nil
}

func decodeRawJSON(raw json.RawMessage) interface{} {
	var value interface{}
	if json.Unmarshal(raw, &value) != nil {
		return string(raw)
	}
	return value
}

func unwrapServiceData(value interface{}) (interface{}, string, bool) {
	payload, ok := value.(map[string]interface{})
	if !ok {
		return value, "", true
	}
	if status, exists := payload["status"].(bool); exists && !status {
		message := strings.TrimSpace(fmt.Sprint(payload["message"]))
		if message == "" {
			message = "smallcat 请求失败"
		}
		return nil, message, false
	}
	if data, exists := payload["data"]; exists {
		return data, "", true
	}
	return value, "", true
}

func findStringInJSON(value interface{}, keys ...string) string {
	keySet := map[string]bool{}
	for _, key := range keys {
		keySet[strings.ToLower(key)] = true
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			if keySet[strings.ToLower(key)] {
				if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
					return text
				}
			}
		}
		for _, item := range typed {
			if text := findStringInJSON(item, keys...); text != "" {
				return text
			}
		}
	case []interface{}:
		for _, item := range typed {
			if text := findStringInJSON(item, keys...); text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizeUserAnnouncementFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "html":
		return "html"
	case "md", "markdown":
		return "markdown"
	default:
		return "text"
	}
}

func toPublicNormalUser(user *normalUser) publicNormalUser {
	return publicNormalUser{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
	}
}
