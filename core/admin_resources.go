package core

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/utils"
)

var adminSettingsKeys = []string{
	"sillyGirl.name",
	"sillyGirl.password",
	"sillyGirl.port",
	"sillyGirl.api_key",
	"sillyGirl.user_announcement_enable",
	"sillyGirl.user_announcement",
	"sillyGirl.user_announcement_format",
	"sillyGirl.debug",
	"sillyGirl.listen_admin",
	"sillyGirl.recall",
	"sillyGirl.storage",
	"sillyGirl.redis_addr",
	"sillyGirl.redis_password",
}

var adminBotSettingsKeys = []string{
	"clawbot.enable", "clawbot.token", "clawbot.api_base", "clawbot.debug",
	"qq.enable", "qq.token", "qq.debug",
	"telegram.token", "telegram.enable", "telegram.api_base", "telegram.debug",
	"dingtalk.enable", "dingtalk.client_id", "dingtalk.client_secret", "dingtalk.debug",
	"qqguild.enable", "qqguild.mode", "qqguild.app_id", "qqguild.app_secret", "qqguild.sandbox", "qqguild.debug",
	"pagermaid.enable", "pagermaid.token", "pagermaid.debug",
	"sillyGirl.web_chat_public",
}

var messageRuleBuckets = map[string]string{
	"listening": "listenOnGroups",
	"muted":     "noReplyGroups",
	"blocked":   "noListenUsers",
}

func init() {
	GinApi(GET, "/api/admin/plugin-market/plugins", RequireAuth, handlePluginMarketPlugins)
	GinApi(POST, "/api/admin/plugin-market-snapshots", RequireAuth, handlePluginMarketPlugins)
	GinApi(GET, "/api/admin/settings", RequireAuth, handleGetAdminSettings)
	GinApi(POST, "/api/admin/settings", RequireAuth, handlePutAdminSettings)
	GinApi(GET, "/api/admin/bots", RequireAuth, func(ctx *gin.Context) {
		ApiOK(ctx, gin.H{
			"settings": adminStorageValues(adminBotSettingsKeys),
			"statuses": overviewAdapterStatuses(),
		})
	})
	GinApi(GET, "/api/admin/message-rules/:kind", RequireAuth, handleGetMessageRules)
	GinApi(POST, "/api/admin/message-rules/:kind/:key", RequireAuth, handlePutMessageRule)
	GinApi(POST, "/api/admin/message-rules/:kind/:key/deletions", RequireAuth, handleDeleteMessageRule)
}

func adminStorageValues(keys []string) map[string]interface{} {
	values := make(map[string]interface{}, len(keys))
	for _, dottedKey := range keys {
		parts := strings.SplitN(dottedKey, ".", 2)
		if len(parts) != 2 || isBackendVersionStorageKey(parts[0], parts[1]) {
			continue
		}
		values[dottedKey] = TransformBucketKeyValue(MakeBucket(parts[0]).GetString(parts[1]))
	}
	return values
}

func adminSettingsResponse() gin.H {
	githubProxy := githubAcceleratorPrefix()
	pnpm := pnpmRegistry()
	pipx := pipxRegistry()
	values := adminStorageValues(adminSettingsKeys)
	// 登录密码只允许写入，不把现值回传到浏览器。
	delete(values, "sillyGirl.password")
	return gin.H{
		"values": values,
		"github_proxy": gin.H{
			"value":   githubProxy,
			"options": settingOptions(pluginSourceGithubProxyOptionsKey, builtinGithubAccelerators, githubProxy, normalizeGithubAcceleratorPrefix),
		},
		"pnpm_registry": gin.H{
			"value":   pnpm,
			"options": settingOptions(pnpmRegistryOptionsKey, builtinPnpmRegistries, pnpm, normalizePnpmRegistry),
		},
		"pipx_registry": gin.H{
			"value":   pipx,
			"options": settingOptions(pipxRegistryOptionsKey, builtinPipxRegistries, pipx, normalizePipxRegistry),
		},
	}
}

func handleGetAdminSettings(ctx *gin.Context) {
	ApiOK(ctx, adminSettingsResponse())
}

