package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/utils"
)

const (
	pluginSmallcatReadScope        = "smallcat:read"
	pluginSmallcatRuntimeBucket    = "__plugin_smallcat_authorized__"
	pluginSmallcatRuntimeRecordKey = "records"
	pluginUserRuntimeBucket        = "__plugin_users__"
	pluginUserRuntimeListKey       = "list"
)

var pluginUserAuthorizations = MakeBucket("plugin_user_authorizations")

func isPluginRuntimeBucket(name string) bool {
	return name == pluginSmallcatRuntimeBucket || name == pluginUserRuntimeBucket
}

type userOpenPluginRecord struct {
	openPluginRecord
	Authorized           bool   `json:"authorized"`
	AuthorizationScope   string `json:"authorization_scope"`
	SmallcatAccountCount int    `json:"smallcat_account_count"`
}

type authorizedSmallcatUser struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Nickname string   `json:"nickname"`
	OpenIDs  []string `json:"openids"`
}

type pluginSmallcatRuntimeRecords struct {
	Enforced bool                     `json:"enforced"`
	Scope    string                   `json:"scope"`
	OpenIDs  []string                 `json:"openids"`
	Users    []authorizedSmallcatUser `json:"users"`
}

type pluginRuntimeUserBindings struct {
	QQ              string   `json:"qq"`
	Telegram        string   `json:"telegram"`
	SmallcatOpenIDs []string `json:"smallcat_openids"`
}

// pluginRuntimeUser is the public, read-only user view exposed to a running
// plugin. Password hashes, storage keys and internal timestamps never cross the
// runtime boundary. Authorized always refers to the current plugin's
// smallcat:read grant; plugins cannot query another plugin's authorization.
type pluginRuntimeUser struct {
	ID         string                    `json:"id"`
	Username   string                    `json:"username"`
	Nickname   string                    `json:"nickname"`
	Disabled   bool                      `json:"disabled"`
	Authorized bool                      `json:"authorized"`
	Bindings   pluginRuntimeUserBindings `json:"bindings"`
}

func init() {
	GinApi(GET, "/api/user/plugins", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		ApiOK(ctx, userOpenPluginRecords(user))
	})

	GinApi(PUT, "/api/user/plugin/authorization", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		payload := struct {
			UUID       string `json:"uuid"`
			Authorized bool   `json:"authorized"`
		}{}
		if err := json.NewDecoder(ctx.Request.Body).Decode(&payload); err != nil {
			ApiFail(ctx, "请求体不是有效 JSON")
			return
		}
		payload.UUID = strings.TrimSpace(payload.UUID)
		plugin := installedPluginByUUID(payload.UUID)
		if plugin == nil || !plugin.Open || plugin.Disable || !plugin.UsesSmallCat {
			ApiFail(ctx, "插件未开放")
			return
		}
		if err := setPluginUserSmallcatAuthorization(user, plugin.UUID, payload.Authorized); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"uuid":                plugin.UUID,
			"authorized":          payload.Authorized,
			"authorization_scope": pluginSmallcatReadScope,
		})
	})

	// This endpoint is the user-session boundary for plugin Web UIs. It only
	// exposes the current user's bound openids after that user grants the plugin
	// smallcat:read. Operations such as getCode keep using their existing route
	// and do not require an additional per-operation grant.
	GinApi(GET, "/api/user/plugin/smallcat", RequireUserAuth, func(ctx *gin.Context) {
		user := currentNormalUser(ctx)
		if user == nil {
			ApiError(ctx, http.StatusUnauthorized, "请先登录")
			return
		}
		uuid := strings.TrimSpace(ctx.Query("uuid"))
		plugin := installedPluginByUUID(uuid)
		if plugin == nil || !plugin.Open || plugin.Disable || !plugin.UsesSmallCat {
			ApiFail(ctx, "插件未开放")
			return
		}
		if !pluginUserSmallcatAuthorized(user.ID, uuid) {
			ApiError(ctx, http.StatusForbidden, "尚未授权该插件读取 smallcat 账号")
			return
		}
		bindings := loadNormalUserBindings(user.Username)
		ApiOK(ctx, gin.H{
			"scope":   pluginSmallcatReadScope,
			"openids": append([]string{}, bindings.SmallcatOpenIDs...),
		})
	})
}

func userOpenPluginRecords(user *normalUser) []userOpenPluginRecord {
	if user == nil {
		return []userOpenPluginRecord{}
	}
	bindings := loadNormalUserBindings(user.Username)
	openRows := openPluginRecords(Functions)
	rows := make([]userOpenPluginRecord, 0, len(openRows))
	for _, plugin := range openRows {
		rows = append(rows, userOpenPluginRecord{
			openPluginRecord:     plugin,
			Authorized:           pluginUserSmallcatAuthorized(user.ID, plugin.ID),
			AuthorizationScope:   pluginSmallcatReadScope,
			SmallcatAccountCount: len(bindings.SmallcatOpenIDs),
		})
	}
	return rows
}

