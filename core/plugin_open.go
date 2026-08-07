package core

import (
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
)

type openPluginRecord struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"desc"`
	Icon         string   `json:"icon"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Class        string   `json:"class"`
	Rule         string   `json:"rule,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	UsesSmallCat bool     `json:"uses_smallcat"`
	HasUserForm  bool     `json:"has_user_form"`
}

func init() {
	GinApi(POST, "/api/admin/plugins/:uuid/access", RequireAuth, func(ctx *gin.Context) {
		payload := struct {
			UUID string `json:"uuid"`
			Open bool   `json:"open"`
		}{}
		if err := ctx.BindJSON(&payload); err != nil {
			ApiFail(ctx, err.Error())
			return
		}
		payload.UUID = ctx.Param("uuid")
		payload.UUID = strings.TrimSpace(payload.UUID)
		plugin := installedPluginByUUID(payload.UUID)
		if plugin == nil {
			ApiNotFound(ctx, "插件未安装")
			return
		}
		if payload.Open && !plugin.UsesSmallCat && !plugin.HasUserForm {
			ApiUnprocessable(ctx, "插件没有用户表单，也未使用 smallcat")
			return
		}
		previous := plugin.Open
		plugin.Open = payload.Open
		if _, _, err := plugin_open.Set(payload.UUID, payload.Open); err != nil {
			plugin.Open = previous
			ApiInternalError(ctx, "保存插件开放状态失败："+err.Error())
			return
		}
		ApiOK(ctx, gin.H{
			"uuid": payload.UUID,
			"open": payload.Open,
		})
	})

	GinApi(GET, "/api/public/plugins", func(ctx *gin.Context) {
		ApiOK(ctx, openPluginRecords(Functions))
	})
}

func installedPluginByUUID(uuid string) *common.Function {
	if uuid == "" {
		return nil
	}
	for _, plugin := range Functions {
		if plugin != nil && plugin.UUID == uuid && (plugin.Type == NODE || plugin.Type == PYTHON) {
			return plugin
		}
	}
	return nil
}

func openPluginRecords(plugins []*common.Function) []openPluginRecord {
	rows := []openPluginRecord{}
	for _, plugin := range plugins {
		if plugin == nil || plugin.UUID == "" || !plugin.Open || (!plugin.UsesSmallCat && !plugin.HasUserForm) || plugin.Disable || (plugin.Type != NODE && plugin.Type != PYTHON) {
			continue
		}
		rows = append(rows, openPluginRecord{
			ID:           plugin.UUID,
			Title:        firstNonEmpty(plugin.Title, plugin.UUID),
			Description:  parseReply2(plugin.Description),
			Icon:         pluginIconOrDefault(plugin.Icon),
			Version:      plugin.Version,
			Author:       plugin.Author,
			Class:        plugin.Class,
			Rule:         plugin.Rule,
			Dependencies: append([]string(nil), plugin.Dependencies...),
			UsesSmallCat: plugin.UsesSmallCat,
			HasUserForm:  plugin.HasUserForm,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Title < rows[j].Title
	})
	return rows
}
