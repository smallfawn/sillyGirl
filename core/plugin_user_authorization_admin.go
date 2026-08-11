package core

import (
	"errors"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

type adminUserPluginAuthorizationRow struct {
	UUID         string `json:"uuid"`
	Title        string `json:"title"`
	Desc         string `json:"desc"`
	Icon         string `json:"icon"`
	Version      string `json:"version"`
	Author       string `json:"author"`
	Class        string `json:"class"`
	Open         bool   `json:"open"`
	Installed    bool   `json:"installed"`
	UsesSmallCat bool   `json:"uses_smallcat"`
	HasUserForm  bool   `json:"has_user_form"`
	Authorized   bool   `json:"authorized"`
}

func init() {
	GinApi(GET, "/api/admin/users/:username/plugins", RequireAuth, func(ctx *gin.Context) {
		username := strings.TrimSpace(ctx.Param("username"))
		rows, err := listNormalUserPluginAuthorizations(username)
		if err != nil {
			if strings.Contains(err.Error(), "不存在") {
				ApiNotFound(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		ApiOK(ctx, gin.H{
			"list":  rows,
			"total": len(rows),
		})
	})

	GinApi(POST, "/api/admin/users/:username/plugins/:uuid", RequireAuth, func(ctx *gin.Context) {
		payload := struct {
			Authorized bool `json:"authorized"`
		}{}
		if err := ctx.ShouldBindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		username := strings.TrimSpace(ctx.Param("username"))
		uuid := strings.TrimSpace(ctx.Param("uuid"))
		if err := setPluginUserSmallcatAuthorizationByUsername(username, uuid, payload.Authorized); err != nil {
			if strings.Contains(err.Error(), "不存在") {
				ApiNotFound(ctx, err.Error())
			} else {
				ApiUnprocessable(ctx, err.Error())
			}
			return
		}
		ApiOK(ctx, gin.H{
			"username":            username,
			"uuid":                uuid,
			"authorized":          payload.Authorized,
			"authorization_scope": pluginSmallcatReadScope,
		})
	})
}

func listNormalUserPluginAuthorizations(username string) ([]adminUserPluginAuthorizationRow, error) {
	user, err := loadNormalUser(username)
	if err != nil {
		return nil, err
	}
	authorized := map[string]bool{}
	pluginUserAuthorizations.Foreach(func(keyBytes, _ []byte) error {
		key := string(keyBytes)
		prefix := user.ID + ":"
		suffix := ":" + pluginSmallcatReadScope
		if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
			return nil
		}
		pluginID := strings.TrimSuffix(strings.TrimPrefix(key, prefix), suffix)
		pluginID = strings.TrimSpace(pluginID)
		if pluginID != "" {
			authorized[pluginID] = true
		}
		return nil
	})
	rows := make([]adminUserPluginAuthorizationRow, 0, len(authorized))
	seen := map[string]bool{}
	for _, plugin := range installedPluginSnapshot() {
		if plugin == nil || plugin.UUID == "" || !plugin.UsesSmallCat {
			continue
		}
		row := adminUserPluginAuthorizationRow{
			UUID:         plugin.UUID,
			Title:        firstNonEmpty(plugin.Title, plugin.UUID),
			Desc:         parseReply2(plugin.Desc),
			Icon:         pluginIconOrDefault(plugin.Icon),
			Version:      plugin.Version,
			Author:       plugin.Author,
			Class:        plugin.Class,
			Open:         plugin.Open,
			Installed:    true,
			UsesSmallCat: plugin.UsesSmallCat,
			HasUserForm:  plugin.HasUserForm,
			Authorized:   authorized[plugin.UUID],
		}
		rows = append(rows, row)
		seen[plugin.UUID] = true
	}
	for pluginID := range authorized {
		if seen[pluginID] {
			continue
		}
		rows = append(rows, adminUserPluginAuthorizationRow{
			UUID:         pluginID,
			Title:        pluginID,
			Installed:    false,
			UsesSmallCat: true,
			Authorized:   true,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Title == rows[j].Title {
			return rows[i].UUID < rows[j].UUID
		}
		return rows[i].Title < rows[j].Title
	})
	return rows, nil
}

func adminUserPluginAuthorizationsForUser(userID string) []adminUserPluginAuthorizationRow {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return []adminUserPluginAuthorizationRow{}
	}
	authorized := map[string]bool{}
	pluginUserAuthorizations.Foreach(func(keyBytes, _ []byte) error {
		key := string(keyBytes)
		prefix := userID + ":"
		suffix := ":" + pluginSmallcatReadScope
		if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
			return nil
		}
		pluginID := strings.TrimSuffix(strings.TrimPrefix(key, prefix), suffix)
		pluginID = strings.TrimSpace(pluginID)
		if pluginID != "" {
			authorized[pluginID] = true
		}
		return nil
	})
	rows := make([]adminUserPluginAuthorizationRow, 0, len(authorized))
	for _, plugin := range installedPluginSnapshot() {
		if plugin == nil || plugin.UUID == "" || !plugin.UsesSmallCat {
			continue
		}
		if !authorized[plugin.UUID] {
			continue
		}
		rows = append(rows, adminUserPluginAuthorizationRow{
			UUID:         plugin.UUID,
			Title:        firstNonEmpty(plugin.Title, plugin.UUID),
			Desc:         parseReply2(plugin.Desc),
			Icon:         pluginIconOrDefault(plugin.Icon),
			Version:      plugin.Version,
			Author:       plugin.Author,
			Class:        plugin.Class,
			Open:         plugin.Open,
			Installed:    true,
			UsesSmallCat: plugin.UsesSmallCat,
			HasUserForm:  plugin.HasUserForm,
			Authorized:   true,
		})
		delete(authorized, plugin.UUID)
	}
	for pluginID := range authorized {
		rows = append(rows, adminUserPluginAuthorizationRow{
			UUID:         pluginID,
			Title:        pluginID,
			Installed:    false,
			UsesSmallCat: true,
			Authorized:   true,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Title == rows[j].Title {
			return rows[i].UUID < rows[j].UUID
		}
		return rows[i].Title < rows[j].Title
	})
	return rows
}

func setPluginUserSmallcatAuthorizationByUsername(username string, pluginID string, authorized bool) error {
	username = strings.TrimSpace(username)
	pluginID = strings.TrimSpace(pluginID)
	if username == "" || pluginID == "" {
		return errors.New("用户或插件无效")
	}
	user, err := loadNormalUser(username)
	if err != nil {
		return err
	}
	if authorized {
		plugin := installedPluginByUUID(pluginID)
		if plugin == nil {
			return errors.New("插件未安装")
		}
		if !plugin.UsesSmallCat {
			return errors.New("插件未使用 smallcat")
		}
	}
	return setPluginUserSmallcatAuthorization(user, pluginID, authorized)
}