func handlePutAdminSettings(ctx *gin.Context) {
	req := struct {
		Values       map[string]interface{} `json:"values"`
		GithubProxy  string                 `json:"github_proxy"`
		PnpmRegistry string                 `json:"pnpm_registry"`
		PipxRegistry string                 `json:"pipx_registry"`
	}{}
	if err := ctx.BindJSON(&req); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	githubProxy, err := normalizeGithubAcceleratorPrefix(req.GithubProxy)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	pnpm, err := normalizePnpmRegistry(req.PnpmRegistry)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}
	pipx, err := normalizePipxRegistry(req.PipxRegistry)
	if err != nil {
		ApiUnprocessable(ctx, err.Error())
		return
	}

	allowed := make(map[string]bool, len(adminSettingsKeys))
	for _, key := range adminSettingsKeys {
		allowed[key] = true
	}
	messages := map[string]interface{}{}
	errorsByKey := map[string]interface{}{}
	changes := map[string]bool{}
	for dottedKey, value := range req.Values {
		if !allowed[dottedKey] {
			errorsByKey[dottedKey] = "设置项不受支持"
			changes[dottedKey] = false
			continue
		}
		parts := strings.SplitN(dottedKey, ".", 2)
		message, changed, setErr := SetBucketKeyValue(MakeBucket(parts[0]), parts[1], value)
		if message != "" {
			messages[dottedKey] = message
		}
		if setErr != nil {
			errorsByKey[dottedKey] = setErr.Error()
		}
		changes[dottedKey] = changed
	}
	if len(errorsByKey) > 0 {
		ApiProblem(ctx, http.StatusUnprocessableEntity, "部分设置项无效", map[string]interface{}{
			"messages": messages,
			"errors":   errorsByKey,
			"changes":  changes,
		})
		return
	}
	sillyGirl.Set(pluginSourceGithubProxyKey, githubProxy)
	sillyGirl.Set("pnpm_registry", pnpm)
	if pipx != pipxRegistry() {
		invalidatePipxRuntimeEnvCache()
	}
	sillyGirl.Set("pipx_registry", pipx)
	response := adminSettingsResponse()
	response["messages"] = messages
	response["errors"] = errorsByKey
	response["changes"] = changes
	ApiOK(ctx, response)
}

func messageRuleBucket(kind string) (string, bool) {
	bucket, ok := messageRuleBuckets[strings.ToLower(strings.TrimSpace(kind))]
	return bucket, ok
}

func handleGetMessageRules(ctx *gin.Context) {
	bucketName, ok := messageRuleBucket(ctx.Param("kind"))
	if !ok {
		ApiNotFound(ctx, "消息规则类型不存在")
		return
	}
	rows := []map[string]string{}
	MakeBucket(bucketName).Foreach(func(key, value []byte) error {
		if !shouldHideStorageKey(bucketName, string(key)) {
			rows = append(rows, map[string]string{"key": string(key), "value": string(value)})
		}
		return nil
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i]["key"] < rows[j]["key"] })
	ApiOK(ctx, gin.H{"list": rows, "platforms": getPltsLabel()})
}

func handlePutMessageRule(ctx *gin.Context) {
	bucketName, ok := messageRuleBucket(ctx.Param("kind"))
	if !ok {
		ApiNotFound(ctx, "消息规则类型不存在")
		return
	}
	key := strings.TrimSpace(ctx.Param("key"))
	if key == "" {
		ApiUnprocessable(ctx, "消息规则 Key 不能为空")
		return
	}
	value := map[string]interface{}{}
	if err := ctx.BindJSON(&value); err != nil {
		ApiFail(ctx, err.Error())
		return
	}
	bucket := MakeBucket(bucketName)
	existed := strings.TrimSpace(bucket.GetString(key)) != ""
	if _, _, err := SetBucketKeyValue(bucket, key, utils.JsonMarshal(value)); err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	if !existed {
		ApiCreated(ctx, ctx.Request.URL.Path, gin.H{"key": key, "value": value})
		return
	}
	ApiOK(ctx, gin.H{"key": key, "value": value})
}

func handleDeleteMessageRule(ctx *gin.Context) {
	bucketName, ok := messageRuleBucket(ctx.Param("kind"))
	if !ok {
		ApiNotFound(ctx, "消息规则类型不存在")
		return
	}
	key := strings.TrimSpace(ctx.Param("key"))
	if key == "" {
		ApiUnprocessable(ctx, "消息规则 Key 不能为空")
		return
	}
	bucket := MakeBucket(bucketName)
	if strings.TrimSpace(bucket.GetString(key)) == "" {
		ApiNotFound(ctx, "消息规则不存在")
		return
	}
	if _, _, err := SetBucketKeyValue(bucket, key, nil); err != nil {
		ApiInternalError(ctx, err.Error())
		return
	}
	ApiNoContent(ctx)
}