func pluginUserAuthorizationKey(userID string, pluginID string) string {
	return strings.TrimSpace(userID) + ":" + strings.TrimSpace(pluginID) + ":" + pluginSmallcatReadScope
}

func pluginUserSmallcatAuthorized(userID string, pluginID string) bool {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(pluginID) == "" {
		return false
	}
	return pluginUserAuthorizations.GetBool(pluginUserAuthorizationKey(userID, pluginID))
}

func setPluginUserSmallcatAuthorization(user *normalUser, pluginID string, authorized bool) error {
	if user == nil || strings.TrimSpace(user.ID) == "" || strings.TrimSpace(pluginID) == "" {
		return errors.New("用户或插件无效")
	}
	value := interface{}(authorized)
	if !authorized {
		value = nil
	}
	_, _, err := pluginUserAuthorizations.Set(pluginUserAuthorizationKey(user.ID, pluginID), value)
	return err
}

func pluginAuthorizedSmallcatRecords(pluginID string) pluginSmallcatRuntimeRecords {
	records := pluginSmallcatRuntimeRecords{
		Scope:   pluginSmallcatReadScope,
		OpenIDs: []string{},
		Users:   []authorizedSmallcatUser{},
	}
	plugin := installedPluginByUUID(strings.TrimSpace(pluginID))
	if plugin == nil {
		return records
	}
	// Plugins that have never been published keep the legacy unrestricted
	// userList behavior. Once a plugin enters the Home authorization model, the
	// policy remains enforced even after the administrator closes it; a closed
	// or disabled plugin then sees an empty account list instead of falling back
	// to the complete smallcat panel list.
	managed := plugin.Open || strings.TrimSpace(plugin_open.GetString(plugin.UUID)) != ""
	if !managed {
		return records
	}
	records.Enforced = true
	if !plugin.Open || plugin.Disable {
		return records
	}
	seenOpenIDs := map[string]bool{}
	for _, row := range pluginRuntimeUsers(plugin.UUID) {
		if !row.Authorized {
			continue
		}
		openids := append([]string(nil), row.Bindings.SmallcatOpenIDs...)
		if len(openids) == 0 {
			continue
		}
		records.Users = append(records.Users, authorizedSmallcatUser{
			UserID:   row.ID,
			Username: row.Username,
			Nickname: row.Nickname,
			OpenIDs:  openids,
		})
		for _, openid := range openids {
			if openid = strings.TrimSpace(openid); openid != "" && !seenOpenIDs[openid] {
				seenOpenIDs[openid] = true
				records.OpenIDs = append(records.OpenIDs, openid)
			}
		}
	}
	sort.Strings(records.OpenIDs)
	sort.SliceStable(records.Users, func(i, j int) bool {
		return records.Users[i].Username < records.Users[j].Username
	})
	return records
}

func pluginSmallcatRuntimeValue(pluginID string, key string) string {
	if strings.TrimSpace(key) != pluginSmallcatRuntimeRecordKey {
		return ""
	}
	return "o:" + string(utils.JsonMarshal(pluginAuthorizedSmallcatRecords(pluginID)))
}

func pluginRuntimeUsers(pluginID string) []pluginRuntimeUser {
	pluginID = strings.TrimSpace(pluginID)
	plugin := installedPluginByUUID(pluginID)
	if pluginID == "" || plugin == nil {
		return []pluginRuntimeUser{}
	}
	grantEnabled := plugin.Open && !plugin.Disable && plugin.UsesSmallCat
	rows, err := listNormalUsers()
	if err != nil {
		return []pluginRuntimeUser{}
	}
	result := make([]pluginRuntimeUser, 0, len(rows))
	for _, row := range rows {
		authorized := grantEnabled && !row.Disabled && pluginUserSmallcatAuthorized(row.ID, pluginID)
		openids := []string{}
		if authorized {
			openids = append(openids, row.Bindings.SmallcatOpenIDs...)
		}
		result = append(result, pluginRuntimeUser{
			ID:         row.ID,
			Username:   row.Username,
			Nickname:   row.Nickname,
			Disabled:   row.Disabled,
			Authorized: authorized,
			Bindings: pluginRuntimeUserBindings{
				QQ:              row.Bindings.QQ,
				Telegram:        row.Bindings.Telegram,
				SmallcatOpenIDs: openids,
			},
		})
	}
	return result
}

func pluginUserRuntimeValue(pluginID string, key string) string {
	if strings.TrimSpace(key) != pluginUserRuntimeListKey {
		return ""
	}
	return "o:" + string(utils.JsonMarshal(pluginRuntimeUsers(pluginID)))
}
